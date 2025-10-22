package http_test

import (
	"testing"

	"github.com/brandon/code-reviewer/internal/adapter/llm/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractJSONFromMarkdown_JSONCodeBlock(t *testing.T) {
	markdown := "```json\n{\"summary\": \"test\", \"findings\": []}\n```"
	result := http.ExtractJSONFromMarkdown(markdown)

	expected := `{"summary": "test", "findings": []}`
	assert.Equal(t, expected, result)
}

func TestExtractJSONFromMarkdown_PlainCodeBlock(t *testing.T) {
	markdown := "```\n{\"summary\": \"test\", \"findings\": []}\n```"
	result := http.ExtractJSONFromMarkdown(markdown)

	expected := `{"summary": "test", "findings": []}`
	assert.Equal(t, expected, result)
}

func TestExtractJSONFromMarkdown_RawJSON(t *testing.T) {
	rawJSON := `{"summary": "test", "findings": []}`
	result := http.ExtractJSONFromMarkdown(rawJSON)

	// Should return trimmed input when no code block
	assert.Equal(t, rawJSON, result)
}

func TestExtractJSONFromMarkdown_EmptyString(t *testing.T) {
	result := http.ExtractJSONFromMarkdown("")
	assert.Equal(t, "", result)
}

func TestExtractJSONFromMarkdown_NoJSON(t *testing.T) {
	plainText := "This is just plain text without JSON"
	result := http.ExtractJSONFromMarkdown(plainText)

	// Should return trimmed input
	assert.Equal(t, plainText, result)
}

func TestExtractJSONFromMarkdown_MultipleCodeBlocks(t *testing.T) {
	markdown := "```json\n{\"first\": true}\n```\nSome text\n```json\n{\"second\": true}\n```"
	result := http.ExtractJSONFromMarkdown(markdown)

	// Should extract first code block
	expected := `{"first": true}`
	assert.Equal(t, expected, result)
}

func TestExtractJSONFromMarkdown_WithWhitespace(t *testing.T) {
	markdown := "```json\n\n  {\"summary\": \"test\"}  \n\n```"
	result := http.ExtractJSONFromMarkdown(markdown)

	// Should trim whitespace from extracted content
	expected := `{"summary": "test"}`
	assert.Equal(t, expected, result)
}

func TestExtractJSONFromMarkdown_NestedBackticks(t *testing.T) {
	// Test with content that has backticks inside
	markdown := "```json\n{\"code\": \"`value`\"}\n```"
	result := http.ExtractJSONFromMarkdown(markdown)

	expected := `{"code": "`+"`value`"+`"}`
	assert.Equal(t, expected, result)
}

func TestParseReviewResponse_ValidJSONInMarkdown(t *testing.T) {
	markdown := "```json\n{\"summary\": \"Good code\", \"findings\": [{\"file\": \"test.go\", \"lineStart\": 10, \"lineEnd\": 15, \"category\": \"style\", \"severity\": \"low\", \"description\": \"Test finding\", \"suggestion\": \"Fix it\", \"evidence\": true}]}\n```"

	summary, findings, err := http.ParseReviewResponse(markdown)
	require.NoError(t, err)

	assert.Equal(t, "Good code", summary)
	require.Len(t, findings, 1)
	assert.Equal(t, "test.go", findings[0].File)
	assert.Equal(t, 10, findings[0].LineStart)
	assert.Equal(t, "style", findings[0].Category)
}

func TestParseReviewResponse_RawJSON(t *testing.T) {
	rawJSON := `{"summary": "No issues", "findings": []}`

	summary, findings, err := http.ParseReviewResponse(rawJSON)
	require.NoError(t, err)

	assert.Equal(t, "No issues", summary)
	assert.Empty(t, findings)
}

func TestParseReviewResponse_InvalidJSON(t *testing.T) {
	invalidJSON := `{"summary": "missing closing brace"`

	_, _, err := http.ParseReviewResponse(invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON review")
}

func TestParseReviewResponse_MissingSummary(t *testing.T) {
	// JSON without summary field
	jsonWithoutSummary := `{"findings": []}`

	summary, findings, err := http.ParseReviewResponse(jsonWithoutSummary)
	require.NoError(t, err)

	assert.Equal(t, "", summary) // Empty string for missing field
	assert.Empty(t, findings)
}

func TestParseReviewResponse_MissingFindings(t *testing.T) {
	// JSON without findings field
	jsonWithoutFindings := `{"summary": "Test"}`

	summary, findings, err := http.ParseReviewResponse(jsonWithoutFindings)
	require.NoError(t, err)

	assert.Equal(t, "Test", summary)
	assert.Nil(t, findings) // nil for missing array
}

func TestParseReviewResponse_EmptyFindings(t *testing.T) {
	jsonWithEmptyFindings := `{"summary": "All good", "findings": []}`

	summary, findings, err := http.ParseReviewResponse(jsonWithEmptyFindings)
	require.NoError(t, err)

	assert.Equal(t, "All good", summary)
	assert.Empty(t, findings)
	assert.NotNil(t, findings) // Empty array, not nil
}

func TestParseReviewResponse_MultipleFindings(t *testing.T) {
	jsonWithMultipleFindings := `{
		"summary": "Found issues",
		"findings": [
			{
				"file": "main.go",
				"lineStart": 10,
				"lineEnd": 15,
				"category": "security",
				"severity": "high",
				"description": "SQL injection",
				"suggestion": "Use parameterized queries",
				"evidence": true
			},
			{
				"file": "util.go",
				"lineStart": 20,
				"lineEnd": 20,
				"category": "style",
				"severity": "low",
				"description": "Naming convention",
				"suggestion": "Use camelCase",
				"evidence": false
			}
		]
	}`

	summary, findings, err := http.ParseReviewResponse(jsonWithMultipleFindings)
	require.NoError(t, err)

	assert.Equal(t, "Found issues", summary)
	require.Len(t, findings, 2)

	// Check first finding
	assert.Equal(t, "main.go", findings[0].File)
	assert.Equal(t, "security", findings[0].Category)
	assert.Equal(t, "high", findings[0].Severity)

	// Check second finding
	assert.Equal(t, "util.go", findings[1].File)
	assert.Equal(t, "style", findings[1].Category)
	assert.Equal(t, "low", findings[1].Severity)
}

func TestParseReviewResponse_ComplexJSONInMarkdown(t *testing.T) {
	// Simulate real LLM response with explanation before JSON
	response := `Here's my code review:

The code looks good overall. I found a few minor issues.

` + "```json" + `
{
	"summary": "Code quality is good with minor improvements needed",
	"findings": [
		{
			"file": "server.go",
			"lineStart": 45,
			"lineEnd": 50,
			"category": "performance",
			"severity": "medium",
			"description": "Inefficient loop",
			"suggestion": "Use range instead of index",
			"evidence": true
		}
	]
}
` + "```" + `

Let me know if you have questions!`

	summary, findings, err := http.ParseReviewResponse(response)
	require.NoError(t, err)

	assert.Equal(t, "Code quality is good with minor improvements needed", summary)
	require.Len(t, findings, 1)
	assert.Equal(t, "server.go", findings[0].File)
	assert.Equal(t, "performance", findings[0].Category)
}
