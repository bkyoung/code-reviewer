package tracking

import (
	"strings"
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

func TestIsTrackingComment(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "valid tracking comment",
			body: "<!-- CODE_REVIEWER_TRACKING_V1 -->\n\n## Tracking\n...",
			want: true,
		},
		{
			name: "valid dashboard comment",
			body: "<!-- CODE_REVIEWER_DASHBOARD_V1 -->\n\n## Dashboard\n...",
			want: true,
		},
		{
			name: "tracking marker in middle",
			body: "Some text\n<!-- CODE_REVIEWER_TRACKING_V1 -->\nMore text",
			want: true,
		},
		{
			name: "dashboard marker in middle",
			body: "Some text\n<!-- CODE_REVIEWER_DASHBOARD_V1 -->\nMore text",
			want: true,
		},
		{
			name: "no marker",
			body: "Just a regular comment",
			want: false,
		},
		{
			name: "partial tracking marker",
			body: "<!-- CODE_REVIEWER_TRACKING",
			want: false,
		},
		{
			name: "partial dashboard marker",
			body: "<!-- CODE_REVIEWER_DASHBOARD",
			want: false,
		},
		{
			name: "empty body",
			body: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTrackingComment(tt.body); got != tt.want {
				t.Errorf("IsTrackingComment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderAndParseTrackingComment_RoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for JSON precision

	finding := domain.NewFinding(domain.FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     15,
		Severity:    "high",
		Category:    "security",
		Description: "SQL injection vulnerability",
		Suggestion:  "Use parameterized queries",
		Evidence:    true,
	})

	trackedFinding, err := domain.NewTrackedFindingFromFinding(finding, now, "abc123")
	if err != nil {
		t.Fatalf("failed to create tracked finding: %v", err)
	}

	originalState := review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   123,
			Branch:     "feature-branch",
			BaseSHA:    "abc123",
			HeadSHA:    "def456",
		},
		ReviewedCommits: []string{"abc123", "def456"},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
		LastUpdated: now,
	}

	// Render to comment body
	body, err := RenderTrackingComment(originalState)
	if err != nil {
		t.Fatalf("RenderTrackingComment() error = %v", err)
	}

	// Verify the comment is identifiable
	if !IsTrackingComment(body) {
		t.Error("rendered comment should be identifiable as tracking comment")
	}

	// Verify it contains expected elements
	if !strings.Contains(body, "## 🤖 Code Review Completed") {
		t.Error("comment should contain completed header")
	}
	if !strings.Contains(body, "| 🔴 Open | 1 |") {
		t.Error("comment should show 1 open finding")
	}
	if !strings.Contains(body, "abc123") {
		t.Error("comment should contain short SHA")
	}

	// Parse it back
	parsedState, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment() error = %v", err)
	}

	// Verify round-trip
	if parsedState.Target.Repository != originalState.Target.Repository {
		t.Errorf("Repository = %s, want %s", parsedState.Target.Repository, originalState.Target.Repository)
	}
	if parsedState.Target.PRNumber != originalState.Target.PRNumber {
		t.Errorf("PRNumber = %d, want %d", parsedState.Target.PRNumber, originalState.Target.PRNumber)
	}
	if len(parsedState.ReviewedCommits) != len(originalState.ReviewedCommits) {
		t.Errorf("ReviewedCommits len = %d, want %d", len(parsedState.ReviewedCommits), len(originalState.ReviewedCommits))
	}
	if len(parsedState.Findings) != len(originalState.Findings) {
		t.Errorf("Findings len = %d, want %d", len(parsedState.Findings), len(originalState.Findings))
	}

	// Verify finding was preserved
	for fp, originalFinding := range originalState.Findings {
		parsedFinding, exists := parsedState.Findings[fp]
		if !exists {
			t.Errorf("finding with fingerprint %s not found after round-trip", fp)
			continue
		}
		if parsedFinding.Status != originalFinding.Status {
			t.Errorf("finding status = %s, want %s", parsedFinding.Status, originalFinding.Status)
		}
		if parsedFinding.Finding.File != originalFinding.Finding.File {
			t.Errorf("finding file = %s, want %s", parsedFinding.Finding.File, originalFinding.Finding.File)
		}
	}
}

func TestRenderTrackingComment_EmptyState(t *testing.T) {
	state := review.NewTrackingState(review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   1,
		HeadSHA:    "abc123",
	})

	body, err := RenderTrackingComment(state)
	if err != nil {
		t.Fatalf("RenderTrackingComment() error = %v", err)
	}

	if !IsTrackingComment(body) {
		t.Error("rendered comment should be identifiable")
	}

	// Should show all zeros
	if !strings.Contains(body, "| 🔴 Open | 0 |") {
		t.Error("should show 0 open findings")
	}
}

