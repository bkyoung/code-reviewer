package github

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// ReviewActions configures the GitHub review action for each finding severity level.
// This mirrors config.ReviewActions but lives in the adapter layer to avoid coupling.
type ReviewActions struct {
	OnCritical string // Action for critical severity findings
	OnHigh     string // Action for high severity findings
	OnMedium   string // Action for medium severity findings
	OnLow      string // Action for low severity findings
	OnClean    string // Action when no findings in diff
}

// NormalizeAction converts a string action to ReviewEvent.
// It handles case-insensitive input and common variations.
// Returns (event, true) if valid, (EventComment, false) if invalid.
func NormalizeAction(action string) (ReviewEvent, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(action))
	// Handle hyphenated variant
	normalized = strings.ReplaceAll(normalized, "-", "_")

	switch normalized {
	case "APPROVE":
		return EventApprove, true
	case "REQUEST_CHANGES":
		return EventRequestChanges, true
	case "COMMENT":
		return EventComment, true
	default:
		return EventComment, false
	}
}

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
// This function uses the default review actions (legacy behavior).
// Returns:
//   - EventApprove if no findings (in diff)
//   - EventRequestChanges if any high or critical severity findings (in diff)
//   - EventComment otherwise
func DetermineReviewEvent(findings []PositionedFinding) ReviewEvent {
	// Use empty ReviewActions to trigger default/fallback behavior
	return DetermineReviewEventWithActions(findings, ReviewActions{})
}

// DetermineReviewEventWithActions determines the appropriate ReviewEvent based on
// finding severities and the provided action configuration.
// Returns:
//   - Configured action for the highest severity found in diff
//   - OnClean action if no findings in diff
//   - Fallback to sensible defaults if action is not configured
func DetermineReviewEventWithActions(findings []PositionedFinding, actions ReviewActions) ReviewEvent {
	inDiffFindings := filterInDiff(findings)

	// No findings = clean code
	if len(inDiffFindings) == 0 {
		if actions.OnClean != "" {
			if event, valid := NormalizeAction(actions.OnClean); valid {
				return event
			}
		}
		return EventApprove // default for clean
	}

	// Find highest severity present
	severityOrder := []string{"critical", "high", "medium", "low"}
	actionMap := map[string]string{
		"critical": actions.OnCritical,
		"high":     actions.OnHigh,
		"medium":   actions.OnMedium,
		"low":      actions.OnLow,
	}
	defaultMap := map[string]ReviewEvent{
		"critical": EventRequestChanges,
		"high":     EventRequestChanges,
		"medium":   EventComment,
		"low":      EventComment,
	}

	for _, severity := range severityOrder {
		// Check if any finding has this severity
		for _, pf := range inDiffFindings {
			if strings.ToLower(pf.Finding.Severity) == severity {
				// Found this severity - use configured action or default
				if action := actionMap[severity]; action != "" {
					if event, valid := NormalizeAction(action); valid {
						return event
					}
				}
				// Fallback to default for this severity
				return defaultMap[severity]
			}
		}
	}

	// No known severity found - treat as comment
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
