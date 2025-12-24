package github

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// fingerprintMarkerStart is the HTML comment prefix for embedding fingerprints.
// This marker is invisible in rendered markdown but can be extracted for reply matching.
const fingerprintMarkerStart = "<!-- CR_FINGERPRINT:"

// fingerprintMarkerEnd closes the HTML comment containing the fingerprint.
const fingerprintMarkerEnd = " -->"

// ReviewActions configures the GitHub review action for each finding severity level.
// This mirrors config.ReviewActions but lives in the adapter layer to avoid coupling.
type ReviewActions struct {
	OnCritical    string // Action for critical severity findings
	OnHigh        string // Action for high severity findings
	OnMedium      string // Action for medium severity findings
	OnLow         string // Action for low severity findings
	OnClean       string // Action when no findings in diff
	OnNonBlocking string // Action when findings exist but none trigger REQUEST_CHANGES
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

// defaultBlockingSeverities defines which severities trigger REQUEST_CHANGES by default.
// Critical and high block by default; medium and low do not.
var defaultBlockingSeverities = map[string]bool{
	"critical": true,
	"high":     true,
	"medium":   false,
	"low":      false,
}

// BuildReviewComments converts positioned findings to GitHub review comments.
// Only findings with a valid DiffPosition (InDiff() == true) are included.
// Each comment includes an embedded fingerprint for linking replies to findings.
// This function is pure and does not modify the input.
func BuildReviewComments(findings []PositionedFinding) []ReviewComment {
	var comments []ReviewComment

	for _, pf := range findings {
		if !pf.InDiff() {
			continue
		}

		// Compute fingerprint and embed in comment body
		fingerprint := domain.FingerprintFromFinding(pf.Finding)
		body := FormatFindingCommentWithFingerprint(pf.Finding, fingerprint)

		comments = append(comments, ReviewComment{
			Path:     pf.Finding.File,
			Position: *pf.DiffPosition,
			Body:     body,
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

// FormatFindingCommentWithFingerprint formats a finding as a GitHub comment with embedded fingerprint.
// The fingerprint is stored in an HTML comment that is invisible when rendered but can be
// extracted to link replies back to the original finding.
func FormatFindingCommentWithFingerprint(f domain.Finding, fingerprint domain.FindingFingerprint) string {
	var sb strings.Builder

	// Start with the standard format
	sb.WriteString(FormatFindingComment(f))

	// Add fingerprint in hidden HTML comment
	sb.WriteString("\n")
	sb.WriteString(fingerprintMarkerStart)
	sb.WriteString(string(fingerprint))
	sb.WriteString(fingerprintMarkerEnd)

	return sb.String()
}

// fingerprintLength is the expected length of a fingerprint (32 hex chars from 16 bytes of SHA256).
const fingerprintLength = 32

// ExtractFingerprintFromComment extracts the finding fingerprint from a GitHub comment body.
// Returns the fingerprint and true if found and valid, or empty and false if not present or invalid.
// This is used to link reply comments back to their original findings.
//
// The function validates that the extracted fingerprint matches the expected format
// (32-character hexadecimal string) to prevent injection of arbitrary strings.
func ExtractFingerprintFromComment(body string) (domain.FindingFingerprint, bool) {
	startIdx := strings.Index(body, fingerprintMarkerStart)
	if startIdx == -1 {
		return "", false
	}

	// Find the content start (after the marker)
	contentStart := startIdx + len(fingerprintMarkerStart)

	// Find the end marker
	remaining := body[contentStart:]
	endIdx := strings.Index(remaining, fingerprintMarkerEnd)
	if endIdx == -1 {
		return "", false
	}

	// Extract and trim the fingerprint
	fp := strings.TrimSpace(remaining[:endIdx])
	if fp == "" {
		return "", false
	}

	// Validate fingerprint format: must be exactly 32 hexadecimal characters
	if !isValidFingerprint(fp) {
		return "", false
	}

	return domain.FindingFingerprint(fp), true
}

// isValidFingerprint checks if a string is a valid fingerprint format.
// A valid fingerprint is exactly 32 lowercase hexadecimal characters.
func isValidFingerprint(fp string) bool {
	if len(fp) != fingerprintLength {
		return false
	}
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
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
//
// Returns:
//   - OnClean action if no findings in diff (default: APPROVE)
//   - OnNonBlocking action if findings exist but none trigger REQUEST_CHANGES (default: APPROVE)
//   - REQUEST_CHANGES if any in-diff finding triggers it based on severity config
//     (default: critical/high block, medium/low don't; configurable via OnCritical, etc.)
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

	// Check if any in-diff finding would trigger REQUEST_CHANGES
	if HasBlockingFindings(inDiffFindings, actions) {
		return EventRequestChanges
	}

	// No blocking findings - use OnNonBlocking action
	if actions.OnNonBlocking != "" {
		if event, valid := NormalizeAction(actions.OnNonBlocking); valid {
			return event
		}
	}
	// Default for non-blocking: APPROVE (findings exist but are informational)
	return EventApprove
}

// wouldTriggerRequestChanges checks if the given action would result in REQUEST_CHANGES.
// If action is empty, uses defaultBlocking. If action is invalid (typo/unknown),
// falls back to defaultBlocking to prevent accidental approval from config errors.
func wouldTriggerRequestChanges(action string, defaultBlocking bool) bool {
	if action == "" {
		return defaultBlocking
	}
	event, valid := NormalizeAction(action)
	if !valid {
		// Invalid action string (typo) - fall back to default behavior
		// to prevent accidental approval from config errors
		return defaultBlocking
	}
	return event == EventRequestChanges
}

// HasBlockingFindings checks if any in-diff finding would trigger REQUEST_CHANGES
// based on the provided action configuration.
// This is exported so that summary_builder can use the same logic to determine
// whether to show "Approved with suggestions" prefix.
func HasBlockingFindings(findings []PositionedFinding, actions ReviewActions) bool {
	inDiffFindings := filterInDiff(findings)
	if len(inDiffFindings) == 0 {
		return false
	}

	// Build action map from configuration
	actionMap := map[string]string{
		"critical": actions.OnCritical,
		"high":     actions.OnHigh,
		"medium":   actions.OnMedium,
		"low":      actions.OnLow,
	}

	// Check each finding's severity against blocking configuration
	for _, pf := range inDiffFindings {
		severity := strings.ToLower(pf.Finding.Severity)
		defaultBlocking, known := defaultBlockingSeverities[severity]
		if !known {
			// Unknown severities don't block by default
			continue
		}
		if wouldTriggerRequestChanges(actionMap[severity], defaultBlocking) {
			return true
		}
	}
	return false
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
