package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	llmhttp "github.com/bkyoung/code-reviewer/internal/adapter/llm/http"
	"github.com/bkyoung/code-reviewer/internal/config"
	"github.com/bkyoung/code-reviewer/internal/domain"
)

const (
	defaultBaseURL = "https://generativelanguage.googleapis.com"
	defaultTimeout = 60 * time.Second
)

// HTTPClient is an HTTP client for the Google Gemini API.
type HTTPClient struct {
	apiKey    string
	model     string
	baseURL   string
	timeout   time.Duration
	retryConf llmhttp.RetryConfig
	client    *http.Client

	// Observability components
	logger  llmhttp.Logger
	metrics llmhttp.Metrics
	pricing llmhttp.Pricing
}

// NewHTTPClient creates a new Gemini HTTP client.
func NewHTTPClient(apiKey, model string, providerCfg config.ProviderConfig, httpCfg config.HTTPConfig) *HTTPClient {
	timeout := llmhttp.ParseTimeout(providerCfg.Timeout, httpCfg.Timeout, defaultTimeout)
	retryConf := llmhttp.BuildRetryConfig(providerCfg, httpCfg)

	return &HTTPClient{
		apiKey:    apiKey,
		model:     model,
		baseURL:   defaultBaseURL,
		timeout:   timeout,
		retryConf: retryConf,
		client:    &http.Client{Timeout: timeout},
	}
}

// SetBaseURL sets a custom base URL (for testing).
func (c *HTTPClient) SetBaseURL(url string) {
	c.baseURL = url
}

// SetTimeout sets the HTTP timeout.
func (c *HTTPClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.client.Timeout = timeout
}

// SetLogger sets the logger for this client.
func (c *HTTPClient) SetLogger(logger llmhttp.Logger) {
	c.logger = logger
}

// SetMetrics sets the metrics tracker for this client.
func (c *HTTPClient) SetMetrics(metrics llmhttp.Metrics) {
	c.metrics = metrics
}

// SetPricing sets the pricing calculator for this client.
func (c *HTTPClient) SetPricing(pricing llmhttp.Pricing) {
	c.pricing = pricing
}

// CallOptions contains options for the API call.
type CallOptions struct {
	Temperature float64
	MaxTokens   int
}

// APIResponse represents the parsed response from the API.
type APIResponse struct {
	Text         string
	TokensIn     int
	TokensOut    int
	FinishReason string
	Cost         float64 // Cost in USD
}

