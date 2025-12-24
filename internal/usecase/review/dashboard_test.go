package review

import (
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

func TestNewDashboardDataInProgress(t *testing.T) {
	target := ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   1,
		HeadSHA:    "abc123",
	}
	timestamp := time.Now()

	existingCommits := []string{"commit1", "commit2"}
	existingFindings := map[domain.FindingFingerprint]domain.TrackedFinding{
		"fp1": {Fingerprint: "fp1", Status: domain.FindingStatusOpen},
	}

	data := NewDashboardDataInProgress(target, timestamp, existingCommits, existingFindings)

	if data.ReviewStatus != domain.ReviewStatusInProgress {
		t.Errorf("expected ReviewStatus to be InProgress, got %v", data.ReviewStatus)
	}

	if len(data.ReviewedCommits) != 2 {
		t.Errorf("expected 2 reviewed commits, got %d", len(data.ReviewedCommits))
	}

	if len(data.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(data.Findings))
	}

	// Verify deep copy - modifying original shouldn't affect data
	existingCommits[0] = "modified"
	if data.ReviewedCommits[0] == "modified" {
		t.Error("deep copy failed: modifying original commits affected data")
	}
}

func TestNewDashboardDataCompleted(t *testing.T) {
	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		ReviewedCommits: []string{"commit1"},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {Fingerprint: "fp1", Status: domain.FindingStatusOpen},
		},
		LastUpdated:  time.Now(),
		ReviewStatus: domain.ReviewStatusCompleted,
	}

	review := &domain.Review{
		ProviderName: "test-provider",
		Cost:         0.01,
	}

	attentionSeverities := map[string]bool{"critical": true, "high": true}

	data := NewDashboardDataCompleted(state, review, nil, attentionSeverities, nil)

	if data.ReviewStatus != domain.ReviewStatusCompleted {
		t.Errorf("expected ReviewStatus to be Completed, got %v", data.ReviewStatus)
	}

	if data.Review == nil {
		t.Error("expected Review to be set")
	}

	if data.Review.ProviderName != "test-provider" {
		t.Errorf("expected provider name 'test-provider', got %s", data.Review.ProviderName)
	}

	if len(data.AttentionSeverities) != 2 {
		t.Errorf("expected 2 attention severities, got %d", len(data.AttentionSeverities))
	}

	// Verify deep copy - modifying originals shouldn't affect data
	state.ReviewedCommits[0] = "modified"
	if data.ReviewedCommits[0] == "modified" {
		t.Error("deep copy failed: modifying original commits affected data")
	}

	attentionSeverities["medium"] = true
	if data.AttentionSeverities["medium"] {
		t.Error("deep copy failed: modifying original attentionSeverities affected data")
	}

	state.Findings["fp2"] = domain.TrackedFinding{Fingerprint: "fp2"}
	if _, exists := data.Findings["fp2"]; exists {
		t.Error("deep copy failed: modifying original findings affected data")
	}
}

func TestDashboardDataCountByStatus(t *testing.T) {
	data := DashboardData{
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {Fingerprint: "fp1", Status: domain.FindingStatusOpen},
			"fp2": {Fingerprint: "fp2", Status: domain.FindingStatusOpen},
			"fp3": {Fingerprint: "fp3", Status: domain.FindingStatusResolved},
			"fp4": {Fingerprint: "fp4", Status: domain.FindingStatusAcknowledged},
		},
	}

	counts := data.CountByStatus()

	if counts.Open != 2 {
		t.Errorf("expected 2 open, got %d", counts.Open)
	}

	if counts.Resolved != 1 {
		t.Errorf("expected 1 resolved, got %d", counts.Resolved)
	}

	if counts.Acknowledged != 1 {
		t.Errorf("expected 1 acknowledged, got %d", counts.Acknowledged)
	}

	if counts.Disputed != 0 {
		t.Errorf("expected 0 disputed, got %d", counts.Disputed)
	}

	if counts.Total != 4 {
		t.Errorf("expected total 4, got %d", counts.Total)
	}
}

func TestDashboardDataCountBySeverity(t *testing.T) {
	data := DashboardData{
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			"fp1": {
				Fingerprint: "fp1",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "critical"},
			},
			"fp2": {
				Fingerprint: "fp2",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "high"},
			},
			"fp3": {
				Fingerprint: "fp3",
				Status:      domain.FindingStatusOpen,
				Finding:     domain.Finding{Severity: "high"},
			},
			"fp4": {
				Fingerprint: "fp4",
				Status:      domain.FindingStatusResolved, // Should not count
				Finding:     domain.Finding{Severity: "medium"},
			},
		},
	}

	counts := data.CountBySeverity()

	if counts.Critical != 1 {
		t.Errorf("expected 1 critical, got %d", counts.Critical)
	}

	if counts.High != 2 {
		t.Errorf("expected 2 high, got %d", counts.High)
	}

	if counts.Medium != 0 {
		t.Errorf("expected 0 medium (resolved should not count), got %d", counts.Medium)
	}

	if counts.Total != 3 {
		t.Errorf("expected total 3 (only open findings), got %d", counts.Total)
	}
}

func TestDashboardDataHasBlockingFindings(t *testing.T) {
	tests := []struct {
		name       string
		findings   map[domain.FindingFingerprint]domain.TrackedFinding
		severities map[string]bool
		expected   bool
	}{
		{
			name: "has blocking critical finding",
			findings: map[domain.FindingFingerprint]domain.TrackedFinding{
				"fp1": {
					Fingerprint: "fp1",
					Status:      domain.FindingStatusOpen,
					Finding:     domain.Finding{Severity: "critical"},
				},
			},
			severities: map[string]bool{"critical": true, "high": true},
			expected:   true,
		},
		{
			name: "only resolved blocking finding",
			findings: map[domain.FindingFingerprint]domain.TrackedFinding{
				"fp1": {
					Fingerprint: "fp1",
					Status:      domain.FindingStatusResolved,
					Finding:     domain.Finding{Severity: "critical"},
				},
			},
			severities: map[string]bool{"critical": true, "high": true},
			expected:   false,
		},
		{
			name: "only non-blocking findings",
			findings: map[domain.FindingFingerprint]domain.TrackedFinding{
				"fp1": {
					Fingerprint: "fp1",
					Status:      domain.FindingStatusOpen,
					Finding:     domain.Finding{Severity: "low"},
				},
			},
			severities: map[string]bool{"critical": true, "high": true},
			expected:   false,
		},
		{
			name:       "no findings",
			findings:   map[domain.FindingFingerprint]domain.TrackedFinding{},
			severities: map[string]bool{"critical": true, "high": true},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := DashboardData{
				Findings:            tt.findings,
				AttentionSeverities: tt.severities,
			}

			result := data.HasBlockingFindings()
			if result != tt.expected {
				t.Errorf("expected HasBlockingFindings()=%v, got %v", tt.expected, result)
			}
		})
	}
}