func TestRenderTrackingComment_MultipleStatuses(t *testing.T) {
	now := time.Now()

	// Create findings with different statuses
	openFinding := createTestTrackedFinding(t, "open.go", domain.FindingStatusOpen, now)
	resolvedFinding := createTestTrackedFinding(t, "resolved.go", domain.FindingStatusResolved, now)
	acknowledgedFinding := createTestTrackedFinding(t, "ack.go", domain.FindingStatusAcknowledged, now)
	disputedFinding := createTestTrackedFinding(t, "disputed.go", domain.FindingStatusDisputed, now)

	state := review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			openFinding.Fingerprint:         openFinding,
			resolvedFinding.Fingerprint:     resolvedFinding,
			acknowledgedFinding.Fingerprint: acknowledgedFinding,
			disputedFinding.Fingerprint:     disputedFinding,
		},
	}

	body, err := RenderTrackingComment(state)
	if err != nil {
		t.Fatalf("RenderTrackingComment() error = %v", err)
	}

	if !strings.Contains(body, "| 🔴 Open | 1 |") {
		t.Error("should show 1 open finding")
	}
	if !strings.Contains(body, "| ✅ Resolved | 1 |") {
		t.Error("should show 1 resolved finding")
	}
	if !strings.Contains(body, "| 💬 Acknowledged | 1 |") {
		t.Error("should show 1 acknowledged finding")
	}
	if !strings.Contains(body, "| ⚠️ Disputed | 1 |") {
		t.Error("should show 1 disputed finding")
	}
}

func TestParseTrackingComment_InvalidJSON(t *testing.T) {
	body := `<!-- CODE_REVIEWER_TRACKING_V1 -->
## Tracking
<!-- TRACKING_METADATA
{invalid json}
-->`

	_, err := ParseTrackingComment(body)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseTrackingComment_MissingMetadata(t *testing.T) {
	body := `<!-- CODE_REVIEWER_TRACKING_V1 -->
## Tracking
No metadata here`

	_, err := ParseTrackingComment(body)
	if err == nil {
		t.Error("expected error for missing metadata")
	}
}

func TestParseTrackingComment_EmptyMetadata(t *testing.T) {
	body := `<!-- CODE_REVIEWER_TRACKING_V1 -->
<!-- TRACKING_METADATA
-->`

	_, err := ParseTrackingComment(body)
	if err == nil {
		t.Error("expected error for empty metadata")
	}
}

func TestParseTrackingComment_InvalidStatus(t *testing.T) {
	// A comment with an invalid status should default to "open"
	body := `<!-- CODE_REVIEWER_TRACKING_V1 -->
<!-- TRACKING_METADATA
{
  "version": 1,
  "repository": "owner/repo",
  "pr_number": 1,
  "head_sha": "abc123",
  "reviewed_commits": [],
  "findings": [{
    "fingerprint": "test123",
    "status": "invalid_status",
    "first_seen": "2024-01-01T00:00:00Z",
    "last_seen": "2024-01-01T00:00:00Z",
    "seen_count": 1,
    "finding_id": "abc",
    "file": "test.go",
    "line_start": 1,
    "line_end": 1,
    "severity": "high",
    "category": "test",
    "description": "test",
    "suggestion": "",
    "evidence": false
  }],
  "last_updated": "2024-01-01T00:00:00Z"
}
-->`

	state, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment() error = %v", err)
	}

	// Should have defaulted to open
	for _, f := range state.Findings {
		if f.Status != domain.FindingStatusOpen {
			t.Errorf("invalid status should default to open, got %s", f.Status)
		}
	}
}

func TestRenderTrackingComment_InProgress(t *testing.T) {
	now := time.Now()
	state := review.NewTrackingStateInProgress(review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   1,
		HeadSHA:    "abc123def456",
	}, now)

	body, err := RenderTrackingComment(state)
	if err != nil {
		t.Fatalf("RenderTrackingComment() error = %v", err)
	}

	if !IsTrackingComment(body) {
		t.Error("rendered comment should be identifiable")
	}

	// Should show in-progress header
	if !strings.Contains(body, "## 🔄 Code Review In Progress") {
		t.Error("should show in-progress header")
	}

	// Should show commit being reviewed (short SHA)
	if !strings.Contains(body, "abc123d") {
		t.Error("should show short SHA of commit being reviewed")
	}

	// Should NOT show status table
	if strings.Contains(body, "| 🔴 Open |") {
		t.Error("in-progress should not show status table")
	}
}

