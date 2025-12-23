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
			name: "marker in middle",
			body: "Some text\n<!-- CODE_REVIEWER_TRACKING_V1 -->\nMore text",
			want: true,
		},
		{
			name: "no marker",
			body: "Just a regular comment",
			want: false,
		},
		{
			name: "partial marker",
			body: "<!-- CODE_REVIEWER_TRACKING",
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

	trackedFinding, err := domain.NewTrackedFindingFromFinding(finding, now)
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
	if !strings.Contains(body, "## 🤖 Code Review Tracking") {
		t.Error("comment should contain header")
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

	tf, err := domain.NewTrackedFinding(domain.TrackedFindingInput{
		Finding:   finding,
		Status:    status,
		FirstSeen: timestamp,
		LastSeen:  timestamp,
		SeenCount: 1,
	})
	if err != nil {
		t.Fatalf("failed to create tracked finding: %v", err)
	}

	return tf
}
