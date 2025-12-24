package github_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

func TestDashboardRenderer_RenderDashboard_InProgress(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123def456",
		},
		ReviewStatus: domain.ReviewStatusInProgress,
		LastUpdated:  time.Now(),
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for dashboard marker
	if !strings.Contains(body, "<!-- CODE_REVIEWER_DASHBOARD_V1 -->") {
		t.Error("expected dashboard marker in body")
	}

	// Check for in-progress header
	if !strings.Contains(body, "## 🔄 Code Review In Progress") {
		t.Error("expected in-progress header")
	}

	// Check for short commit SHA
	if !strings.Contains(body, "`abc123d`") {
		t.Error("expected short commit SHA in body")
	}

	// Check for embedded metadata
	if !strings.Contains(body, "<!-- DASHBOARD_METADATA_B64") {
		t.Error("expected embedded metadata marker")
	}
}

func TestDashboardRenderer_RenderDashboard_Completed(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	now := time.Now()
	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123def456",
		},
		ReviewedCommits: []string{"abc123", "def456"},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding: domain.Finding{
					File:        "main.go",
					LineStart:   10,
					LineEnd:     15,
					Severity:    "critical",
					Category:    "security",
					Description: "SQL injection vulnerability",
				},
			},
			"fp2": {
				Fingerprint: "fp2",
				Status:      domain.FindingStatusResolved,
				Finding: domain.Finding{
					File:        "util.go",
					LineStart:   5,
					Severity:    "medium",
					Category:    "style",
					Description: "Missing error check",
				},
			},
		},
		LastUpdated:         now,
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true},
		Review: &domain.Review{
			ProviderName: "anthropic",
			ModelName:    "claude-3-sonnet",
			Cost:         0.0123,
		},
		Diff: &domain.Diff{
			Files: []domain.FileDiff{
				{Path: "main.go", Status: domain.FileStatusModified},
				{Path: "util.go", Status: domain.FileStatusModified},
			},
		},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for dashboard marker
	if !strings.Contains(body, "<!-- CODE_REVIEWER_DASHBOARD_V1 -->") {
		t.Error("expected dashboard marker in body")
	}

	// Check for status header (should show Changes Requested due to critical finding)
	if !strings.Contains(body, "## 🔴 Changes Requested") {
		t.Error("expected 'Changes Requested' header for blocking findings")
	}

	// Check for status table
	if !strings.Contains(body, "| 🔴 Open | 1 |") {
		t.Error("expected open count in status table")
	}
	if !strings.Contains(body, "| ✅ Resolved | 1 |") {
		t.Error("expected resolved count in status table")
	}

	// Check for severity badges
	if !strings.Contains(body, "📊 **Reviewed 2 files**") {
		t.Error("expected file count badge")
	}
	if !strings.Contains(body, "🔴 1 critical") {
		t.Error("expected critical count badge")
	}

	// Check for Files Requiring Attention
	if !strings.Contains(body, "### Files Requiring Attention") {
		t.Error("expected Files Requiring Attention section")
	}
	if !strings.Contains(body, "`main.go`") {
		t.Error("expected main.go in files requiring attention")
	}

	// Check for collapsible findings by severity (new format uses <strong>Critical</strong>)
	if !strings.Contains(body, "<details open>") {
		t.Error("expected critical findings section to be open by default")
	}
	if !strings.Contains(body, "<strong>Critical</strong>") {
		t.Error("expected Critical severity section")
	}

	// Check for Findings Requiring Attention header
	if !strings.Contains(body, "### Findings Requiring Attention") {
		t.Error("expected 'Findings Requiring Attention' section header")
	}

	// Check for review metadata
	if !strings.Contains(body, "📊 Review Metadata") {
		t.Error("expected Review Metadata section")
	}
	if !strings.Contains(body, "anthropic") {
		t.Error("expected provider name in metadata")
	}
	if !strings.Contains(body, "$0.0123") {
		t.Error("expected cost in metadata")
	}

	// Check for reviewed commits
	if !strings.Contains(body, "📋 Reviewed Commits") {
		t.Error("expected Reviewed Commits section")
	}

	// Check for embedded metadata
	if !strings.Contains(body, "<!-- DASHBOARD_METADATA_B64") {
		t.Error("expected embedded metadata marker")
	}
}

