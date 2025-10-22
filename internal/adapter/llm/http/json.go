package http

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

var (
	// Compile regex once and reuse (thread-safe)
	jsonBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")
)

// ExtractJSONFromMarkdown extracts JSON from markdown code blocks.
// Supports both ```json and ``` code blocks.
// Returns extracted JSON or original text if no code block found.
func ExtractJSONFromMarkdown(text string) string {
	matches := jsonBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	// No code block found, return original text (might be raw JSON)
	return strings.TrimSpace(text)
}

// ParseReviewResponse parses JSON into a structured review response.
// Handles both markdown-wrapped and raw JSON responses.
func ParseReviewResponse(text string) (summary string, findings []domain.Finding, err error) {
	// Extract JSON from markdown if present
	jsonText := ExtractJSONFromMarkdown(text)

	// Parse into intermediate structure
	var result struct {
		Summary  string           `json:"summary"`
		Findings []domain.Finding `json:"findings"`
	}

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse JSON review: %w", err)
	}

	return result.Summary, result.Findings, nil
}
