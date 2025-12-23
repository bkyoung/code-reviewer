package domain

import (
	"testing"
	"time"
)

func TestFindingStatus_IsValid(t *testing.T) {
	tests := []struct {
		status FindingStatus
		want   bool
	}{
		{FindingStatusOpen, true},
		{FindingStatusResolved, true},
		{FindingStatusAcknowledged, true},
		{FindingStatusDisputed, true},
		{FindingStatus("invalid"), false},
		{FindingStatus(""), false},
		{FindingStatus("OPEN"), false}, // Case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("FindingStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestNewFindingFingerprint_Deterministic(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")
	fp2 := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")

	if fp1 != fp2 {
		t.Errorf("fingerprints should be deterministic: %s != %s", fp1, fp2)
	}
}

func TestNewFindingFingerprint_UniqueAcrossFiles(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")
	fp2 := NewFindingFingerprint("db.go", "security", "high", "SQL injection risk")

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different files")
	}
}

func TestNewFindingFingerprint_UniqueAcrossCategories(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "Issue description")
	fp2 := NewFindingFingerprint("main.go", "performance", "high", "Issue description")

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different categories")
	}
}

func TestNewFindingFingerprint_UniqueAcrossSeverities(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "Issue description")
	fp2 := NewFindingFingerprint("main.go", "security", "low", "Issue description")

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different severities")
	}
}

func TestNewFindingFingerprint_TruncatesLongDescriptions(t *testing.T) {
	longDesc := make([]byte, 200)
	for i := range longDesc {
		longDesc[i] = 'a'
	}

	// Two descriptions that differ only after char 100 should have same fingerprint
	desc1 := string(longDesc)
	longDesc[150] = 'b'
	desc2 := string(longDesc)

	fp1 := NewFindingFingerprint("main.go", "security", "high", desc1)
	fp2 := NewFindingFingerprint("main.go", "security", "high", desc2)

	if fp1 != fp2 {
		t.Error("fingerprints should match when descriptions only differ after 100 chars")
	}
}

func TestFingerprintFromFinding(t *testing.T) {
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     15,
		Severity:    "high",
		Category:    "security",
		Description: "SQL injection risk",
		Suggestion:  "Use parameterized queries",
		Evidence:    true,
	})

	fp := FingerprintFromFinding(finding)
	expected := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")

	if fp != expected {
		t.Errorf("FingerprintFromFinding() = %s, want %s", fp, expected)
	}
}

func TestNewTrackedFinding_Valid(t *testing.T) {
	now := time.Now()
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

	tf, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 1,
	})

	if err != nil {
		t.Fatalf("NewTrackedFinding() error = %v", err)
	}

	if tf.Status != FindingStatusOpen {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusOpen)
	}

	if tf.SeenCount != 1 {
		t.Errorf("SeenCount = %d, want 1", tf.SeenCount)
	}

	if tf.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}
}

func TestNewTrackedFinding_EmptyFindingID(t *testing.T) {
	now := time.Now()
	finding := Finding{} // Empty ID

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 1,
	})

	if err == nil {
		t.Error("expected error for empty finding ID")
	}
}

func TestNewTrackedFinding_InvalidStatus(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatus("invalid"),
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 1,
	})

	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestNewTrackedFinding_ZeroSeenCount(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 0,
	})

	if err == nil {
		t.Error("expected error for zero seen count")
	}
}

func TestNewTrackedFinding_LastSeenBeforeFirstSeen(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now.Add(-time.Hour), // Before first seen
		SeenCount: 1,
	})

	if err == nil {
		t.Error("expected error when LastSeen is before FirstSeen")
	}
}

func TestNewTrackedFindingFromFinding(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, err := NewTrackedFindingFromFinding(finding, now, "abc123")
	if err != nil {
		t.Fatalf("NewTrackedFindingFromFinding() error = %v", err)
	}

	if tf.Status != FindingStatusOpen {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusOpen)
	}

	if tf.SeenCount != 1 {
		t.Errorf("SeenCount = %d, want 1", tf.SeenCount)
	}
}

func TestTrackedFinding_MarkSeen(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "abc123")
	tf.MarkSeen(later)

	if tf.SeenCount != 2 {
		t.Errorf("SeenCount = %d, want 2", tf.SeenCount)
	}

	if tf.LastSeen != later {
		t.Errorf("LastSeen = %v, want %v", tf.LastSeen, later)
	}

	if tf.FirstSeen != now {
		t.Errorf("FirstSeen should not change: got %v, want %v", tf.FirstSeen, now)
	}
}

