package domain

import (
	"testing"
)

func TestParseStatusKeyword(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantStatus FindingStatus
		wantReason string
		wantFound  bool
	}{
		// Acknowledge keywords
		{
			name:       "acknowledged keyword",
			text:       "acknowledged - this is by design",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "acknowledged - this is by design",
			wantFound:  true,
		},
		{
			name:       "won't fix keyword",
			text:       "won't fix - out of scope",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "won't fix - out of scope",
			wantFound:  true,
		},
		{
			name:       "wont fix without apostrophe",
			text:       "wont fix",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "wont fix",
			wantFound:  true,
		},
		{
			name:       "intentional keyword",
			text:       "intentional behavior",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "intentional behavior",
			wantFound:  true,
		},
		{
			name:       "ack shorthand",
			text:       "ack",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "ack",
			wantFound:  true,
		},
		{
			name:       "ack with context",
			text:       "ack - will address in follow-up PR",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "ack - will address in follow-up PR",
			wantFound:  true,
		},

		// Dispute keywords
		{
			name:       "disputed keyword",
			text:       "disputed - the analysis is incorrect",
			wantStatus: FindingStatusDisputed,
			wantReason: "disputed - the analysis is incorrect",
			wantFound:  true,
		},
		{
			name:       "false positive keyword",
			text:       "false positive",
			wantStatus: FindingStatusDisputed,
			wantReason: "false positive",
			wantFound:  true,
		},
		{
			name:       "not a bug keyword",
			text:       "not a bug - expected behavior",
			wantStatus: FindingStatusDisputed,
			wantReason: "not a bug - expected behavior",
			wantFound:  true,
		},
		{
			name:       "not an issue keyword",
			text:       "not an issue",
			wantStatus: FindingStatusDisputed,
			wantReason: "not an issue",
			wantFound:  true,
		},

		// Case insensitivity
		{
			name:       "uppercase ACKNOWLEDGED",
			text:       "ACKNOWLEDGED",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "ACKNOWLEDGED",
			wantFound:  true,
		},
		{
			name:       "mixed case False Positive",
			text:       "False Positive",
			wantStatus: FindingStatusDisputed,
			wantReason: "False Positive",
			wantFound:  true,
		},

		// Keyword at start of message (common pattern)
		{
			name:       "keyword at start with explanation",
			text:       "Acknowledged: This is intentional for backward compatibility",
			wantStatus: FindingStatusAcknowledged,
			wantReason: "Acknowledged: This is intentional for backward compatibility",
			wantFound:  true,
		},

		// No keyword found
		{
			name:       "no keyword - regular comment",
			text:       "Thanks for the review!",
			wantStatus: "",
			wantReason: "",
			wantFound:  false,
		},
		{
			name:       "no keyword - question",
			text:       "Can you explain why this is an issue?",
			wantStatus: "",
			wantReason: "",
			wantFound:  false,
		},
		{
			name:       "empty text",
			text:       "",
			wantStatus: "",
			wantReason: "",
			wantFound:  false,
		},
		{
			name:       "partial match should not trigger - acknowledge vs acknowledged",
			text:       "I acknowledge your point but disagree",
			wantStatus: "",
			wantReason: "",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotReason, gotFound := ParseStatusKeyword(tt.text)

			if gotFound != tt.wantFound {
				t.Errorf("ParseStatusKeyword() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("ParseStatusKeyword() status = %v, want %v", gotStatus, tt.wantStatus)
			}
			if gotFound && gotReason != tt.wantReason {
				t.Errorf("ParseStatusKeyword() reason = %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}

func TestParseStatusKeyword_ReasonTruncation(t *testing.T) {
	// Test that very long reasons are truncated to MaxStatusReasonLength
	longText := "acknowledged - " + string(make([]byte, 600))

	status, reason, found := ParseStatusKeyword(longText)

	if !found {
		t.Fatal("expected keyword to be found")
	}
	if status != FindingStatusAcknowledged {
		t.Errorf("expected acknowledged status, got %v", status)
	}
	if len(reason) > MaxStatusReasonLength {
		t.Errorf("reason should be truncated to %d chars, got %d", MaxStatusReasonLength, len(reason))
	}
}

func TestStatusKeywordPatterns(t *testing.T) {
	// Verify the exported patterns are correct
	if len(AcknowledgeKeywords) == 0 {
		t.Error("AcknowledgeKeywords should not be empty")
	}
	if len(DisputeKeywords) == 0 {
		t.Error("DisputeKeywords should not be empty")
	}

	// Check specific keywords are present
	ackFound := false
	for _, k := range AcknowledgeKeywords {
		if k == "acknowledged" {
			ackFound = true
			break
		}
	}
	if !ackFound {
		t.Error("'acknowledged' should be in AcknowledgeKeywords")
	}

	disputeFound := false
	for _, k := range DisputeKeywords {
		if k == "false positive" {
			disputeFound = true
			break
		}
	}
	if !disputeFound {
		t.Error("'false positive' should be in DisputeKeywords")
	}
}