func TestDashboardRenderer_RenderDashboard_NoFindings(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		ReviewedCommits:     []string{"abc123"},
		Findings:            map[domain.FindingFingerprint]domain.TrackedFinding{},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true},
		Review: &domain.Review{
			ProviderName: "anthropic",
		},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for "No Issues Found" header
	if !strings.Contains(body, "## ✅ No Issues Found") {
		t.Error("expected 'No Issues Found' header for clean review")
	}
}

func TestDashboardRenderer_RenderDashboard_ApprovedWithSuggestions(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "low"}, // Non-blocking
			},
		},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true}, // low is not blocking
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for "Approved with Suggestions" header
	if !strings.Contains(body, "## ✅ Approved with Suggestions") {
		t.Error("expected 'Approved with Suggestions' header for non-blocking findings")
	}
}

func TestIsDashboardComment(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{
			name:     "dashboard comment",
			body:     "<!-- CODE_REVIEWER_DASHBOARD_V1 -->\n## Dashboard",
			expected: true,
		},
		{
			name:     "regular comment",
			body:     "This is a regular comment",
			expected: false,
		},
		{
			name:     "empty comment",
			body:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := github.IsDashboardComment(tt.body)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildReviewPointer(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "with URL",
			url:      "https://github.com/owner/repo/pull/1#issuecomment-123",
			expected: "See the [Code Review Dashboard](https://github.com/owner/repo/pull/1#issuecomment-123) for full details.",
		},
		{
			name:     "empty URL",
			url:      "",
			expected: "Code review complete. See the tracking comment for details.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := github.BuildReviewPointer(tt.url)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{1.50, "$1.50"},     // >= $1.00 → 2 decimals
		{0.50, "$0.500"},    // >= $0.10 → 3 decimals
		{0.05, "$0.0500"},   // < $0.10 → 4 decimals
		{0.0123, "$0.0123"}, // < $0.10 → 4 decimals
		{0.001, "$0.0010"},  // < $0.10 → 4 decimals
	}

	for _, tt := range tests {
		result := github.FormatCost(tt.cost)
		if result != tt.expected {
			t.Errorf("github.FormatCost(%v) = %q, expected %q", tt.cost, result, tt.expected)
		}
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		desc     string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long description", 20, "this is a very lo..."},
		{"exactly twenty chars", 20, "exactly twenty chars"},
	}

	for _, tt := range tests {
		result := github.TruncateDescription(tt.desc, tt.maxLen)
		if result != tt.expected {
			t.Errorf("github.TruncateDescription(%q, %d) = %q, expected %q", tt.desc, tt.maxLen, result, tt.expected)
		}
	}
}

func TestDashboardRenderer_RenderDashboard_IncludesInstructions(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "high", File: "main.go", LineStart: 10},
			},
		},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true},
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for instructions section
	if !strings.Contains(body, "How to Update Finding Status") {
		t.Error("expected 'How to Update Finding Status' section header")
	}

	// Check for collapsible section
	if !strings.Contains(body, "<details>") {
		t.Error("expected instructions to be in a collapsible section")
	}

	// Check for acknowledge keywords
	if !strings.Contains(body, "acknowledged") {
		t.Error("expected 'acknowledged' keyword in instructions")
	}
	if !strings.Contains(body, "won't fix") {
		t.Error("expected \"won't fix\" keyword in instructions")
	}

	// Check for dispute keywords
	if !strings.Contains(body, "disputed") {
		t.Error("expected 'disputed' keyword in instructions")
	}
	if !strings.Contains(body, "false positive") {
		t.Error("expected 'false positive' keyword in instructions")
	}

	// Check for auto-resolution explanation
	if !strings.Contains(body, "auto-resolved") {
		t.Error("expected auto-resolution explanation in instructions")
	}
}