func TestTrackedFinding_UpdateStatus_Basic(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "abc123")

	if err := tf.UpdateStatus(FindingStatusResolved, "fixed", "commit-sha", now); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusResolved {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusResolved)
	}
}

func TestTrackedFinding_UpdateStatus_InvalidStatus(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "abc123")

	if err := tf.UpdateStatus(FindingStatus("invalid"), "", "", time.Time{}); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestTrackedFinding_IsActive(t *testing.T) {
	tests := []struct {
		status FindingStatus
		want   bool
	}{
		{FindingStatusOpen, true},
		{FindingStatusResolved, false},
		{FindingStatusAcknowledged, false},
		{FindingStatusDisputed, false},
	}

	now := time.Now()
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			finding := NewFinding(FindingInput{
				File:        "main.go",
				LineStart:   10,
				LineEnd:     10,
				Severity:    "high",
				Category:    "security",
				Description: "Test",
				Suggestion:  "",
				Evidence:    false,
			})

			input := TrackedFindingInput{
				Finding:   finding,
				Status:    tt.status,
				FirstSeen: now,
				LastSeen:  now,
				SeenCount: 1,
			}

			// Resolved status requires ResolvedAt
			if tt.status == FindingStatusResolved {
				input.ResolvedAt = &now
			}

			tf, _ := NewTrackedFinding(input)

			if got := tf.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackedFinding_IsResolved(t *testing.T) {
	tests := []struct {
		status FindingStatus
		want   bool
	}{
		{FindingStatusOpen, false},
		{FindingStatusResolved, true},
		{FindingStatusAcknowledged, false},
		{FindingStatusDisputed, false},
	}

	now := time.Now()
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			finding := NewFinding(FindingInput{
				File:        "main.go",
				LineStart:   10,
				LineEnd:     10,
				Severity:    "high",
				Category:    "security",
				Description: "Test",
				Suggestion:  "",
				Evidence:    false,
			})

			input := TrackedFindingInput{
				Finding:   finding,
				Status:    tt.status,
				FirstSeen: now,
				LastSeen:  now,
				SeenCount: 1,
			}

			// Resolved status requires ResolvedAt
			if tt.status == FindingStatusResolved {
				input.ResolvedAt = &now
			}

			tf, _ := NewTrackedFinding(input)

			if got := tf.IsResolved(); got != tt.want {
				t.Errorf("IsResolved() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Tests for new status transition fields and behavior (Issue #55)

func TestTrackedFinding_NewFieldsInitialization(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	resolvedAt := now.Add(-time.Hour)
	resolvedIn := "abc123"

	tf, err := NewTrackedFinding(TrackedFindingInput{
		Finding:      finding,
		Status:       FindingStatusResolved,
		FirstSeen:    now.Add(-2 * time.Hour),
		LastSeen:     now,
		SeenCount:    3,
		StatusReason: "Fixed in refactor",
		ReviewCommit: "def456",
		ResolvedAt:   &resolvedAt,
		ResolvedIn:   &resolvedIn,
	})

	if err != nil {
		t.Fatalf("NewTrackedFinding() error = %v", err)
	}

	if tf.StatusReason != "Fixed in refactor" {
		t.Errorf("StatusReason = %q, want %q", tf.StatusReason, "Fixed in refactor")
	}

	if tf.ReviewCommit != "def456" {
		t.Errorf("ReviewCommit = %q, want %q", tf.ReviewCommit, "def456")
	}

	if tf.ResolvedAt == nil || !tf.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("ResolvedAt = %v, want %v", tf.ResolvedAt, resolvedAt)
	}

	if tf.ResolvedIn == nil || *tf.ResolvedIn != "abc123" {
		t.Errorf("ResolvedIn = %v, want %q", tf.ResolvedIn, "abc123")
	}
}

func TestNewTrackedFinding_StatusReasonMaxLength(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	// Create a reason that exceeds 500 characters
	longReason := make([]byte, 501)
	for i := range longReason {
		longReason[i] = 'a'
	}

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:      finding,
		Status:       FindingStatusAcknowledged,
		FirstSeen:    now,
		LastSeen:     now,
		SeenCount:    1,
		StatusReason: string(longReason),
		ReviewCommit: "abc123",
	})

	if err == nil {
		t.Error("expected error for status reason exceeding 500 characters")
	}
}

func TestNewTrackedFinding_StatusReasonAtMaxLength(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	// Create a reason that is exactly 500 characters
	maxReason := make([]byte, 500)
	for i := range maxReason {
		maxReason[i] = 'a'
	}

	tf, err := NewTrackedFinding(TrackedFindingInput{
		Finding:      finding,
		Status:       FindingStatusAcknowledged,
		FirstSeen:    now,
		LastSeen:     now,
		SeenCount:    1,
		StatusReason: string(maxReason),
		ReviewCommit: "abc123",
	})

	if err != nil {
		t.Fatalf("NewTrackedFinding() error = %v (500 chars should be valid)", err)
	}

	if len(tf.StatusReason) != 500 {
		t.Errorf("StatusReason length = %d, want 500", len(tf.StatusReason))
	}
}

func TestNewTrackedFindingFromFinding_WithReviewCommit(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, err := NewTrackedFindingFromFinding(finding, now, "abc123")
	if err != nil {
		t.Fatalf("NewTrackedFindingFromFinding() error = %v", err)
	}

	if tf.ReviewCommit != "abc123" {
		t.Errorf("ReviewCommit = %q, want %q", tf.ReviewCommit, "abc123")
	}

	if tf.StatusReason != "" {
		t.Errorf("StatusReason should be empty for new finding, got %q", tf.StatusReason)
	}

	if tf.ResolvedAt != nil {
		t.Errorf("ResolvedAt should be nil for new finding, got %v", tf.ResolvedAt)
	}

	if tf.ResolvedIn != nil {
		t.Errorf("ResolvedIn should be nil for new finding, got %v", tf.ResolvedIn)
	}
}

func TestTrackedFinding_UpdateStatus_TransitionToResolved(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "initial-commit")

	resolvedTime := now.Add(time.Hour)
	err := tf.UpdateStatus(FindingStatusResolved, "Fixed the bug", "fix-commit", resolvedTime)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusResolved {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusResolved)
	}

	if tf.StatusReason != "Fixed the bug" {
		t.Errorf("StatusReason = %q, want %q", tf.StatusReason, "Fixed the bug")
	}

	if tf.ResolvedAt == nil {
		t.Error("ResolvedAt should be set when transitioning to resolved")
	}

	if tf.ResolvedIn == nil || *tf.ResolvedIn != "fix-commit" {
		t.Errorf("ResolvedIn = %v, want %q", tf.ResolvedIn, "fix-commit")
	}
}

