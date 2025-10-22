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
	// Updated to handle nested code blocks: match from ```json (or ```) at start
	// to the LAST ``` in the text (greedy match), not the first
	jsonBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*)```")
)

// ExtractJSONFromMarkdown extracts JSON from markdown code blocks.
//
// Supports both ```json and ``` code blocks. Uses greedy matching to extract
// content from the first opening backticks to the LAST closing backticks.
//
// This greedy approach is necessary to handle nested code blocks within JSON
// content. For example, when LLM suggestions contain example code like:
//
//	"suggestion": "Use this code:\n\n```go\nfunc main() {}\n```"
//
// The greedy regex correctly extracts the entire JSON block by matching to the
// outermost closing backticks, not the inner ones from the code example.
//
// Assumption: LLMs are instructed to return a single JSON code block. If multiple
// separate code blocks are present, the greedy match will include all content
// between the first and last backticks, which may result in invalid JSON.
// This trade-off is acceptable for the typical LLM response patterns we observe.
//
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