func TestDashboardRenderer_RenderDashboard_NoInstructionsForInProgress(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	// Include findings to ensure instructions are suppressed due to in-progress
	// status, not just because there are no findings
	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "high", File: "main.go", LineStart: 10},
			},
		},
		ReviewStatus: domain.ReviewStatusInProgress,
		LastUpdated:  time.Now(),
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Instructions should NOT appear in in-progress reviews, even with findings
	if strings.Contains(body, "How to Update Finding Status") {
		t.Error("instructions should not appear in in-progress reviews")
	}
}

func TestDashboardRenderer_RenderDashboard_NoInstructionsForNoFindings(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		ReviewedCommits:     []string{"abc123"},
		Findings:            map[domain.FindingFingerprint]domain.TrackedFinding{},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true},
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Instructions should NOT appear when there are no findings
	if strings.Contains(body, "How to Update Finding Status") {
		t.Error("instructions should not appear when there are no findings")
	}
}

func TestDashboardRenderer_RenderDashboard_ResolvedFindings(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	resolvedCommit := "abc123def"
	resolvedTime := time.Now()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "xyz789",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusResolved,
				ResolvedIn:  &resolvedCommit,
				ResolvedAt:  &resolvedTime,
				Finding: domain.Finding{
					File:        "main.go",
					LineStart:   10,
					Severity:    "high",
					Category:    "security",
					Description: "Fixed SQL injection vulnerability",
				},
			},
			"fp2": {
				Fingerprint: "fp2",
				Status:      domain.FindingStatusOpen,
				Finding: domain.Finding{
					File:        "api.go",
					LineStart:   20,
					Severity:    "low",
					Category:    "style",
					Description: "Open style issue",
				},
			},
		},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true},
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for resolved findings section
	if !strings.Contains(body, "Resolved Findings") {
		t.Error("expected 'Resolved Findings' section")
	}

	// Check for specific strikethrough table cell with file name
	if !strings.Contains(body, "~~`main.go`~~") {
		t.Error("expected strikethrough with file name in resolved findings")
	}

	// Check for strikethrough severity
	if !strings.Contains(body, "~~high~~") {
		t.Error("expected strikethrough severity in resolved findings")
	}

	// Check for short commit SHA in resolved info
	if !strings.Contains(body, "abc123d") {
		t.Error("expected short commit SHA in resolved findings")
	}

	// Resolved section should be collapsed by default (not "open")
	if strings.Contains(body, `<details open>
<summary>📋 <strong>Resolved`) {
		t.Error("resolved section should be collapsed, not open")
	}
}

func TestDashboardRenderer_RenderDashboard_ExpandableIndividualFindings(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding: domain.Finding{
					File:        "main.go",
					LineStart:   10,
					LineEnd:     15,
					Severity:    "critical",
					Category:    "security",
					Description: "SQL injection vulnerability detected in user input handler",
					Suggestion:  "Use parameterized queries instead of string concatenation",
				},
			},
		},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true},
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for expandable individual finding (nested details block)
	// After the summary table, there should be individual finding details
	if !strings.Contains(body, "<code>main.go:10-15</code>") {
		t.Error("expected individual finding with file and line range")
	}

	// Check for suggestion in individual finding
	if !strings.Contains(body, "**Suggestion:**") {
		t.Error("expected suggestion in individual finding")
	}
	if !strings.Contains(body, "parameterized queries") {
		t.Error("expected suggestion text in individual finding")
	}

	// Check for category in individual finding
	if !strings.Contains(body, "**Category:** security") {
		t.Error("expected category in individual finding")
	}
}

