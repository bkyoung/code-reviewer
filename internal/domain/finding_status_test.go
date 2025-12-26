package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectStatusFromText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected FindingStatus
	}{
		// Acknowledgment keywords
		{"acknowledged explicit", "Acknowledged, I'll look into this", StatusAcknowledged},
		{"ack shorthand", "ack, will fix", StatusAcknowledged},
		{"won't fix", "This is a won't fix for now", StatusAcknowledged},
		{"wont fix no apostrophe", "wont fix, too risky to change", StatusAcknowledged},
		{"wontfix single word", "Marking as wontfix", StatusAcknowledged},
		{"intentional", "This is intentional behavior", StatusAcknowledged},
		{"by design", "This is by design", StatusAcknowledged},
		{"as designed", "Working as designed", StatusAcknowledged},
		{"working as intended", "This is working as intended", StatusAcknowledged},
		{"works as designed", "It works as designed", StatusAcknowledged},
		{"good catch", "Good catch! Will fix", StatusAcknowledged},
		{"thanks", "Thanks for pointing this out", StatusAcknowledged},
		{"agreed", "Agreed, this should be fixed", StatusAcknowledged},
		{"valid", "Valid point", StatusAcknowledged},
		{"noted", "Noted, adding to backlog", StatusAcknowledged},
		{"known issue", "This is a known issue", StatusAcknowledged},
		{"will fix later", "Will fix later in a separate PR", StatusAcknowledged},

		// Dispute keywords
		{"false positive", "This is a false positive", StatusDisputed},
		{"not an issue", "Not an issue in our case", StatusDisputed},
		{"not a bug", "Not a bug, this is expected", StatusDisputed},
		{"disagree", "I disagree with this finding", StatusDisputed},
		{"disputed", "This finding is disputed", StatusDisputed},
		{"incorrect", "This analysis is incorrect", StatusDisputed},
		{"expected behavior", "This is expected behavior", StatusDisputed},
		{"expected behaviour", "This is expected behaviour", StatusDisputed},
		{"not applicable", "Not applicable to our use case", StatusDisputed},
		{"n/a", "N/A for this codebase", StatusDisputed},

		// Open (no keywords)
		{"question", "Can you explain this more?", StatusOpen},
		{"empty", "", StatusOpen},
		{"unrelated", "I'm not sure what to do here", StatusOpen},
		{"just code", "Let me check the implementation", StatusOpen},

		// Case insensitivity
		{"ACKNOWLEDGED uppercase", "ACKNOWLEDGED", StatusAcknowledged},
		{"False Positive mixed case", "False Positive detected", StatusDisputed},
		{"AcKnOwLeDgEd weird case", "AcKnOwLeDgEd", StatusAcknowledged},

		// Word boundary tests - should NOT match
		{"acknowledged in word", "unacknowledged issue", StatusOpen},
		{"valid in word", "invalidated the test", StatusOpen},
		{"wrong in word", "wrongly formatted", StatusOpen},
		{"fix in prefix", "fixing the code", StatusOpen},

		// Word boundary tests - should match
		{"ack with punctuation", "ack! will do", StatusAcknowledged},
		{"thanks with comma", "thanks, updating now", StatusAcknowledged},
		{"disputed at end", "This is disputed", StatusDisputed},
		{"valid at start", "valid concern here", StatusAcknowledged},

		// Multi-word phrases
		{"good catch in sentence", "That's a good catch, thanks!", StatusAcknowledged},
		{"false positive in sentence", "I believe this is a false positive because...", StatusDisputed},
		{"by design explanation", "This behavior is by design - see docs", StatusAcknowledged},

		// Dispute takes precedence over acknowledgment
		{"both dispute and ack", "Thanks, but this is a false positive", StatusDisputed},
		{"ack then dispute", "Good catch, but I disagree with the severity", StatusDisputed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectStatusFromText(tt.text)
			assert.Equal(t, tt.expected, result, "text: %q", tt.text)
		})
	}
}

func TestDetectStatusFromReplies(t *testing.T) {
	tests := []struct {
		name     string
		replies  []string
		expected FindingStatus
	}{
		// Empty replies
		{"no replies", []string{}, StatusOpen},
		{"empty reply", []string{""}, StatusOpen},

		// Single reply
		{"single ack", []string{"acknowledged"}, StatusAcknowledged},
		{"single dispute", []string{"false positive"}, StatusDisputed},
		{"single open", []string{"what does this mean?"}, StatusOpen},

		// Multiple replies - latest doesn't matter, presence does
		{"ack then question", []string{"acknowledged", "actually, what does this mean?"}, StatusAcknowledged},
		{"question then ack", []string{"what?", "oh, acknowledged"}, StatusAcknowledged},

		// Dispute takes precedence
		{"ack then dispute", []string{"acknowledged", "wait, this is a false positive"}, StatusDisputed},
		{"dispute then ack", []string{"false positive", "actually, acknowledged"}, StatusDisputed},

		// Multiple open replies stay open
		{"all questions", []string{"what?", "can you explain?", "not sure"}, StatusOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectStatusFromReplies(tt.replies)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsPhrase(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		phrase   string
		expected bool
	}{
		// Basic matches
		{"exact match", "acknowledged", "acknowledged", true},
		{"phrase in sentence", "this is acknowledged by me", "acknowledged", true},
		{"at start", "acknowledged this issue", "acknowledged", true},
		{"at end", "issue acknowledged", "acknowledged", true},

		// Word boundaries
		{"not in middle of word", "unacknowledged", "acknowledged", false},
		{"not partial prefix", "acknowledge", "acknowledged", false},
		{"with punctuation after", "acknowledged!", "acknowledged", true},
		{"with punctuation before", "!acknowledged", "acknowledged", true},
		{"with space after", "acknowledged ", "acknowledged", true},
		{"with newline after", "acknowledged\n", "acknowledged", true},

		// Multi-word phrases
		{"multi word match", "this is a false positive", "false positive", true},
		{"multi word no match", "falsepositive", "false positive", false},
		{"phrase with punctuation", "it's a false positive!", "false positive", true},

		// Case sensitivity (containsPhrase expects lowercase)
		{"lowercase only", "acknowledged", "acknowledged", true},

		// Edge cases
		{"empty text", "", "acknowledged", false},
		{"empty phrase", "some text", "", true}, // strings.Index returns 0 for empty
		{"phrase longer than text", "ack", "acknowledged", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsPhrase(tt.text, tt.phrase)
			assert.Equal(t, tt.expected, result, "text=%q phrase=%q", tt.text, tt.phrase)
		})
	}
}
