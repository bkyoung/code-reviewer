package github

import (
	"strings"
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

func TestDashboardRenderer_RenderDashboard_InProgress(t *testing.T) {
	renderer := NewDashboardRenderer()

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
	renderer := NewDashboardRenderer()

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

	// Check for collapsible findings by severity
	if !strings.Contains(body, "<details open>") {
		t.Error("expected critical findings section to be open by default")
	}
	if !strings.Contains(body, "Critical Issues") {
		t.Error("expected Critical Issues section")
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
	renderer := NewDashboardRenderer()

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
	renderer := NewDashboardRenderer()

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
			result := IsDashboardComment(tt.body)
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
			result := BuildReviewPointer(tt.url)
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
		result := formatCost(tt.cost)
		if result != tt.expected {
			t.Errorf("formatCost(%v) = %q, expected %q", tt.cost, result, tt.expected)
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
		result := truncateDescription(tt.desc, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateDescription(%q, %d) = %q, expected %q", tt.desc, tt.maxLen, result, tt.expected)
		}
	}
}
