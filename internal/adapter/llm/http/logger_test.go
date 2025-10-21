package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brandon/code-reviewer/internal/adapter/llm/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultLogger(t *testing.T) {
	logger := http.NewDefaultLogger(http.LogLevelInfo, http.LogFormatHuman, true)
	assert.NotNil(t, logger)
}

func TestDefaultLogger_RedactAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "full key",
			key:      "sk-1234567890abcdef",
			expected: "****cdef",
		},
		{
			name:     "anthropic key",
			key:      "sk-ant-1234567890abcdef",
			expected: "****cdef",
		},
		{
			name:     "short key",
			key:      "abc",
			expected: "****",
		},
		{
			name:     "empty key",
			key:      "",
			expected: "****",
		},
		{
			name:     "4 char key",
			key:      "abcd",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := http.NewDefaultLogger(http.LogLevelDebug, http.LogFormatHuman, true)
			result := logger.RedactAPIKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultLogger_LogRequest_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelDebug, http.LogFormatHuman, true)
	logger.LogRequest(context.Background(), http.RequestLog{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Timestamp:   time.Now(),
		PromptChars: 1000,
		APIKey:      "sk-1234567890abcdef",
	})

	output := buf.String()
	assert.Contains(t, output, "[DEBUG]")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "gpt-4o-mini")
	assert.Contains(t, output, "1000")
	assert.Contains(t, output, "****cdef")
	assert.NotContains(t, output, "sk-1234567890abcdef")
}

func TestDefaultLogger_LogRequest_InfoLevel_Skipped(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelInfo, http.LogFormatHuman, true)
	logger.LogRequest(context.Background(), http.RequestLog{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Timestamp:   time.Now(),
		PromptChars: 1000,
		APIKey:      "sk-1234567890abcdef",
	})

	output := buf.String()
	assert.Empty(t, output, "Should not log at Info level")
}

func TestDefaultLogger_LogRequest_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelDebug, http.LogFormatJSON, true)
	now := time.Now()
	logger.LogRequest(context.Background(), http.RequestLog{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		Timestamp:   now,
		PromptChars: 1000,
		APIKey:      "sk-1234567890abcdef",
	})

	output := buf.String()

	// Extract JSON from log output (skip log prefix)
	jsonStart := strings.Index(output, "{")
	require.NotEqual(t, -1, jsonStart, "Should contain JSON")

	var logData map[string]interface{}
	err := json.Unmarshal([]byte(output[jsonStart:]), &logData)
	require.NoError(t, err)

	assert.Equal(t, "debug", logData["level"])
	assert.Equal(t, "request", logData["type"])
	assert.Equal(t, "openai", logData["provider"])
	assert.Equal(t, "gpt-4o-mini", logData["model"])
	assert.Equal(t, float64(1000), logData["prompt_chars"])
	assert.Equal(t, "****cdef", logData["api_key"])
}

func TestDefaultLogger_LogResponse(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelInfo, http.LogFormatHuman, true)
	logger.LogResponse(context.Background(), http.ResponseLog{
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		Timestamp:    time.Now(),
		Duration:     2500 * time.Millisecond,
		TokensIn:     100,
		TokensOut:    50,
		Cost:         0.0015,
		StatusCode:   200,
		FinishReason: "stop",
	})

	output := buf.String()
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "gpt-4o-mini")
	assert.Contains(t, output, "2.5")
	assert.Contains(t, output, "100")
	assert.Contains(t, output, "50")
	assert.Contains(t, output, "0.0015")
}

func TestDefaultLogger_LogResponse_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelInfo, http.LogFormatJSON, true)
	logger.LogResponse(context.Background(), http.ResponseLog{
		Provider:     "anthropic",
		Model:        "claude-3-5-sonnet-20241022",
		Timestamp:    time.Now(),
		Duration:     3200 * time.Millisecond,
		TokensIn:     200,
		TokensOut:    150,
		Cost:         0.0028,
		StatusCode:   200,
		FinishReason: "end_turn",
	})

	output := buf.String()
	jsonStart := strings.Index(output, "{")
	require.NotEqual(t, -1, jsonStart)

	var logData map[string]interface{}
	err := json.Unmarshal([]byte(output[jsonStart:]), &logData)
	require.NoError(t, err)

	assert.Equal(t, "info", logData["level"])
	assert.Equal(t, "response", logData["type"])
	assert.Equal(t, "anthropic", logData["provider"])
	assert.Equal(t, float64(200), logData["tokens_in"])
	assert.Equal(t, float64(150), logData["tokens_out"])
	assert.Equal(t, 0.0028, logData["cost"])
}

func TestDefaultLogger_LogError(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelError, http.LogFormatHuman, true)

	err := &http.Error{
		Type:       http.ErrTypeRateLimit,
		Message:    "Rate limit exceeded",
		StatusCode: 429,
		Retryable:  true,
		Provider:   "openai",
	}

	logger.LogError(context.Background(), http.ErrorLog{
		Provider:   "openai",
		Model:      "gpt-4o-mini",
		Timestamp:  time.Now(),
		Duration:   1500 * time.Millisecond,
		Error:      err,
		ErrorType:  http.ErrTypeRateLimit,
		StatusCode: 429,
		Retryable:  true,
	})

	output := buf.String()
	assert.Contains(t, output, "[ERROR]")
	assert.Contains(t, output, "openai")
	assert.Contains(t, output, "gpt-4o-mini")
	assert.Contains(t, output, "429")
	assert.Contains(t, output, "retryable")
}

func TestDefaultLogger_LogError_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := http.NewDefaultLogger(http.LogLevelError, http.LogFormatJSON, true)

	err := &http.Error{
		Type:       http.ErrTypeAuthentication,
		Message:    "Invalid API key",
		StatusCode: 401,
		Retryable:  false,
		Provider:   "gemini",
	}

	logger.LogError(context.Background(), http.ErrorLog{
		Provider:   "gemini",
		Model:      "gemini-1.5-pro",
		Timestamp:  time.Now(),
		Duration:   500 * time.Millisecond,
		Error:      err,
		ErrorType:  http.ErrTypeAuthentication,
		StatusCode: 401,
		Retryable:  false,
	})

	output := buf.String()
	jsonStart := strings.Index(output, "{")
	require.NotEqual(t, -1, jsonStart)

	var logData map[string]interface{}
	err2 := json.Unmarshal([]byte(output[jsonStart:]), &logData)
	require.NoError(t, err2)

	assert.Equal(t, "error", logData["level"])
	assert.Equal(t, "error", logData["type"])
	assert.Equal(t, "gemini", logData["provider"])
	assert.Equal(t, float64(401), logData["status_code"])
	assert.Equal(t, false, logData["retryable"])
}

func TestDefaultLogger_NoRedaction_WhenDisabled(t *testing.T) {
	logger := http.NewDefaultLogger(http.LogLevelDebug, http.LogFormatHuman, true)
	logger.SetRedaction(false)

	result := logger.RedactAPIKey("sk-1234567890abcdef")
	assert.Equal(t, "sk-1234567890abcdef", result, "Should not redact when disabled")
}