// Call makes a request to the Gemini generateContent API.
func (c *HTTPClient) Call(ctx context.Context, prompt string, options CallOptions) (*APIResponse, error) {
	startTime := time.Now()

	// Log request (if logger configured)
	if c.logger != nil {
		c.logger.LogRequest(ctx, llmhttp.RequestLog{
			Provider:    "gemini",
			Model:       c.model,
			Timestamp:   startTime,
			PromptChars: len(prompt),
			APIKey:      c.apiKey,
		})
	}

	// Record request metric
	if c.metrics != nil {
		c.metrics.RecordRequest("gemini", c.model)
	}

	// Build request
	reqBody := GenerateContentRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
	}

	// Add generation config if options provided
	if options.Temperature > 0 || options.MaxTokens > 0 {
		reqBody.GenerationConfig = &GenerationConfig{}
		if options.Temperature > 0 {
			reqBody.GenerationConfig.Temperature = options.Temperature
		}
		if options.MaxTokens > 0 {
			reqBody.GenerationConfig.MaxOutputTokens = options.MaxTokens
		}
		reqBody.GenerationConfig.CandidateCount = 1
	}

	// Add default safety settings (block only high severity)
	reqBody.SafetySettings = []SafetySetting{
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_ONLY_HIGH"},
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create URL with API key
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	// Execute request with retry logic (using configured retry settings)
	var resp *http.Response

	err = llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
		// Recreate request for each retry
		retryReq, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if reqErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   reqErr.Error(),
				Retryable: false,
				Provider:  "gemini",
			}
		}

		retryReq.Header.Set("Content-Type", "application/json")

		var callErr error
		resp, callErr = c.client.Do(retryReq)
		if callErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeTimeout,
				Message:   callErr.Error(),
				Retryable: false,
				Provider:  "gemini",
			}
		}

		// Check for error status codes
		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return c.handleErrorResponse(resp.StatusCode, bodyBytes)
		}

		return nil
	}, c.retryConf)

	duration := time.Since(startTime)

	if err != nil {
		// Log error
		if c.logger != nil {
			var httpErr *llmhttp.Error
			if errors.As(err, &httpErr) {
				c.logger.LogError(ctx, llmhttp.ErrorLog{
					Provider:   "gemini",
					Model:      c.model,
					Timestamp:  time.Now(),
					Duration:   duration,
					Error:      err,
					ErrorType:  httpErr.Type,
					StatusCode: httpErr.StatusCode,
					Retryable:  httpErr.Retryable,
				})
			}
		}
		// Record error metric
		if c.metrics != nil {
			var httpErr *llmhttp.Error
			if errors.As(err, &httpErr) {
				c.metrics.RecordError("gemini", c.model, httpErr.Type)
			}
		}
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var genResp GenerateContentResponse
	if err := json.Unmarshal(bodyBytes, &genResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate response
	if len(genResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := genResp.Candidates[0]

	// Check for content filtering
	if candidate.FinishReason == "SAFETY" {
		return nil, &llmhttp.Error{
			Type:      llmhttp.ErrTypeContentFiltered,
			Message:   "Content blocked by safety filters",
			Retryable: false,
			Provider:  "gemini",
		}
	}

	// Extract text from parts
	var textParts []string
	for _, part := range candidate.Content.Parts {
		textParts = append(textParts, part.Text)
	}

	response := &APIResponse{
		Text:         strings.Join(textParts, ""),
		TokensIn:     genResp.UsageMetadata.PromptTokenCount,
		TokensOut:    genResp.UsageMetadata.CandidatesTokenCount,
		FinishReason: candidate.FinishReason,
	}

	// Calculate cost
	var cost float64
	if c.pricing != nil {
		cost = c.pricing.GetCost("gemini", c.model, response.TokensIn, response.TokensOut)
		response.Cost = cost
	}

	// Log response
	if c.logger != nil {
		c.logger.LogResponse(ctx, llmhttp.ResponseLog{
			Provider:     "gemini",
			Model:        c.model,
			Timestamp:    time.Now(),
			Duration:     duration,
			TokensIn:     response.TokensIn,
			TokensOut:    response.TokensOut,
			Cost:         cost,
			StatusCode:   200,
			FinishReason: response.FinishReason,
		})
	}

	// Record metrics
	if c.metrics != nil {
		c.metrics.RecordDuration("gemini", c.model, duration)
		c.metrics.RecordTokens("gemini", c.model, response.TokensIn, response.TokensOut)
		c.metrics.RecordCost("gemini", c.model, cost)
	}

	return response, nil
}

// handleErrorResponse maps HTTP status codes to typed errors.
func (c *HTTPClient) handleErrorResponse(statusCode int, body []byte) error {
	// Try to parse Gemini error format
	var errResp ErrorResponse
	defaultMessage := fmt.Sprintf("HTTP %d", statusCode)
	message := defaultMessage

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	// Map status codes to error types
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeAuthentication,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "gemini",
		}
	case http.StatusTooManyRequests:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeRateLimit,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  true,
			Provider:   "gemini",
		}
	case http.StatusBadRequest:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeInvalidRequest,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "gemini",
		}
	case http.StatusServiceUnavailable, http.StatusInternalServerError:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeServiceUnavailable,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  true,
			Provider:   "gemini",
		}
	default:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeUnknown,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "gemini",
		}
	}
}

// CreateReview implements the Client interface for the Provider.
func (c *HTTPClient) CreateReview(ctx context.Context, req Request) (Response, error) {
	apiResp, err := c.Call(ctx, req.Prompt, CallOptions{
		MaxTokens: req.MaxTokens,
	})
	if err != nil {
		return Response{}, fmt.Errorf("gemini: %w", err)
	}

	// Parse the response text to extract JSON review
	review, err := parseReviewJSON(apiResp.Text)
	if err != nil {
		// If JSON parsing fails, return text as summary
		return Response{
			Model:    c.model,
			Summary:  apiResp.Text,
			Findings: []domain.Finding{},
		}, nil
	}

	review.Model = c.model
	return review, nil
}

// parseReviewJSON extracts and parses the JSON review from the response text.
func parseReviewJSON(text string) (Response, error) {
	// Use shared JSON parsing utility
	summary, findings, err := llmhttp.ParseReviewResponse(text)
	if err != nil {
		return Response{}, err
	}

	return Response{
		Model:    "", // Will be set by caller
		Summary:  summary,
		Findings: findings,
	}, nil
}
