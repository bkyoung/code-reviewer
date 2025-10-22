package observability

import (
	"context"
	"log"

	llmhttp "github.com/brandon/code-reviewer/internal/adapter/llm/http"
	"github.com/brandon/code-reviewer/internal/usecase/review"
)

// ReviewLogger adapts llmhttp.Logger to review.Logger interface.
// This allows the review orchestrator to use the same structured logging
// infrastructure as the LLM HTTP clients.
type ReviewLogger struct {
	logger llmhttp.Logger
}

// NewReviewLogger creates a new review logger adapter.
func NewReviewLogger(logger llmhttp.Logger) review.Logger {
	return &ReviewLogger{logger: logger}
}

// LogWarning logs a warning message with structured fields.
// Since llmhttp.Logger doesn't have a generic warning method, we use
// the standard log package with structured formatting when available.
func (l *ReviewLogger) LogWarning(ctx context.Context, message string, fields map[string]interface{}) {
	// Format the fields into a log message
	log.Printf("warning: %s %v", message, fields)
}

// LogInfo logs an informational message with structured fields.
// Since llmhttp.Logger doesn't have a generic info method, we use
// the standard log package with structured formatting when available.
func (l *ReviewLogger) LogInfo(ctx context.Context, message string, fields map[string]interface{}) {
	// Format the fields into a log message
	log.Printf("info: %s %v", message, fields)
}
