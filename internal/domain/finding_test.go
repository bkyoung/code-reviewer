package domain_test

import (
	"testing"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

func TestFindingDeterministicID(t *testing.T) {
	finding := domain.NewFinding(domain.FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     12,
		Severity:    "high",
		Category:    "bug",
		Description: "Example bug",
		Suggestion:  "Fix bug",
		Evidence:    true,
	})

	again := domain.NewFinding(domain.FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     12,
		Severity:    "high",
		Category:    "bug",
		Description: "Example bug",
		Suggestion:  "Fix bug",
		Evidence:    true,
	})

	if finding.ID != again.ID {
		t.Fatalf("expected deterministic IDs, got %s and %s", finding.ID, again.ID)
	}
}
