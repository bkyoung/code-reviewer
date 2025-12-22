package github

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// BuildReviewComments converts positioned findings to GitHub review comments.
// Only findings with a valid DiffPosition (InDiff() == true) are included.
// This function is pure and does not modify the input.
func BuildReviewComments(findings []PositionedFinding) []ReviewComment {
	var comments []ReviewComment

	for _, pf := range findings {
		if !pf.InDiff() {
			continue
		}

		comments = append(comments, ReviewComment{
			Path:     pf.Finding.File,
			Position: *pf.DiffPosition,
			Body:     FormatFindingComment(pf.Finding),
		})
	}

	return comments
}

// FormatFindingComment formats a domain.Finding as a GitHub-flavored Markdown comment.
func FormatFindingComment(f domain.Finding) string {
	var sb strings.Builder

	// Header with severity and category
	sb.WriteString(fmt.Sprintf("**Severity:** %s", f.Severity))
	if f.Category != "" {
		sb.WriteString(fmt.Sprintf(" | **Category:** %s", f.Category))
	}
	sb.WriteString("\n\n")

	// Line reference
	if f.LineStart == f.LineEnd || f.LineEnd == 0 {
		sb.WriteString(fmt.Sprintf("📍 Line %d\n\n", f.LineStart))
	} else {
		sb.WriteString(fmt.Sprintf("📍 Lines %d-%d\n\n", f.LineStart, f.LineEnd))
	}

	// Description
	sb.WriteString(f.Description)
	sb.WriteString("\n")

	// Suggestion if present
	if f.Suggestion != "" {
		sb.WriteString("\n**Suggestion:** ")
		sb.WriteString(f.Suggestion)
		sb.WriteString("\n")
	}

	return sb.String()
}

// DetermineReviewEvent determines the appropriate ReviewEvent based on finding severities.
// Returns:
//   - EventApprove if no findings (in diff)
//   - EventRequestChanges if any high or critical severity findings (in diff)
//   - EventComment otherwise
func DetermineReviewEvent(findings []PositionedFinding) ReviewEvent {
	inDiffFindings := filterInDiff(findings)

	if len(inDiffFindings) == 0 {
		return EventApprove
	}

	for _, pf := range inDiffFindings {
		sev := strings.ToLower(pf.Finding.Severity)
		if sev == "high" || sev == "critical" {
			return EventRequestChanges
		}
	}

	return EventComment
}

// CountInDiffFindings returns the count of findings that are in the diff.
func CountInDiffFindings(findings []PositionedFinding) int {
	count := 0
	for _, pf := range findings {
		if pf.InDiff() {
			count++
		}
	}
	return count
}

// filterInDiff returns only findings that are in the diff.
func filterInDiff(findings []PositionedFinding) []PositionedFinding {
	var result []PositionedFinding
	for _, pf := range findings {
		if pf.InDiff() {
			result = append(result, pf)
		}
	}
	return result
}
