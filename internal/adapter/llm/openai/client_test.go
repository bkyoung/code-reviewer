package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	llmhttp "github.com/brandon/code-reviewer/internal/adapter/llm/http"
	"github.com/brandon/code-reviewer/internal/adapter/llm/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	client := openai.NewHTTPClient("test-api-key", "gpt-4o-mini")

	assert.NotNil(t, client)
}

func TestNewHTTPClient_EmptyAPIKey(t *testing.T) {
	client := openai.NewHTTPClient("", "gpt-4o-mini")

	// Should still create client, but will fail on actual API calls
	assert.NotNil(t, client)
}

func TestHTTPClient_Call_Success(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var req openai.ChatCompletionRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "gpt-4o-mini", req.Model)
		assert.Equal(t, 0.0, req.Temperature)
		assert.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "user", req.Messages[1].Role)

		// Send response
		resp := openai.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []openai.Choice{
				{
					Index: 0,
					Message: openai.Message{
						Role:    "assistant",
						Content: `{"summary": "Code looks good", "findings": []}`,
					},
					FinishReason: "stop",
				},
			},
			Usage: openai.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-api-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	response, err := client.Call(context.Background(), "test prompt", openai.CallOptions{
		Temperature: 0.0,
		Seed:        uint64Ptr(12345),
		MaxTokens:   16384,
	})

	require.NoError(t, err)
	assert.Equal(t, `{"summary": "Code looks good", "findings": []}`, response.Text)
	assert.Equal(t, 100, response.TokensIn)
	assert.Equal(t, 50, response.TokensOut)
	assert.Equal(t, "gpt-4o-mini", response.Model)
	assert.Equal(t, "stop", response.FinishReason)
}

func TestHTTPClient_Call_AuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(openai.ErrorResponse{
			Error: openai.ErrorDetail{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
				Code:    "invalid_api_key",
			},
		})
	}))
	defer server.Close()

	client := openai.NewHTTPClient("invalid-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	_, err := client.Call(context.Background(), "test prompt", openai.CallOptions{})

	require.Error(t, err)
	var httpErr *llmhttp.Error
	assert.ErrorAs(t, err, &httpErr)
	assert.Equal(t, llmhttp.ErrTypeAuthentication, httpErr.Type)
	assert.Contains(t, httpErr.Message, "Invalid API key")
}

func TestHTTPClient_Call_RateLimitError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Return rate limit error first 2 times
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(openai.ErrorResponse{
				Error: openai.ErrorDetail{
					Message: "Rate limit exceeded",
					Type:    "rate_limit_error",
				},
			})
			return
		}

		// Success on 3rd attempt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []openai.Choice{
				{
					Index: 0,
					Message: openai.Message{
						Role:    "assistant",
						Content: "success",
					},
					FinishReason: "stop",
				},
			},
			Usage: openai.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		})
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	response, err := client.Call(context.Background(), "test", openai.CallOptions{})

	require.NoError(t, err, "should succeed after retries")
	assert.Equal(t, "success", response.Text)
	assert.Equal(t, 3, attempts, "should have retried twice")
}

func TestHTTPClient_Call_ServiceUnavailable(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// Success on 2nd attempt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []openai.Choice{
				{
					Message:      openai.Message{Role: "assistant", Content: "ok"},
					FinishReason: "stop",
				},
			},
			Usage: openai.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
		})
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	response, err := client.Call(context.Background(), "test", openai.CallOptions{})

	require.NoError(t, err)
	assert.Equal(t, "ok", response.Text)
	assert.Equal(t, 2, attempts)
}

func TestHTTPClient_Call_InvalidRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(openai.ErrorResponse{
			Error: openai.ErrorDetail{
				Message: "Invalid request",
				Type:    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	_, err := client.Call(context.Background(), "test", openai.CallOptions{})

	require.Error(t, err)
	var httpErr *llmhttp.Error
	assert.ErrorAs(t, err, &httpErr)
	assert.Equal(t, llmhttp.ErrTypeInvalidRequest, httpErr.Type)
	assert.False(t, httpErr.IsRetryable(), "invalid request should not be retryable")
}

func TestHTTPClient_Call_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Longer than timeout
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)
	client.SetTimeout(50 * time.Millisecond)

	_, err := client.Call(context.Background(), "test", openai.CallOptions{})

	require.Error(t, err)
	var httpErr *llmhttp.Error
	assert.ErrorAs(t, err, &httpErr)
	assert.Equal(t, llmhttp.ErrTypeTimeout, httpErr.Type)
}

func TestHTTPClient_Call_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Call(ctx, "test", openai.CallOptions{})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestHTTPClient_Call_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	_, err := client.Call(context.Background(), "test", openai.CallOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestHTTPClient_Call_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []openai.Choice{}, // Empty choices
			Usage:   openai.Usage{PromptTokens: 10, CompletionTokens: 0, TotalTokens: 10},
		})
	}))
	defer server.Close()

	client := openai.NewHTTPClient("test-key", "gpt-4o-mini")
	client.SetBaseURL(server.URL)

	_, err := client.Call(context.Background(), "test", openai.CallOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no choices in response")
}

// Helper function
func uint64Ptr(v uint64) *uint64 {
	return &v
}