func TestRenderTrackingComment_InProgressRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	originalState := review.NewTrackingStateInProgress(review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   42,
		HeadSHA:    "abc123",
		BaseSHA:    "def456",
		Branch:     "feature",
	}, now)

	// Render
	body, err := RenderTrackingComment(originalState)
	if err != nil {
		t.Fatalf("RenderTrackingComment() error = %v", err)
	}

	// Parse back
	parsedState, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment() error = %v", err)
	}

	// Verify round-trip preserves ReviewStatus
	if parsedState.ReviewStatus != domain.ReviewStatusInProgress {
		t.Errorf("ReviewStatus = %s, want %s", parsedState.ReviewStatus, domain.ReviewStatusInProgress)
	}
	if parsedState.Target.PRNumber != originalState.Target.PRNumber {
		t.Errorf("PRNumber = %d, want %d", parsedState.Target.PRNumber, originalState.Target.PRNumber)
	}
}

func TestParseTrackingComment_BackwardCompatibility(t *testing.T) {
	// Old comments without review_status field should default to "completed"
	body := `<!-- CODE_REVIEWER_TRACKING_V1 -->
<!-- TRACKING_METADATA
{
  "version": 1,
  "repository": "owner/repo",
  "pr_number": 1,
  "head_sha": "abc123",
  "reviewed_commits": [],
  "findings": [],
  "last_updated": "2024-01-01T00:00:00Z"
}
-->`

	state, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment() error = %v", err)
	}

	// Should default to completed for backward compatibility
	if state.ReviewStatus != domain.ReviewStatusCompleted {
		t.Errorf("ReviewStatus = %s, want %s (backward compatibility)", state.ReviewStatus, domain.ReviewStatusCompleted)
	}
}

func TestParseTrackingComment_InvalidReviewStatus(t *testing.T) {
	// Comments with invalid non-empty review_status should default to "completed"
	// and log a warning (logging not verified in this test)
	body := `<!-- CODE_REVIEWER_TRACKING_V1 -->
<!-- TRACKING_METADATA
{
  "version": 1,
  "repository": "owner/repo",
  "pr_number": 1,
  "head_sha": "abc123",
  "reviewed_commits": [],
  "findings": [],
  "last_updated": "2024-01-01T00:00:00Z",
  "review_status": "invalid_status"
}
-->`

	state, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment() error = %v", err)
	}

	// Invalid status should default to completed
	if state.ReviewStatus != domain.ReviewStatusCompleted {
		t.Errorf("ReviewStatus = %s, want %s for invalid status", state.ReviewStatus, domain.ReviewStatusCompleted)
	}
}

func TestRenderTrackingComment_InProgressToCompleted(t *testing.T) {
	// Test the core workflow: in-progress state transitions to completed with findings
	now := time.Now().Truncate(time.Second)

	// Create in-progress state
	inProgressState := review.NewTrackingStateInProgress(review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   42,
		HeadSHA:    "abc123",
	}, now)

	// Render in-progress
	body, err := RenderTrackingComment(inProgressState)
	if err != nil {
		t.Fatalf("RenderTrackingComment(in-progress) error = %v", err)
	}

	if !strings.Contains(body, "## 🔄 Code Review In Progress") {
		t.Error("in-progress state should show in-progress header")
	}

	// Parse it back
	parsedState, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment() error = %v", err)
	}

	// Verify it's still in-progress
	if parsedState.ReviewStatus != domain.ReviewStatusInProgress {
		t.Errorf("parsed ReviewStatus = %s, want %s", parsedState.ReviewStatus, domain.ReviewStatusInProgress)
	}

	// Now transition to completed with a finding
	finding := createTestTrackedFinding(t, "test.go", domain.FindingStatusOpen, now)
	parsedState.Findings = map[domain.FindingFingerprint]domain.TrackedFinding{
		finding.Fingerprint: finding,
	}
	parsedState.ReviewStatus = domain.ReviewStatusCompleted
	parsedState.ReviewedCommits = []string{"abc123"}

	// Render the completed state
	completedBody, err := RenderTrackingComment(parsedState)
	if err != nil {
		t.Fatalf("RenderTrackingComment(completed) error = %v", err)
	}

	// Verify completed state rendering
	if !strings.Contains(completedBody, "## 🤖 Code Review Completed") {
		t.Error("completed state should show completed header")
	}
	if !strings.Contains(completedBody, "| 🔴 Open | 1 |") {
		t.Error("completed state should show 1 open finding")
	}
	if !strings.Contains(completedBody, "abc123") {
		t.Error("completed state should show reviewed commit")
	}

	// Parse completed and verify round-trip
	finalState, err := ParseTrackingComment(completedBody)
	if err != nil {
		t.Fatalf("ParseTrackingComment(completed) error = %v", err)
	}

	if finalState.ReviewStatus != domain.ReviewStatusCompleted {
		t.Errorf("final ReviewStatus = %s, want %s", finalState.ReviewStatus, domain.ReviewStatusCompleted)
	}
	if len(finalState.Findings) != 1 {
		t.Errorf("final Findings count = %d, want 1", len(finalState.Findings))
	}
}

