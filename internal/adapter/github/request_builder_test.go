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

func TestNormalizeAction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected github.ReviewEvent
		valid    bool
	}{
		// Valid APPROVE variations
		{"uppercase APPROVE", "APPROVE", github.EventApprove, true},
		{"lowercase approve", "approve", github.EventApprove, true},
		{"mixed case Approve", "Approve", github.EventApprove, true},
		{"with whitespace", "  approve  ", github.EventApprove, true},

		// Valid REQUEST_CHANGES variations
		{"uppercase REQUEST_CHANGES", "REQUEST_CHANGES", github.EventRequestChanges, true},
		{"lowercase request_changes", "request_changes", github.EventRequestChanges, true},
		{"hyphenated request-changes", "request-changes", github.EventRequestChanges, true},
		{"mixed case Request_Changes", "Request_Changes", github.EventRequestChanges, true},

		// Valid COMMENT variations
		{"uppercase COMMENT", "COMMENT", github.EventComment, true},
		{"lowercase comment", "comment", github.EventComment, true},
		{"mixed case Comment", "Comment", github.EventComment, true},

		// Invalid values - should return EventComment and false
		{"empty string", "", github.EventComment, false},
		{"invalid value", "invalid", github.EventComment, false},
		{"typo", "approv", github.EventComment, false},
		{"only whitespace", "   ", github.EventComment, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, valid := github.NormalizeAction(tt.input)
			assert.Equal(t, tt.expected, event, "event mismatch")
			assert.Equal(t, tt.valid, valid, "valid mismatch")
		})
	}
}

func TestDetermineReviewEventWithActions(t *testing.T) {
	// Custom actions for testing
	customActions := github.ReviewActions{
		OnCritical: "comment", // Instead of request_changes
		OnHigh:     "approve", // Instead of request_changes
		OnMedium:   "approve", // Instead of comment
		OnLow:      "approve", // Instead of comment
		OnClean:    "comment", // Instead of approve
	}

	tests := []struct {
		name     string
		findings []github.PositionedFinding
		actions  github.ReviewActions
		expected github.ReviewEvent
	}{
		{
			name:     "clean code with custom action",
			findings: []github.PositionedFinding{},
			actions:  customActions,
			expected: github.EventComment, // customActions.OnClean = comment
		},
		{
			name: "critical finding with custom action",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "critical", "security issue"), DiffPosition: diff.IntPtr(1)},
			},
			actions:  customActions,
			expected: github.EventComment, // customActions.OnCritical = comment
		},
		{
			name: "high finding with custom action",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "high", "bug"), DiffPosition: diff.IntPtr(1)},
			},
			actions:  customActions,
			expected: github.EventApprove, // customActions.OnHigh = approve
		},
		{
			name: "medium finding with custom action",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "medium", "code smell"), DiffPosition: diff.IntPtr(1)},
			},
			actions:  customActions,
			expected: github.EventApprove, // customActions.OnMedium = approve
		},
		{
			name: "low finding with custom action",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "low", "minor issue"), DiffPosition: diff.IntPtr(1)},
			},
			actions:  customActions,
			expected: github.EventApprove, // customActions.OnLow = approve
		},
		{
			name: "highest severity wins (critical over high)",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "high", "bug"), DiffPosition: diff.IntPtr(1)},
				{Finding: makeFinding("b.go", 2, "critical", "security issue"), DiffPosition: diff.IntPtr(2)},
			},
			actions:  customActions,
			expected: github.EventComment, // critical wins, customActions.OnCritical = comment
		},
		{
			name: "case insensitive action values",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "critical", "issue"), DiffPosition: diff.IntPtr(1)},
			},
			actions: github.ReviewActions{
				OnCritical: "REQUEST_CHANGES", // uppercase
			},
			expected: github.EventRequestChanges,
		},
		{
			name: "out of diff findings ignored",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "critical", "not in diff"), DiffPosition: nil},
				{Finding: makeFinding("b.go", 2, "low", "in diff"), DiffPosition: diff.IntPtr(1)},
			},
			actions:  customActions,
			expected: github.EventApprove, // only low in diff, customActions.OnLow = approve
		},
		{
			name: "default actions when config is empty",
			findings: []github.PositionedFinding{
				{Finding: makeFinding("a.go", 1, "high", "bug"), DiffPosition: diff.IntPtr(1)},
			},
			actions:  github.ReviewActions{},     // empty config
			expected: github.EventRequestChanges, // fallback to default
		},
		{
			name:     "clean code with empty config uses approve fallback",
			findings: []github.PositionedFinding{},
			actions:  github.ReviewActions{}, // empty config
			expected: github.EventApprove,    // fallback to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := github.DetermineReviewEventWithActions(tt.findings, tt.actions)
			assert.Equal(t, tt.expected, event)
		})
	}
}
