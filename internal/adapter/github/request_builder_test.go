package github_test

import (
	"testing"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/diff"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildReviewComments_OnlyIncludesInDiffFindings(t *testing.T) {
	findings := []github.PositionedFinding{
		{
			Finding:      makeFinding("file1.go", 10, "high", "Issue 1"),
			DiffPosition: diff.IntPtr(5), // In diff
		},
		{
			Finding:      makeFinding("file2.go", 20, "medium", "Issue 2"),
			DiffPosition: nil, // NOT in diff - should be excluded
		},
		{
			Finding:      makeFinding("file3.go", 30, "low", "Issue 3"),
			DiffPosition: diff.IntPtr(15), // In diff
		},
	}

	comments := github.BuildReviewComments(findings)

	require.Len(t, comments, 2)
	assert.Equal(t, "file1.go", comments[0].Path)
	assert.Equal(t, 5, comments[0].Position)
	assert.Equal(t, "file3.go", comments[1].Path)
	assert.Equal(t, 15, comments[1].Position)
}

func TestBuildReviewComments_Empty(t *testing.T) {
	comments := github.BuildReviewComments([]github.PositionedFinding{})
	assert.Empty(t, comments)
}

func TestBuildReviewComments_AllOutOfDiff(t *testing.T) {
	findings := []github.PositionedFinding{
		{
			Finding:      makeFinding("file1.go", 10, "high", "Issue 1"),
			DiffPosition: nil,
		},
		{
			Finding:      makeFinding("file2.go", 20, "medium", "Issue 2"),
			DiffPosition: nil,
		},
	}

	comments := github.BuildReviewComments(findings)
	assert.Empty(t, comments)
}

func TestFormatFindingComment(t *testing.T) {
	finding := makeFinding("main.go", 42, "high", "SQL injection vulnerability")
	finding.Category = "security"
	finding.Suggestion = "Use parameterized queries instead"

	comment := github.FormatFindingComment(finding)

	assert.Contains(t, comment, "**Severity:** high")
	assert.Contains(t, comment, "**Category:** security")
	assert.Contains(t, comment, "SQL injection vulnerability")
	assert.Contains(t, comment, "Use parameterized queries instead")
}

func TestFormatFindingComment_NoSuggestion(t *testing.T) {
	finding := makeFinding("main.go", 42, "low", "Minor style issue")
	finding.Suggestion = ""

	comment := github.FormatFindingComment(finding)

	assert.Contains(t, comment, "Minor style issue")
	assert.NotContains(t, comment, "**Suggestion:**")
}

func TestFormatFindingComment_LineRange(t *testing.T) {
	finding := domain.Finding{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     15, // Multi-line finding
		Severity:    "medium",
		Description: "Long function",
	}

	comment := github.FormatFindingComment(finding)

	assert.Contains(t, comment, "Lines 10-15")
}

func TestFormatFindingComment_SingleLine(t *testing.T) {
	finding := domain.Finding{
		File:        "main.go",
		LineStart:   42,
		LineEnd:     42, // Same line
		Severity:    "medium",
		Description: "Issue on line 42",
	}

	comment := github.FormatFindingComment(finding)

	assert.Contains(t, comment, "Line 42")
	assert.NotContains(t, comment, "Lines")
}

func TestDetermineReviewEvent_RequestChangesOnHighSeverity(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "low", "minor"), DiffPosition: diff.IntPtr(1)},
		{Finding: makeFinding("b.go", 2, "high", "critical bug"), DiffPosition: diff.IntPtr(2)},
	}

	event := github.DetermineReviewEvent(findings)

	assert.Equal(t, github.EventRequestChanges, event)
}

func TestDetermineReviewEvent_RequestChangesOnCriticalSeverity(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "critical", "security issue"), DiffPosition: diff.IntPtr(1)},
	}

	event := github.DetermineReviewEvent(findings)

	assert.Equal(t, github.EventRequestChanges, event)
}

func TestDetermineReviewEvent_CommentOnMediumSeverity(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "medium", "code smell"), DiffPosition: diff.IntPtr(1)},
		{Finding: makeFinding("b.go", 2, "low", "minor issue"), DiffPosition: diff.IntPtr(2)},
	}

	event := github.DetermineReviewEvent(findings)

	assert.Equal(t, github.EventComment, event)
}

func TestDetermineReviewEvent_ApproveOnNoFindings(t *testing.T) {
	event := github.DetermineReviewEvent([]github.PositionedFinding{})

	assert.Equal(t, github.EventApprove, event)
}

func TestDetermineReviewEvent_IgnoresOutOfDiffFindings(t *testing.T) {
	// High severity finding but NOT in diff - should not trigger REQUEST_CHANGES
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "critical"), DiffPosition: nil},
		{Finding: makeFinding("b.go", 2, "low", "minor"), DiffPosition: diff.IntPtr(1)},
	}

	event := github.DetermineReviewEvent(findings)

	// Only in-diff findings count, so only low severity → COMMENT
	assert.Equal(t, github.EventComment, event)
}

func TestCountInDiffFindings(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "a"), DiffPosition: diff.IntPtr(1)},
		{Finding: makeFinding("b.go", 2, "low", "b"), DiffPosition: nil},
		{Finding: makeFinding("c.go", 3, "low", "c"), DiffPosition: diff.IntPtr(3)},
	}

	count := github.CountInDiffFindings(findings)

	assert.Equal(t, 2, count)
}

// Helper to create a finding for tests
func makeFinding(file string, line int, severity, description string) domain.Finding {
	return domain.Finding{
		ID:          "test-id",
		File:        file,
		LineStart:   line,
		LineEnd:     line,
		Severity:    severity,
		Category:    "test",
		Description: description,
	}
}
