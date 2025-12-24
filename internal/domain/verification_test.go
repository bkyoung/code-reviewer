package domain

import (
	"testing"
)

func TestClassification_IsValid(t *testing.T) {
	tests := []struct {
		classification Classification
		want           bool
	}{
		{ClassBlockingBug, true},
		{ClassSecurity, true},
		{ClassPerformance, true},
		{ClassStyle, true},
		{Classification("invalid"), false},
		{Classification(""), false},
		{Classification("BLOCKING_BUG"), false}, // Case-sensitive
		{Classification("blocking-bug"), false}, // Wrong format
	}

	for _, tt := range tests {
		t.Run(string(tt.classification), func(t *testing.T) {
			if got := tt.classification.IsValid(); got != tt.want {
				t.Errorf("Classification(%q).IsValid() = %v, want %v", tt.classification, got, tt.want)
			}
		})
	}
}

func TestCandidateFinding_Struct(t *testing.T) {
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "SQL injection risk",
		Suggestion:  "Use parameterized queries",
		Evidence:    true,
	})

	candidate := CandidateFinding{
		Finding:        finding,
		Sources:        []string{"openai", "anthropic"},
		AgreementScore: 0.85,
	}

	if candidate.Finding.ID != finding.ID {
		t.Errorf("CandidateFinding.Finding.ID = %s, want %s", candidate.Finding.ID, finding.ID)
	}

	if len(candidate.Sources) != 2 {
		t.Errorf("len(CandidateFinding.Sources) = %d, want 2", len(candidate.Sources))
	}

	if candidate.AgreementScore != 0.85 {
		t.Errorf("CandidateFinding.AgreementScore = %f, want 0.85", candidate.AgreementScore)
	}
}

func TestVerificationAction_Struct(t *testing.T) {
	action := VerificationAction{
		Tool:   "read",
		Input:  "main.go",
		Output: "package main\n\nimport \"strings\"",
	}

	if action.Tool != "read" {
		t.Errorf("VerificationAction.Tool = %s, want read", action.Tool)
	}

	if action.Input != "main.go" {
		t.Errorf("VerificationAction.Input = %s, want main.go", action.Input)
	}
}

func TestVerifiedFinding_Struct(t *testing.T) {
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "SQL injection risk",
		Suggestion:  "Use parameterized queries",
		Evidence:    true,
	})

	verified := VerifiedFinding{
		Finding:         finding,
		Verified:        true,
		Classification:  ClassSecurity,
		Confidence:      92,
		Evidence:        "Confirmed via static analysis: user input concatenated directly",
		BlocksOperation: true,
		VerificationLog: []VerificationAction{
			{Tool: "read", Input: "db.go", Output: "query := \"SELECT * FROM users WHERE id = \" + userID"},
			{Tool: "grep", Input: "sql.Query", Output: "db.go:45"},
		},
	}

	if !verified.Verified {
		t.Error("VerifiedFinding.Verified should be true")
	}

	if verified.Classification != ClassSecurity {
		t.Errorf("VerifiedFinding.Classification = %s, want %s", verified.Classification, ClassSecurity)
	}

	if verified.Confidence != 92 {
		t.Errorf("VerifiedFinding.Confidence = %d, want 92", verified.Confidence)
	}

	if !verified.BlocksOperation {
		t.Error("VerifiedFinding.BlocksOperation should be true")
	}

	if len(verified.VerificationLog) != 2 {
		t.Errorf("len(VerifiedFinding.VerificationLog) = %d, want 2", len(verified.VerificationLog))
	}
}

func TestVerificationResult_Struct(t *testing.T) {
	result := VerificationResult{
		Verified:        true,
		Classification:  ClassSecurity,
		Confidence:      92,
		Evidence:        "Confirmed via static analysis",
		BlocksOperation: true,
		Actions: []VerificationAction{
			{Tool: "read", Input: "db.go", Output: "..."},
		},
	}

	if !result.Verified {
		t.Error("VerificationResult.Verified should be true")
	}

	if result.Classification != ClassSecurity {
		t.Errorf("VerificationResult.Classification = %s, want %s", result.Classification, ClassSecurity)
	}

	if len(result.Actions) != 1 {
		t.Errorf("len(VerificationResult.Actions) = %d, want 1", len(result.Actions))
	}
}

func TestVerifiedFinding_EmptyVerificationLog(t *testing.T) {
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "low",
		Category:    "style",
		Description: "Variable naming",
		Suggestion:  "",
		Evidence:    false,
	})

	// Style findings are discarded during verification - verify empty log is valid
	verified := VerifiedFinding{
		Finding:         finding,
		Verified:        false,
		Classification:  ClassStyle,
		Confidence:      0,
		Evidence:        "Style finding - discarded",
		BlocksOperation: false,
		VerificationLog: nil, // Empty is valid
	}

	if verified.Verified {
		t.Error("VerifiedFinding.Verified should be false for discarded style finding")
	}

	if verified.VerificationLog != nil {
		t.Error("VerifiedFinding.VerificationLog should be nil when empty")
	}
}

func TestClassification_AllValidValues(t *testing.T) {
	validClassifications := []Classification{
		ClassBlockingBug,
		ClassSecurity,
		ClassPerformance,
		ClassStyle,
	}

	for _, c := range validClassifications {
		if !c.IsValid() {
			t.Errorf("Classification %q should be valid", c)
		}
	}
}