func createTestTrackedFinding(t *testing.T, file string, status domain.FindingStatus, timestamp time.Time) domain.TrackedFinding {
	t.Helper()

	finding := domain.NewFinding(domain.FindingInput{
		File:        file,
		LineStart:   1,
		LineEnd:     1,
		Severity:    "medium",
		Category:    "test",
		Description: "Test finding for " + file,
		Suggestion:  "",
		Evidence:    false,
	})

	input := domain.TrackedFindingInput{
		Finding:   finding,
		Status:    status,
		FirstSeen: timestamp,
		LastSeen:  timestamp,
		SeenCount: 1,
	}

	// Resolved status requires ResolvedAt
	if status == domain.FindingStatusResolved {
		input.ResolvedAt = &timestamp
	}

	tf, err := domain.NewTrackedFinding(input)
	if err != nil {
		t.Fatalf("failed to create tracked finding: %v", err)
	}

	return tf
}

func TestParseTrackingComment_DashboardMetadataFormat(t *testing.T) {
	// Test that ParseTrackingComment can parse dashboard-formatted comments
	// (using DASHBOARD_METADATA_B64 marker instead of TRACKING_METADATA_B64)
	now := time.Now().Truncate(time.Second)

	originalState := review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   42,
			Branch:     "feature",
			BaseSHA:    "base123",
			HeadSHA:    "head456",
		},
		ReviewedCommits: []string{"commit1", "commit2", "commit3"},
		Findings:        make(map[domain.FindingFingerprint]domain.TrackedFinding),
		LastUpdated:     now,
		ReviewStatus:    domain.ReviewStatusCompleted,
	}

	// Create a dashboard-formatted comment body
	// This simulates what DashboardRenderer produces
	body := `<!-- CODE_REVIEWER_DASHBOARD_V1 -->

## ✅ Code Review Complete

| Status | Count |
|--------|-------|
| 🔴 Open | 0 |
| ✅ Resolved | 0 |

<!-- DASHBOARD_METADATA_B64
eyJ2ZXJzaW9uIjoxLCJyZXBvc2l0b3J5Ijoib3duZXIvcmVwbyIsInByX251bWJlciI6NDIsImJyYW5jaCI6ImZlYXR1cmUiLCJiYXNlX3NoYSI6ImJhc2UxMjMiLCJoZWFkX3NoYSI6ImhlYWQ0NTYiLCJyZXZpZXdlZF9jb21taXRzIjpbImNvbW1pdDEiLCJjb21taXQyIiwiY29tbWl0MyJdLCJmaW5kaW5ncyI6W10sImxhc3RfdXBkYXRlZCI6IjIwMjQtMDEtMDFUMTI6MDA6MDBaIiwicmV2aWV3X3N0YXR1cyI6ImNvbXBsZXRlZCJ9
-->`

	// Parse the dashboard comment
	parsedState, err := ParseTrackingComment(body)
	if err != nil {
		t.Fatalf("ParseTrackingComment failed: %v", err)
	}

	// Verify key fields are preserved
	if parsedState.Target.Repository != originalState.Target.Repository {
		t.Errorf("Repository mismatch: got %q, want %q",
			parsedState.Target.Repository, originalState.Target.Repository)
	}

	if parsedState.Target.PRNumber != originalState.Target.PRNumber {
		t.Errorf("PRNumber mismatch: got %d, want %d",
			parsedState.Target.PRNumber, originalState.Target.PRNumber)
	}

	if len(parsedState.ReviewedCommits) != 3 {
		t.Errorf("ReviewedCommits count mismatch: got %d, want 3",
			len(parsedState.ReviewedCommits))
	}

	// Verify all commits are preserved
	expectedCommits := []string{"commit1", "commit2", "commit3"}
	for i, expected := range expectedCommits {
		if i >= len(parsedState.ReviewedCommits) {
			t.Errorf("Missing commit at index %d", i)
			continue
		}
		if parsedState.ReviewedCommits[i] != expected {
			t.Errorf("Commit mismatch at index %d: got %q, want %q",
				i, parsedState.ReviewedCommits[i], expected)
		}
	}

	if parsedState.ReviewStatus != domain.ReviewStatusCompleted {
		t.Errorf("ReviewStatus mismatch: got %q, want %q",
			parsedState.ReviewStatus, domain.ReviewStatusCompleted)
	}
}