func TestTrackedFinding_UpdateStatus_TransitionToOpen_ClearsFields(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	resolvedAt := now
	resolvedIn := "fix-commit"

	// Start with a resolved finding
	tf, _ := NewTrackedFinding(TrackedFindingInput{
		Finding:      finding,
		Status:       FindingStatusResolved,
		FirstSeen:    now.Add(-time.Hour),
		LastSeen:     now,
		SeenCount:    2,
		StatusReason: "Previously fixed",
		ReviewCommit: "initial-commit",
		ResolvedAt:   &resolvedAt,
		ResolvedIn:   &resolvedIn,
	})

	// Transition back to open (reopen)
	err := tf.UpdateStatus(FindingStatusOpen, "", "", time.Time{})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusOpen {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusOpen)
	}

	if tf.StatusReason != "" {
		t.Errorf("StatusReason should be cleared, got %q", tf.StatusReason)
	}

	if tf.ResolvedAt != nil {
		t.Errorf("ResolvedAt should be cleared, got %v", tf.ResolvedAt)
	}

	if tf.ResolvedIn != nil {
		t.Errorf("ResolvedIn should be cleared, got %v", tf.ResolvedIn)
	}

	// ReviewCommit should NOT be cleared (immutable)
	if tf.ReviewCommit != "initial-commit" {
		t.Errorf("ReviewCommit should not change, got %q", tf.ReviewCommit)
	}
}

func TestTrackedFinding_UpdateStatus_TransitionToOpen_IgnoresParams(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	resolvedAt := now
	resolvedIn := "fix-commit"

	// Start with a resolved finding
	tf, _ := NewTrackedFinding(TrackedFindingInput{
		Finding:      finding,
		Status:       FindingStatusResolved,
		FirstSeen:    now.Add(-time.Hour),
		LastSeen:     now,
		SeenCount:    2,
		StatusReason: "Previously fixed",
		ReviewCommit: "initial-commit",
		ResolvedAt:   &resolvedAt,
		ResolvedIn:   &resolvedIn,
	})

	// Transition to open with non-empty params that should be ignored
	err := tf.UpdateStatus(FindingStatusOpen, "this reason should be ignored", "ignored-commit", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	// Status should be open
	if tf.Status != FindingStatusOpen {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusOpen)
	}

	// StatusReason should be cleared (not set to "this reason should be ignored")
	if tf.StatusReason != "" {
		t.Errorf("StatusReason should be cleared regardless of input, got %q", tf.StatusReason)
	}

	// Resolution fields should be cleared
	if tf.ResolvedAt != nil {
		t.Errorf("ResolvedAt should be cleared, got %v", tf.ResolvedAt)
	}

	if tf.ResolvedIn != nil {
		t.Errorf("ResolvedIn should be cleared, got %v", tf.ResolvedIn)
	}
}

