package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	llmhttp "github.com/brandon/code-reviewer/internal/adapter/llm/http"
	"github.com/brandon/code-reviewer/internal/domain"
)

const (
	defaultTimeout = 120 * time.Second // Local models can be slower
)

// HTTPClient is an HTTP client for the Ollama API.
type HTTPClient struct {
	baseURL string
	model   string
	timeout time.Duration
	client  *http.Client
}

// NewHTTPClient creates a new Ollama HTTP client.
func NewHTTPClient(baseURL, model string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		model:   model,
		timeout: defaultTimeout,
		client:  &http.Client{Timeout: defaultTimeout},
	}
}

// SetTimeout sets the HTTP timeout.
func (c *HTTPClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.client.Timeout = timeout
}

// CallOptions contains options for the API call.
type CallOptions struct {
	Temperature float64
	Seed        *uint64
}

// APIResponse represents the parsed response from the API.
type APIResponse struct {
	Text      string
	TokensIn  int
	TokensOut int
	Model     string
}

// Call makes a request to the Ollama Generate API.
func (c *HTTPClient) Call(ctx context.Context, prompt string, options CallOptions) (*APIResponse, error) {
	// Build request
	reqBody := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false, // We don't use streaming
	}

	// Add options
	opts := make(map[string]interface{})
	if options.Temperature > 0 {
		opts["temperature"] = options.Temperature
	}
	if options.Seed != nil {
		opts["seed"] = float64(*options.Seed)
	}
	if len(opts) > 0 {
		reqBody.Options = opts
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request with retry logic
	var resp *http.Response
	retryConfig := llmhttp.RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     8 * time.Second,
		Multiplier:     2.0,
	}

	err = llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
		// Recreate request for each retry
		retryReq, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if reqErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   reqErr.Error(),
				Retryable: false,
				Provider:  "ollama",
			}
		}

		retryReq.Header.Set("Content-Type", "application/json")

		var callErr error
		resp, callErr = c.client.Do(retryReq)
		if callErr != nil {
			// Check for connection refused (Ollama not running)
			if strings.Contains(callErr.Error(), "connection refused") {
				return &llmhttp.Error{
					Type:      llmhttp.ErrTypeServiceUnavailable,
					Message:   fmt.Sprintf("Ollama server not reachable. Is Ollama running? Try: ollama serve. Error: %s", callErr.Error()),
					Retryable: false,
					Provider:  "ollama",
				}
			}
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeTimeout,
				Message:   callErr.Error(),
				Retryable: false,
				Provider:  "ollama",
			}
		}

		// Check for error status codes
		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return c.handleErrorResponse(resp.StatusCode, bodyBytes)
		}

		return nil
	}, retryConfig)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(bodyBytes, &genResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate response
	if !genResp.Done {
		return nil, fmt.Errorf("incomplete response from Ollama (done=false)")
	}

	if genResp.Response == "" {
		return nil, fmt.Errorf("empty response from Ollama")
	}

	return &APIResponse{
		Text:      genResp.Response,
		TokensIn:  genResp.PromptEvalCount,
		TokensOut: genResp.EvalCount,
		Model:     genResp.Model,
	}, nil
}

// handleErrorResponse maps HTTP status codes to typed errors.
func (c *HTTPClient) handleErrorResponse(statusCode int, body []byte) error {
	// Try to parse Ollama error format
	var errResp ErrorResponse
	defaultMessage := fmt.Sprintf("HTTP %d", statusCode)
	message := defaultMessage

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		message = errResp.Error
	}

	// Map status codes to error types
	switch statusCode {
	case http.StatusNotFound:
		// Model not found
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeModelNotFound,
			Message:    fmt.Sprintf("%s. Pull it with: ollama pull %s", message, c.model),
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "ollama",
		}
	case http.StatusBadRequest:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeInvalidRequest,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "ollama",
		}
	case http.StatusServiceUnavailable, http.StatusInternalServerError:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeServiceUnavailable,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  true,
			Provider:   "ollama",
		}
	default:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeUnknown,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "ollama",
		}
	}
}

// CreateReview implements the Client interface for the Provider.
func (c *HTTPClient) CreateReview(ctx context.Context, req Request) (Response, error) {
	var seed *uint64
	if req.Seed > 0 {
		seed = &req.Seed
	}

	apiResp, err := c.Call(ctx, req.Prompt, CallOptions{
		Seed: seed,
	})
	if err != nil {
		return Response{}, fmt.Errorf("ollama: %w", err)
	}

	// Parse the response text to extract JSON review
	review, err := parseReviewJSON(apiResp.Text)
	if err != nil {
		// If JSON parsing fails, return text as summary
		return Response{
			Model:    apiResp.Model,
			Summary:  apiResp.Text,
			Findings: []domain.Finding{},
		}, nil
	}

	review.Model = apiResp.Model
	return review, nil
}

// parseReviewJSON extracts and parses the JSON review from the response text.
// The LLM may return JSON wrapped in markdown code blocks.
func parseReviewJSON(text string) (Response, error) {
	// Try to extract JSON from markdown code blocks
	jsonPattern := regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)```")
	matches := jsonPattern.FindStringSubmatch(text)

	var jsonText string
	if len(matches) > 1 {
		jsonText = strings.TrimSpace(matches[1])
	} else {
		// Try parsing the raw text as JSON
		jsonText = strings.TrimSpace(text)
	}

	// Parse the JSON
	var result struct {
		Summary  string           `json:"summary"`
		Findings []domain.Finding `json:"findings"`
	}

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return Response{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return Response{
		Model:    "", // Will be set by caller
		Summary:  result.Summary,
		Findings: result.Findings,
	}, nil
}
