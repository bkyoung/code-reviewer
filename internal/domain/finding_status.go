package domain

import (
	"strings"
	"unicode"
)

// FindingStatus represents the status of a finding based on reply analysis.
type FindingStatus string

const (
	// StatusOpen indicates no reply or unrecognized reply - finding is unresolved.
	StatusOpen FindingStatus = "open"

	// StatusAcknowledged indicates the author acknowledged the finding but won't fix it.
	StatusAcknowledged FindingStatus = "acknowledged"

	// StatusDisputed indicates the author disputes the finding as incorrect.
	StatusDisputed FindingStatus = "disputed"
)

// acknowledgmentKeywords are phrases that indicate the author acknowledges a finding.
// These are checked with word/phrase boundary matching.
var acknowledgmentKeywords = []string{
	// Explicit acknowledgment
	"acknowledged",
	"ack",
	// Won't fix variants
	"won't fix",
	"wont fix",
	"will not fix",
	"wontfix",
	// Intentional/by design
	"intentional",
	"by design",
	"as designed",
	"working as intended",
	"works as intended",
	"working as designed",
	"works as designed",
	// Positive acknowledgment
	"good catch",
	"good point",
	"fair point",
	"valid point",
	"thanks",
	"thank you",
	"agreed",
	"valid",
	"noted",
	// Deferred
	"will fix later",
	"fix later",
	"tracking",
	"tracked",
	"known issue",
}

// disputeKeywords are phrases that indicate the author disputes a finding.
// These are checked with word/phrase boundary matching.
var disputeKeywords = []string{
	// False positive
	"false positive",
	"not an issue",
	"not a bug",
	"not a problem",
	// Disagreement
	"disagree",
	"disputed",
	"incorrect",
	"wrong",
	// Expected behavior
	"expected behavior",
	"expected behaviour",
	"expected result",
	"intended behavior",
	"intended behaviour",
	// Rejection
	"not applicable",
	"n/a",
	"doesn't apply",
	"does not apply",
}

// DetectStatusFromText analyzes text for status keywords and returns the detected status.
// Returns StatusDisputed if dispute keywords are found (checked first as it's more specific).
// Returns StatusAcknowledged if acknowledgment keywords are found.
// Returns StatusOpen if no keywords are found.
//
// Matching is case-insensitive and uses word/phrase boundary detection to avoid
// false positives from partial matches within words.
func DetectStatusFromText(text string) FindingStatus {
	normalized := strings.ToLower(text)

	// Check dispute keywords first (more specific rejection)
	for _, keyword := range disputeKeywords {
		if containsPhrase(normalized, keyword) {
			return StatusDisputed
		}
	}

	// Check acknowledgment keywords
	for _, keyword := range acknowledgmentKeywords {
		if containsPhrase(normalized, keyword) {
			return StatusAcknowledged
		}
	}

	return StatusOpen
}

// DetectStatusFromReplies analyzes multiple reply texts and returns the aggregate status.
// If any reply contains dispute keywords, returns StatusDisputed.
// If any reply contains acknowledgment keywords, returns StatusAcknowledged.
// Otherwise returns StatusOpen.
func DetectStatusFromReplies(replies []string) FindingStatus {
	hasAcknowledgment := false

	for _, reply := range replies {
		status := DetectStatusFromText(reply)
		switch status {
		case StatusDisputed:
			// Dispute takes precedence - return immediately
			return StatusDisputed
		case StatusAcknowledged:
			hasAcknowledgment = true
		}
	}

	if hasAcknowledgment {
		return StatusAcknowledged
	}

	return StatusOpen
}

// containsPhrase checks if text contains the phrase with word boundaries.
// A word boundary is either the start/end of string or a non-alphanumeric character.
func containsPhrase(text, phrase string) bool {
	idx := strings.Index(text, phrase)
	if idx == -1 {
		return false
	}

	// Check left boundary
	if idx > 0 {
		prevChar := rune(text[idx-1])
		if isWordChar(prevChar) {
			// Not at word boundary, search for next occurrence
			return containsPhraseFrom(text, phrase, idx+1)
		}
	}

	// Check right boundary
	endIdx := idx + len(phrase)
	if endIdx < len(text) {
		nextChar := rune(text[endIdx])
		if isWordChar(nextChar) {
			// Not at word boundary, search for next occurrence
			return containsPhraseFrom(text, phrase, idx+1)
		}
	}

	return true
}

// containsPhraseFrom searches for phrase starting from offset.
func containsPhraseFrom(text, phrase string, offset int) bool {
	if offset >= len(text) {
		return false
	}
	remaining := text[offset:]
	idx := strings.Index(remaining, phrase)
	if idx == -1 {
		return false
	}
	// Recurse with absolute position
	return containsPhrase(text[offset:], phrase)
}

// isWordChar returns true if the rune is a letter, digit, or underscore.
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
