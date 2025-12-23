package review

import (
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

func TestReviewTarget_Validate(t *testing.T) {
	tests := []struct {
		name    string
		target  ReviewTarget
		wantErr bool
	}{
		{
			name: "valid PR review",
			target: ReviewTarget{
				Repository: "owner/repo",
				PRNumber:   123,
				HeadSHA:    "abc123",
			},
			wantErr: false,
		},
		{
			name: "valid branch review",
			target: ReviewTarget{
				Repository: "owner/repo",
				Branch:     "feature-branch",
				HeadSHA:    "abc123",
			},
			wantErr: false,
		},
		{
			name: "missing repository",
			target: ReviewTarget{
				PRNumber: 123,
				HeadSHA:  "abc123",
			},
			wantErr: true,
		},
		{
			name: "missing head SHA",
			target: ReviewTarget{
				Repository: "owner/repo",
				PRNumber:   123,
			},
			wantErr: true,
		},
		{
			name: "minimal valid - repo and head SHA only",
			target: ReviewTarget{
				Repository: "owner/repo",
				HeadSHA:    "abc123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReviewTarget_Key(t *testing.T) {
	tests := []struct {
		name   string
		target ReviewTarget
		want   string
	}{
		{
			name: "PR-based key",
			target: ReviewTarget{
				Repository: "owner/repo",
				PRNumber:   123,
				Branch:     "feature",
				HeadSHA:    "abc123",
			},
			want: "owner/repo:pr:123",
		},
		{
			name: "branch-based key",
			target: ReviewTarget{
				Repository: "owner/repo",
				Branch:     "feature-branch",
				HeadSHA:    "abc123",
			},
			want: "owner/repo:branch:feature-branch",
		},
		{
			name: "PR takes precedence over branch",
			target: ReviewTarget{
				Repository: "owner/repo",
				PRNumber:   42,
				Branch:     "some-branch",
				HeadSHA:    "abc123",
			},
			want: "owner/repo:pr:42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.target.Key(); got != tt.want {
				t.Errorf("Key() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewTrackingState(t *testing.T) {
	target := ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	state := NewTrackingState(target)

	if state.Target.Repository != target.Repository {
		t.Errorf("Repository = %s, want %s", state.Target.Repository, target.Repository)
	}
	if state.Target.PRNumber != target.PRNumber {
		t.Errorf("PRNumber = %d, want %d", state.Target.PRNumber, target.PRNumber)
	}
	if len(state.ReviewedCommits) != 0 {
		t.Errorf("ReviewedCommits should be empty, got %d", len(state.ReviewedCommits))
	}
	if len(state.Findings) != 0 {
		t.Errorf("Findings should be empty, got %d", len(state.Findings))
	}
	if !state.LastUpdated.IsZero() {
		t.Errorf("LastUpdated should be zero time, got %v", state.LastUpdated)
	}
}

func TestTrackingState_HasBeenReviewed(t *testing.T) {
	state := TrackingState{
		ReviewedCommits: []string{"abc123", "def456", "ghi789"},
	}

	tests := []struct {
		commitSHA string
		want      bool
	}{
		{"abc123", true},
		{"def456", true},
		{"ghi789", true},
		{"xyz000", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.commitSHA, func(t *testing.T) {
			if got := state.HasBeenReviewed(tt.commitSHA); got != tt.want {
				t.Errorf("HasBeenReviewed(%q) = %v, want %v", tt.commitSHA, got, tt.want)
			}
		})
	}
}

func TestTrackingState_HasBeenReviewed_Empty(t *testing.T) {
	state := NewTrackingState(ReviewTarget{
		Repository: "owner/repo",
		HeadSHA:    "abc123",
	})

	if state.HasBeenReviewed("abc123") {
		t.Error("empty state should not have reviewed any commits")
	}
}

func TestTrackingState_ActiveFindings(t *testing.T) {
	now := time.Now()

	// Create findings with different statuses
	openFinding := createTestTrackedFinding(t, "open.go", domain.FindingStatusOpen, now)
	resolvedFinding := createTestTrackedFinding(t, "resolved.go", domain.FindingStatusResolved, now)
	ackFinding := createTestTrackedFinding(t, "ack.go", domain.FindingStatusAcknowledged, now)
	disputedFinding := createTestTrackedFinding(t, "disputed.go", domain.FindingStatusDisputed, now)

	state := TrackingState{
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			openFinding.Fingerprint:     openFinding,
			resolvedFinding.Fingerprint: resolvedFinding,
			ackFinding.Fingerprint:      ackFinding,
			disputedFinding.Fingerprint: disputedFinding,
		},
	}

	active := state.ActiveFindings()

	// Only open status is considered active
	if len(active) != 1 {
		t.Errorf("ActiveFindings() returned %d, want 1", len(active))
	}

	if len(active) > 0 && active[0].Finding.File != "open.go" {
		t.Errorf("active finding file = %s, want open.go", active[0].Finding.File)
	}
}

func TestTrackingState_ActiveFindings_Empty(t *testing.T) {
	state := NewTrackingState(ReviewTarget{
		Repository: "owner/repo",
		HeadSHA:    "abc123",
	})

	active := state.ActiveFindings()
	if len(active) != 0 {
		t.Errorf("empty state should have no active findings, got %d", len(active))
	}
}

func TestTrackingState_ActiveFindings_AllResolved(t *testing.T) {
	now := time.Now()

	resolved1 := createTestTrackedFinding(t, "file1.go", domain.FindingStatusResolved, now)
	resolved2 := createTestTrackedFinding(t, "file2.go", domain.FindingStatusResolved, now)

	state := TrackingState{
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			resolved1.Fingerprint: resolved1,
			resolved2.Fingerprint: resolved2,
		},
	}

	active := state.ActiveFindings()
	if len(active) != 0 {
		t.Errorf("all-resolved state should have no active findings, got %d", len(active))
	}
}

func TestTrackingState_LatestReviewedCommit(t *testing.T) {
	tests := []struct {
		name            string
		reviewedCommits []string
		want            string
	}{
		{
			name:            "empty returns empty string",
			reviewedCommits: []string{},
			want:            "",
		},
		{
			name:            "nil returns empty string",
			reviewedCommits: nil,
			want:            "",
		},
		{
			name:            "single commit",
			reviewedCommits: []string{"abc123"},
			want:            "abc123",
		},
		{
			name:            "multiple commits returns last",
			reviewedCommits: []string{"abc123", "def456", "ghi789"},
			want:            "ghi789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := TrackingState{
				ReviewedCommits: tt.reviewedCommits,
			}
			if got := state.LatestReviewedCommit(); got != tt.want {
				t.Errorf("LatestReviewedCommit() = %q, want %q", got, tt.want)
			}
		})
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