func TestDashboardRenderer_RenderDashboard_EmptySuggestion(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding: domain.Finding{
					File:        "main.go",
					LineStart:   10,
					Severity:    "high",
					Category:    "bug",
					Description: "Potential null pointer dereference",
					Suggestion:  "", // Empty suggestion
				},
			},
		},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"high": true},
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not contain suggestion section when suggestion is empty
	if strings.Contains(body, "**Suggestion:**") {
		t.Error("should not show suggestion section when suggestion is empty")
	}

	// Should still contain the finding details
	if !strings.Contains(body, "main.go") {
		t.Error("expected file name in finding")
	}
	if !strings.Contains(body, "null pointer") {
		t.Error("expected description in finding")
	}
}

func TestDashboardRenderer_RenderDashboard_SectionSeparator(t *testing.T) {
	renderer := github.NewDashboardRenderer()

	data := review.DashboardData{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "low", File: "test.go", LineStart: 1},
			},
		},
		LastUpdated:         time.Now(),
		ReviewStatus:        domain.ReviewStatusCompleted,
		AttentionSeverities: map[string]bool{"critical": true, "high": true},
		Review:              &domain.Review{ProviderName: "anthropic"},
	}

	body, err := renderer.RenderDashboard(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for horizontal rule separator between findings and metadata
	if !strings.Contains(body, "---\n") {
		t.Error("expected horizontal rule separator between findings and metadata sections")
	}
}

func TestTruncateDescription_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		maxLen   int
		expected string
	}{
		{
			name:     "normal truncation",
			desc:     "This is a long description that needs truncation",
			maxLen:   20,
			expected: "This is a long de...",
		},
		{
			name:     "no truncation needed",
			desc:     "Short",
			maxLen:   20,
			expected: "Short",
		},
		{
			name:     "maxLen zero",
			desc:     "Some text",
			maxLen:   0,
			expected: "",
		},
		{
			name:     "maxLen negative",
			desc:     "Some text",
			maxLen:   -5,
			expected: "",
		},
		{
			name:     "maxLen 1",
			desc:     "ABCDEF",
			maxLen:   1,
			expected: "A",
		},
		{
			name:     "maxLen 2",
			desc:     "ABCDEF",
			maxLen:   2,
			expected: "AB",
		},
		{
			name:     "maxLen 3",
			desc:     "ABCDEF",
			maxLen:   3,
			expected: "ABC",
		},
		{
			name:     "maxLen 4 with truncation",
			desc:     "ABCDEF",
			maxLen:   4,
			expected: "A...",
		},
		{
			name:     "UTF-8 characters",
			desc:     "こんにちは世界", // Japanese: "Hello World"
			maxLen:   5,
			expected: "こん...",
		},
		{
			name:     "UTF-8 no truncation",
			desc:     "日本語",
			maxLen:   10,
			expected: "日本語",
		},
		{
			name:     "UTF-8 maxLen 1",
			desc:     "日本語",
			maxLen:   1,
			expected: "日",
		},
		{
			name:     "UTF-8 maxLen 2",
			desc:     "日本語",
			maxLen:   2,
			expected: "日本",
		},
		{
			name:     "UTF-8 maxLen 3",
			desc:     "日本語",
			maxLen:   3,
			expected: "日本語", // No truncation needed (exactly 3 runes)
		},
		{
			name:     "UTF-8 maxLen 4 with truncation",
			desc:     "日本語テスト",
			maxLen:   4,
			expected: "日...",
		},
		{
			name:     "invalid UTF-8 normalized",
			desc:     string([]byte{0xff, 0xfe, 0x41, 0x42}), // Invalid UTF-8 + "AB"
			maxLen:   10,
			expected: "\ufffd\ufffdAB", // Replacement chars + "AB"
		},
		{
			name:     "empty string",
			desc:     "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := github.TruncateDescription(tt.desc, tt.maxLen)
			if result != tt.expected {
				t.Errorf("TruncateDescription(%q, %d) = %q, want %q",
					tt.desc, tt.maxLen, result, tt.expected)
			}
		})
	}
}