func TestTrackedFinding_UpdateStatus_TransitionToAcknowledged(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "initial-commit")

	err := tf.UpdateStatus(FindingStatusAcknowledged, "Intentional design choice", "", time.Time{})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusAcknowledged {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusAcknowledged)
	}

	if tf.StatusReason != "Intentional design choice" {
		t.Errorf("StatusReason = %q, want %q", tf.StatusReason, "Intentional design choice")
	}

	// Acknowledged is not resolved, so these should remain nil
	if tf.ResolvedAt != nil {
		t.Errorf("ResolvedAt should be nil for acknowledged, got %v", tf.ResolvedAt)
	}

	if tf.ResolvedIn != nil {
		t.Errorf("ResolvedIn should be nil for acknowledged, got %v", tf.ResolvedIn)
	}
}

func TestTrackedFinding_UpdateStatus_TransitionToDisputed(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "initial-commit")

	err := tf.UpdateStatus(FindingStatusDisputed, "False positive - pattern is safe", "", time.Time{})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusDisputed {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusDisputed)
	}

	if tf.StatusReason != "False positive - pattern is safe" {
		t.Errorf("StatusReason = %q, want %q", tf.StatusReason, "False positive - pattern is safe")
	}
}

func TestTrackedFinding_UpdateStatus_ReasonMaxLength(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "initial-commit")

	// Create a reason that exceeds 500 characters
	longReason := make([]byte, 501)
	for i := range longReason {
		longReason[i] = 'a'
	}

	err := tf.UpdateStatus(FindingStatusAcknowledged, string(longReason), "", time.Time{})
	if err == nil {
		t.Error("expected error for status reason exceeding 500 characters")
	}

	// Status should not have changed
	if tf.Status != FindingStatusOpen {
		t.Errorf("Status should not change on error, got %v", tf.Status)
	}
}

func TestTrackedFinding_UpdateStatus_ResolvedWithEmptyCommit(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now, "initial-commit")

	// Transition to resolved without providing a commit SHA
	resolvedTime := now.Add(time.Hour)
	err := tf.UpdateStatus(FindingStatusResolved, "Fixed", "", resolvedTime)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusResolved {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusResolved)
	}

	// ResolvedAt should still be set
	if tf.ResolvedAt == nil {
		t.Error("ResolvedAt should be set even without commit SHA")
	}

	// ResolvedIn should be nil when no commit provided
	if tf.ResolvedIn != nil {
		t.Errorf("ResolvedIn should be nil when no commit provided, got %v", tf.ResolvedIn)
	}
}

func TestTrackedFinding_UpdateStatus_AllTransitionsAllowed(t *testing.T) {
	statuses := []FindingStatus{
		FindingStatusOpen,
		FindingStatusResolved,
		FindingStatusAcknowledged,
		FindingStatusDisputed,
	}

	now := time.Now()

	// Test that all transitions are allowed
	for _, fromStatus := range statuses {
		for _, toStatus := range statuses {
			t.Run(string(fromStatus)+"_to_"+string(toStatus), func(t *testing.T) {
				finding := NewFinding(FindingInput{
					File:        "main.go",
					LineStart:   10,
					LineEnd:     10,
					Severity:    "high",
					Category:    "security",
					Description: "Test",
					Suggestion:  "",
					Evidence:    false,
				})

				input := TrackedFindingInput{
					Finding:      finding,
					Status:       fromStatus,
					FirstSeen:    now,
					LastSeen:     now,
					SeenCount:    1,
					ReviewCommit: "abc123",
				}

				// Resolved status requires ResolvedAt
				if fromStatus == FindingStatusResolved {
					input.ResolvedAt = &now
				}

				tf, _ := NewTrackedFinding(input)

				err := tf.UpdateStatus(toStatus, "test reason", "commit-sha", now)
				if err != nil {
					t.Errorf("Transition from %s to %s should be allowed, got error: %v",
						fromStatus, toStatus, err)
				}

				if tf.Status != toStatus {
					t.Errorf("Status = %v, want %v", tf.Status, toStatus)
				}
			})
		}
	}
}
