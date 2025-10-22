package http

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Logger provides structured logging for LLM API calls.
type Logger interface {
	// LogRequest logs an outgoing API request (API key redacted)
	LogRequest(ctx context.Context, req RequestLog)

	// LogResponse logs an API response with timing and token info
	LogResponse(ctx context.Context, resp ResponseLog)

	// LogError logs an API error
	LogError(ctx context.Context, err ErrorLog)
}

// RequestLog contains request information for logging.
type RequestLog struct {
	Provider    string
	Model       string
	Timestamp   time.Time
	PromptChars int    // Character count of prompt
	APIKey      string // Will be redacted to last 4 chars
}

// ResponseLog contains response information for logging.
type ResponseLog struct {
	Provider     string
	Model        string
	Timestamp    time.Time
	Duration     time.Duration
	TokensIn     int
	TokensOut    int
	Cost         float64
	StatusCode   int
	FinishReason string
}

// ErrorLog contains error information for logging.
type ErrorLog struct {
	Provider   string
	Model      string
	Timestamp  time.Time
	Duration   time.Duration
	Error      error
	ErrorType  ErrorType
	StatusCode int
	Retryable  bool
}

// LogLevel defines the logging verbosity level.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelError
)

// LogFormat defines the output format for logs.
type LogFormat int

const (
	LogFormatHuman LogFormat = iota
	LogFormatJSON
)

// DefaultLogger writes logs in structured format to stdout.
type DefaultLogger struct {
	level      LogLevel
	redactKeys bool
	format     LogFormat
}

// NewDefaultLogger creates a logger with the specified config.
func NewDefaultLogger(level LogLevel, format LogFormat, redactKeys bool) *DefaultLogger {
	return &DefaultLogger{
		level:      level,
		redactKeys: redactKeys,
		format:     format,
	}
}

// SetRedaction enables or disables API key redaction.
func (l *DefaultLogger) SetRedaction(enabled bool) {
	l.redactKeys = enabled
}

// LogRequest logs an API request.
func (l *DefaultLogger) LogRequest(ctx context.Context, req RequestLog) {
	if l.level > LogLevelDebug {
		return
	}

	// Redact API key to last 4 characters
	redacted := l.RedactAPIKey(req.APIKey)

	if l.format == LogFormatJSON {
		// JSON format for machine parsing
		log.Printf(`{"level":"debug","type":"request","provider":"%s","model":"%s","timestamp":"%s","prompt_chars":%d,"api_key":"%s"}`,
			req.Provider, req.Model, req.Timestamp.Format(time.RFC3339),
			req.PromptChars, redacted)
	} else {
		// Human-readable format
		log.Printf("[DEBUG] %s/%s: Request sent (prompt=%d chars, key=%s)",
			req.Provider, req.Model, req.PromptChars, redacted)
	}
}

// LogResponse logs an API response.
func (l *DefaultLogger) LogResponse(ctx context.Context, resp ResponseLog) {
	if l.level > LogLevelInfo {
		return
	}

	if l.format == LogFormatJSON {
		// JSON format for machine parsing
		log.Printf(`{"level":"info","type":"response","provider":"%s","model":"%s","timestamp":"%s","duration_ms":%d,"tokens_in":%d,"tokens_out":%d,"cost":%.6f,"status_code":%d,"finish_reason":"%s"}`,
			resp.Provider, resp.Model, resp.Timestamp.Format(time.RFC3339),
			resp.Duration.Milliseconds(), resp.TokensIn, resp.TokensOut,
			resp.Cost, resp.StatusCode, resp.FinishReason)
	} else {
		// Human-readable format
		log.Printf("[INFO] %s/%s: Response received (duration=%.1fs, tokens=%d/%d, cost=$%.4f)",
			resp.Provider, resp.Model, resp.Duration.Seconds(),
			resp.TokensIn, resp.TokensOut, resp.Cost)
	}
}

// LogError logs an API error.
func (l *DefaultLogger) LogError(ctx context.Context, err ErrorLog) {
	if l.level > LogLevelError {
		return
	}

	retryableStr := "non-retryable"
	if err.Retryable {
		retryableStr = "retryable"
	}

	if l.format == LogFormatJSON {
		// JSON format for machine parsing
		log.Printf(`{"level":"error","type":"error","provider":"%s","model":"%s","timestamp":"%s","duration_ms":%d,"error":"%s","error_type":%d,"status_code":%d,"retryable":%t}`,
			err.Provider, err.Model, err.Timestamp.Format(time.RFC3339),
			err.Duration.Milliseconds(), err.Error.Error(), err.ErrorType,
			err.StatusCode, err.Retryable)
	} else {
		// Human-readable format
		log.Printf("[ERROR] %s/%s: API call failed (status=%d, %s): %v",
			err.Provider, err.Model, err.StatusCode, retryableStr, err.Error)
	}
}

// RedactAPIKey shows only the last 4 characters of an API key with explicit redaction markers.
func (l *DefaultLogger) RedactAPIKey(key string) string {
	if !l.redactKeys {
		return key
	}
	if len(key) <= 4 {
		return "[REDACTED]"
	}
	return fmt.Sprintf("[REDACTED-%s]", key[len(key)-4:])
}
