package domain

import (
	"strings"
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
// A word boundary is either the start/end of string or a non-word byte.
//
// This implementation uses byte-based boundary checks because all keywords
// are ASCII. This avoids UTF-8 multi-byte issues while remaining correct
// for the intended use case (English keywords in code review comments).
func containsPhrase(text, phrase string) bool {
	// Guard against empty phrase (strings.Index returns 0 for empty)
	if phrase == "" {
		return false
	}

	// Iterative search: find all occurrences and check boundaries
	searchStart := 0
	for searchStart <= len(text)-len(phrase) {
		idx := strings.Index(text[searchStart:], phrase)
		if idx == -1 {
			return false
		}

		// Convert to absolute index
		absIdx := searchStart + idx
		endIdx := absIdx + len(phrase)

		// Check left boundary (byte-based, ASCII-safe)
		leftOK := absIdx == 0 || !isWordByte(text[absIdx-1])

		// Check right boundary (byte-based, ASCII-safe)
		rightOK := endIdx == len(text) || !isWordByte(text[endIdx])

		if leftOK && rightOK {
			return true
		}

		// Move past this occurrence and continue searching
		searchStart = absIdx + 1
	}

	return false
}

// isWordByte returns true if the byte is an ASCII word character (a-z, A-Z, 0-9, _).
// This is intentionally byte-based rather than rune-based because:
// 1. All keywords are ASCII, so boundary checks only need ASCII awareness
// 2. Byte-based checks avoid UTF-8 multi-byte indexing issues
// 3. Non-ASCII bytes (>127) are not word characters, so they act as boundaries
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}
