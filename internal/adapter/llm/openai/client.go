package openai

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
	defaultBaseURL = "https://api.openai.com"
	defaultTimeout = 60 * time.Second
)

// isO1Model returns true if the model is an o1-series reasoning model.
// These models have different API requirements:
// - Use max_completion_tokens instead of max_tokens
// - Don't support temperature, seed, or response_format
func isO1Model(model string) bool {
	modelLower := strings.ToLower(model)
	return strings.HasPrefix(modelLower, "o1-") || strings.HasPrefix(modelLower, "o4-")
}

// HTTPClient is an HTTP client for the OpenAI API.
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

// NewHTTPClient creates a new OpenAI HTTP client.
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
	Seed        *uint64
	MaxTokens   int
}

// APIResponse represents the parsed response from the API.
type APIResponse struct {
	Text         string
	TokensIn     int
	TokensOut    int
	Model        string
	FinishReason string
	Cost         float64 // Cost in USD
}

// Call makes a request to the OpenAI Chat Completion API.
func (c *HTTPClient) Call(ctx context.Context, prompt string, options CallOptions) (*APIResponse, error) {
	startTime := time.Now()

	// Log request (if logger configured)
	if c.logger != nil {
		c.logger.LogRequest(ctx, llmhttp.RequestLog{
			Provider:    "openai",
			Model:       c.model,
			Timestamp:   startTime,
			PromptChars: len(prompt),
			APIKey:      c.apiKey,
		})
	}

	// Record request metric
	if c.metrics != nil {
		c.metrics.RecordRequest("openai", c.model)
	}

	// Build request
	reqBody := ChatCompletionRequest{
		Model: c.model,
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a code review assistant. Analyze the code and provide feedback in JSON format.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// o1-series models have different API requirements
	isO1 := isO1Model(c.model)

	// Set token limits
	if options.MaxTokens > 0 {
		if isO1 {
			reqBody.MaxCompletionTokens = options.MaxTokens
		} else {
			reqBody.MaxTokens = options.MaxTokens
		}
	}

	// o1 models don't support temperature, seed, or response_format
	if !isO1 {
		reqBody.Temperature = options.Temperature
		reqBody.Seed = options.Seed
	}

	// Marshal request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute request with retry logic
	url := c.baseURL + "/v1/chat/completions"
	var response *APIResponse
	operation := func(ctx context.Context) error {
		// Recreate request for each retry with fresh body
		req, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if reqErr != nil {
			return fmt.Errorf("failed to create request: %w", reqErr)
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.client.Do(req)
		if err != nil {
			// Check if it's a timeout
			if ctx.Err() == context.DeadlineExceeded {
				return llmhttp.NewTimeoutError("openai", "request timed out")
			}
			return llmhttp.NewTimeoutError("openai", err.Error())
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// Check for errors
		if resp.StatusCode != http.StatusOK {
			return c.handleErrorResponse(resp.StatusCode, body)
		}

		// Parse success response
		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Validate response
		if len(chatResp.Choices) == 0 {
			return fmt.Errorf("no choices in response")
		}

		// Extract response data
		response = &APIResponse{
			Text:         chatResp.Choices[0].Message.Content,
			TokensIn:     chatResp.Usage.PromptTokens,
			TokensOut:    chatResp.Usage.CompletionTokens,
			Model:        chatResp.Model,
			FinishReason: chatResp.Choices[0].FinishReason,
		}

		return nil
	}

	// Execute with retry (using configured retry settings)
	err = llmhttp.RetryWithBackoff(ctx, operation, c.retryConf)
	duration := time.Since(startTime)

	if err != nil {
		// Log error
		if c.logger != nil {
			var httpErr *llmhttp.Error
			if errors.As(err, &httpErr) {
				c.logger.LogError(ctx, llmhttp.ErrorLog{
					Provider:   "openai",
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
				c.metrics.RecordError("openai", c.model, httpErr.Type)
			}
		}
		return nil, err
	}

	// Calculate cost
	var cost float64
	if c.pricing != nil {
		cost = c.pricing.GetCost("openai", c.model, response.TokensIn, response.TokensOut)
		response.Cost = cost
	}

	// Log response
	if c.logger != nil {
		c.logger.LogResponse(ctx, llmhttp.ResponseLog{
			Provider:     "openai",
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
		c.metrics.RecordDuration("openai", c.model, duration)
		c.metrics.RecordTokens("openai", c.model, response.TokensIn, response.TokensOut)
		c.metrics.RecordCost("openai", c.model, cost)
	}

	return response, nil
}

// handleErrorResponse converts HTTP error responses to typed errors.
func (c *HTTPClient) handleErrorResponse(statusCode int, body []byte) error {
	// Map status codes to error types first (before trying to parse body)
	// This ensures we handle errors correctly even without JSON body
	defaultMessage := fmt.Sprintf("HTTP %d", statusCode)

	// Try to parse OpenAI error format for better message
	var errResp ErrorResponse
	message := defaultMessage
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	} else if len(body) > 0 && len(body) < 200 {
		// If body is short and not JSON, use it as message
		message = string(body)
	}

	// Map status codes to error types
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return llmhttp.NewAuthenticationError("openai", message)
	case http.StatusTooManyRequests:
		return llmhttp.NewRateLimitError("openai", message)
	case http.StatusBadRequest:
		return llmhttp.NewInvalidRequestError("openai", message)
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return llmhttp.NewServiceUnavailableError("openai", message)
	default:
		return &llmhttp.Error{
			Type:       llmhttp.ErrTypeUnknown,
			Message:    message,
			StatusCode: statusCode,
			Retryable:  false,
			Provider:   "openai",
		}
	}
}

// CreateReview implements the Client interface for the Provider.
func (c *HTTPClient) CreateReview(ctx context.Context, req Request) (Response, error) {
	// Call the API
	apiResp, err := c.Call(ctx, req.Prompt, CallOptions{
		Temperature: 0.0, // Deterministic
		Seed:        &req.Seed,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return Response{}, err
	}

	// Parse the JSON response into domain types
	response, err := parseReviewJSON(apiResp.Text)
	if err != nil {
		// If JSON parsing fails, return a text summary with no findings
		return Response{
			Model:    apiResp.Model,
			Summary:  apiResp.Text,
			Findings: []domain.Finding{},
		}, nil
	}

	response.Model = apiResp.Model
	return response, nil
}

// parseReviewJSON extracts review data from JSON response.
func parseReviewJSON(text string) (Response, error) {
	// Use shared JSON parsing utility
	summary, findings, err := llmhttp.ParseReviewResponse(text)
	if err != nil {
		return Response{}, err
	}

	return Response{
		Summary:  summary,
		Findings: findings,
	}, nil
}

// Close cleans up resources.
func (c *HTTPClient) Close() error {
	// HTTP client doesn't need cleanup
	return nil
}
