package domain

import (
	"strings"
)

// AcknowledgeKeywords are phrases that indicate a finding is acknowledged but won't be fixed.
// These are checked case-insensitively.
var AcknowledgeKeywords = []string{
	"acknowledged",
	"won't fix",
	"wont fix",
	"intentional",
	"ack",
}

// DisputeKeywords are phrases that indicate a finding is contested.
// These are checked case-insensitively.
var DisputeKeywords = []string{
	"disputed",
	"false positive",
	"not a bug",
	"not an issue",
}

// ParseStatusKeyword analyzes text for status update keywords.
// Returns the detected status, the full text as reason (truncated to MaxStatusReasonLength),
// and whether a keyword was found.
//
// The function checks for keywords at word boundaries to avoid false positives
// like "I acknowledge your point" triggering on "acknowledge".
//
// Keywords are matched case-insensitively, but the original text is preserved
// as the reason to maintain user's formatting.
func ParseStatusKeyword(text string) (status FindingStatus, reason string, found bool) {
	if text == "" {
		return "", "", false
	}

	textLower := strings.ToLower(text)

	// Check acknowledge keywords first (they're more common)
	for _, keyword := range AcknowledgeKeywords {
		if containsKeyword(textLower, keyword) {
			return FindingStatusAcknowledged, truncateReason(text), true
		}
	}

	// Check dispute keywords
	for _, keyword := range DisputeKeywords {
		if containsKeyword(textLower, keyword) {
			return FindingStatusDisputed, truncateReason(text), true
		}
	}

	return "", "", false
}

// containsKeyword checks if text contains the keyword at a word boundary.
// This prevents "I acknowledge" from matching "ack" and similar false positives.
//
// A keyword is considered at a word boundary if:
// - It appears at the start/end of the text, OR
// - It's surrounded by non-alphanumeric characters
func containsKeyword(textLower, keyword string) bool {
	idx := strings.Index(textLower, keyword)
	if idx == -1 {
		return false
	}

	// Check character before keyword (if any)
	if idx > 0 {
		prevChar := textLower[idx-1]
		if isAlphanumeric(prevChar) {
			return false
		}
	}

	// Check character after keyword (if any)
	endIdx := idx + len(keyword)
	if endIdx < len(textLower) {
		nextChar := textLower[endIdx]
		if isAlphanumeric(nextChar) {
			return false
		}
	}

	return true
}

// isAlphanumeric returns true if the byte is a letter or digit.
func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// truncateReason truncates the reason to MaxStatusReasonLength characters.
// If truncation occurs, it tries to break at a word boundary for readability.
func truncateReason(reason string) string {
	if len(reason) <= MaxStatusReasonLength {
		return reason
	}

	// Truncate and try to find a word boundary
	truncated := reason[:MaxStatusReasonLength]

	// Look for last space within last 50 chars to break at word boundary
	lastSpace := strings.LastIndex(truncated[MaxStatusReasonLength-50:], " ")
	if lastSpace != -1 {
		return truncated[:MaxStatusReasonLength-50+lastSpace] + "..."
	}

	return truncated[:MaxStatusReasonLength-3] + "..."
}
