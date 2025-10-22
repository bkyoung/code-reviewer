package http_test

import (
	"errors"
	"testing"

	llmhttp "github.com/bkyoung/code-reviewer/internal/adapter/llm/http"
	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	err := &llmhttp.Error{
		Type:       llmhttp.ErrTypeAuthentication,
		Message:    "invalid API key",
		StatusCode: 401,
		Provider:   "openai",
	}

	expected := "openai: authentication error: invalid API key (status: 401)"
	assert.Equal(t, expected, err.Error())
}

func TestError_Is(t *testing.T) {
	err1 := &llmhttp.Error{Type: llmhttp.ErrTypeRateLimit, Message: "rate limited"}
	err2 := &llmhttp.Error{Type: llmhttp.ErrTypeRateLimit, Message: "different message"}
	err3 := &llmhttp.Error{Type: llmhttp.ErrTypeAuthentication, Message: "auth failed"}

	// Same type should match
	assert.True(t, errors.Is(err1, err2))

	// Different type should not match
	assert.False(t, errors.Is(err1, err3))
}

func TestError_Retryable(t *testing.T) {
	tests := []struct {
		name      string
		errType   llmhttp.ErrorType
		retryable bool
	}{
		{"rate limit is retryable", llmhttp.ErrTypeRateLimit, true},
		{"service unavailable is retryable", llmhttp.ErrTypeServiceUnavailable, true},
		{"timeout is retryable", llmhttp.ErrTypeTimeout, true},
		{"authentication is not retryable", llmhttp.ErrTypeAuthentication, false},
		{"invalid request is not retryable", llmhttp.ErrTypeInvalidRequest, false},
		{"content filtered is not retryable", llmhttp.ErrTypeContentFiltered, false},
		{"model not found is not retryable", llmhttp.ErrTypeModelNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &llmhttp.Error{
				Type:      tt.errType,
				Message:   "test error",
				Retryable: tt.retryable,
			}
			assert.Equal(t, tt.retryable, err.IsRetryable())
		})
	}
}

func TestNewAuthenticationError(t *testing.T) {
	err := llmhttp.NewAuthenticationError("openai", "invalid API key")

	assert.Equal(t, llmhttp.ErrTypeAuthentication, err.Type)
	assert.Equal(t, "invalid API key", err.Message)
	assert.Equal(t, "openai", err.Provider)
	assert.Equal(t, 401, err.StatusCode)
	assert.False(t, err.IsRetryable())
}

func TestNewRateLimitError(t *testing.T) {
	err := llmhttp.NewRateLimitError("anthropic", "too many requests")

	assert.Equal(t, llmhttp.ErrTypeRateLimit, err.Type)
	assert.Equal(t, "too many requests", err.Message)
	assert.Equal(t, "anthropic", err.Provider)
	assert.Equal(t, 429, err.StatusCode)
	assert.True(t, err.IsRetryable())
}

func TestNewServiceUnavailableError(t *testing.T) {
	err := llmhttp.NewServiceUnavailableError("gemini", "server overloaded")

	assert.Equal(t, llmhttp.ErrTypeServiceUnavailable, err.Type)
	assert.Equal(t, "server overloaded", err.Message)
	assert.Equal(t, "gemini", err.Provider)
	assert.Equal(t, 503, err.StatusCode)
	assert.True(t, err.IsRetryable())
}

func TestNewInvalidRequestError(t *testing.T) {
	err := llmhttp.NewInvalidRequestError("openai", "missing required field")

	assert.Equal(t, llmhttp.ErrTypeInvalidRequest, err.Type)
	assert.Equal(t, "missing required field", err.Message)
	assert.Equal(t, "openai", err.Provider)
	assert.Equal(t, 400, err.StatusCode)
	assert.False(t, err.IsRetryable())
}

func TestNewTimeoutError(t *testing.T) {
	err := llmhttp.NewTimeoutError("ollama", "request timed out after 60s")

	assert.Equal(t, llmhttp.ErrTypeTimeout, err.Type)
	assert.Equal(t, "request timed out after 60s", err.Message)
	assert.Equal(t, "ollama", err.Provider)
	assert.Equal(t, 0, err.StatusCode)
	assert.True(t, err.IsRetryable())
}

func TestNewModelNotFoundError(t *testing.T) {
	err := llmhttp.NewModelNotFoundError("ollama", "model 'codellama' not found")

	assert.Equal(t, llmhttp.ErrTypeModelNotFound, err.Type)
	assert.Equal(t, "model 'codellama' not found", err.Message)
	assert.Equal(t, "ollama", err.Provider)
	assert.Equal(t, 404, err.StatusCode)
	assert.False(t, err.IsRetryable())
}

func TestNewContentFilteredError(t *testing.T) {
	err := llmhttp.NewContentFilteredError("gemini", "content blocked by safety filters")

	assert.Equal(t, llmhttp.ErrTypeContentFiltered, err.Type)
	assert.Equal(t, "content blocked by safety filters", err.Message)
	assert.Equal(t, "gemini", err.Provider)
	assert.Equal(t, 400, err.StatusCode)
	assert.False(t, err.IsRetryable())
}

func TestErrorTypeString(t *testing.T) {
	tests := []struct {
		errType  llmhttp.ErrorType
		expected string
	}{
		{llmhttp.ErrTypeAuthentication, "authentication error"},
		{llmhttp.ErrTypeRateLimit, "rate limit exceeded"},
		{llmhttp.ErrTypeServiceUnavailable, "service unavailable"},
		{llmhttp.ErrTypeInvalidRequest, "invalid request"},
		{llmhttp.ErrTypeTimeout, "timeout"},
		{llmhttp.ErrTypeModelNotFound, "model not found"},
		{llmhttp.ErrTypeContentFiltered, "content filtered"},
		{llmhttp.ErrTypeUnknown, "unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.errType.String())
		})
	}
}
