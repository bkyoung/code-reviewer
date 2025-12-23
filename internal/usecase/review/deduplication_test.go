package review

import (
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

func TestReconcileFindings_GenuinelyNew(t *testing.T) {
	// Empty state, all findings are new
	state := NewTrackingState(ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   1,
		HeadSHA:    "abc123",
	})

	newFindings := []domain.Finding{
		createTestFinding("file1.go", 10, "high", "security", "SQL injection"),
		createTestFinding("file2.go", 20, "medium", "style", "Unused variable"),
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, []string{"file1.go", "file2.go"}, "def456", timestamp)

	// All findings should be genuinely new
	if len(result.New) != 2 {
		t.Errorf("expected 2 new findings, got %d", len(result.New))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated findings, got %d", len(result.Updated))
	}
	if len(result.RedetectedResolved) != 0 {
		t.Errorf("expected 0 redetected resolved, got %d", len(result.RedetectedResolved))
	}
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(result.Resolved))
	}

	// State should not be modified (new findings aren't added to state by reconcile)
	if len(newState.Findings) != 0 {
		t.Errorf("expected state findings to remain empty (caller adds new), got %d", len(newState.Findings))
	}
}

func TestReconcileFindings_ExistingUpdated(t *testing.T) {
	// State has an existing open finding, same finding is detected again
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "SQL injection")
	trackedFinding := createTrackedFindingFromFinding(t, existingFinding, firstSeen)

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	// Same finding detected again (possibly at different line)
	newFindings := []domain.Finding{
		createTestFinding("file1.go", 15, "high", "security", "SQL injection"), // Same fingerprint, different line
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, []string{"file1.go"}, "def456", timestamp)

	// Should be updated, not new
	if len(result.New) != 0 {
		t.Errorf("expected 0 new findings, got %d", len(result.New))
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated finding, got %d", len(result.Updated))
	}

	// Check that LastSeen and SeenCount were updated
	updated := result.Updated[0]
	if !updated.LastSeen.Equal(timestamp) {
		t.Errorf("LastSeen = %v, want %v", updated.LastSeen, timestamp)
	}
	if updated.SeenCount != 2 {
		t.Errorf("SeenCount = %d, want 2", updated.SeenCount)
	}

	// State should have updated finding
	if len(newState.Findings) != 1 {
		t.Errorf("expected 1 finding in state, got %d", len(newState.Findings))
	}
}

func TestReconcileFindings_AcknowledgedPreserved(t *testing.T) {
	// Acknowledged finding is detected again - status should be preserved
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "medium", "style", "Naming convention")
	trackedFinding := createTrackedFindingWithStatus(t, existingFinding, domain.FindingStatusAcknowledged, firstSeen, "Intentional design choice")

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	newFindings := []domain.Finding{
		createTestFinding("file1.go", 10, "medium", "style", "Naming convention"),
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	_, result := ReconcileFindings(state, newFindings, []string{"file1.go"}, "def456", timestamp)

	// Should be updated but status preserved
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated finding, got %d", len(result.Updated))
	}
	if result.Updated[0].Status != domain.FindingStatusAcknowledged {
		t.Errorf("status = %s, want acknowledged", result.Updated[0].Status)
	}
	if result.Updated[0].StatusReason != "Intentional design choice" {
		t.Errorf("status reason not preserved")
	}
}

func TestReconcileFindings_DisputedPreserved(t *testing.T) {
	// Disputed finding is detected again - status should be preserved
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "False positive")
	trackedFinding := createTrackedFindingWithStatus(t, existingFinding, domain.FindingStatusDisputed, firstSeen, "This is a false positive")

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	newFindings := []domain.Finding{
		createTestFinding("file1.go", 10, "high", "security", "False positive"),
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	_, result := ReconcileFindings(state, newFindings, []string{"file1.go"}, "def456", timestamp)

	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated finding, got %d", len(result.Updated))
	}
	if result.Updated[0].Status != domain.FindingStatusDisputed {
		t.Errorf("status = %s, want disputed", result.Updated[0].Status)
	}
}

func TestReconcileFindings_ResolvedRedetected(t *testing.T) {
	// Resolved finding is detected again - should be reported but stay resolved
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	resolvedAt := time.Date(2025, 1, 12, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "SQL injection")
	trackedFinding := createResolvedTrackedFinding(t, existingFinding, firstSeen, resolvedAt, "abc123")

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	newFindings := []domain.Finding{
		createTestFinding("file1.go", 10, "high", "security", "SQL injection"),
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, []string{"file1.go"}, "def456", timestamp)

	// Should NOT be in New (don't re-raise)
	if len(result.New) != 0 {
		t.Errorf("expected 0 new findings, got %d", len(result.New))
	}

	// Should be in RedetectedResolved
	if len(result.RedetectedResolved) != 1 {
		t.Errorf("expected 1 redetected resolved, got %d", len(result.RedetectedResolved))
	}

	// Status should remain resolved
	redetected := result.RedetectedResolved[0]
	if redetected.Status != domain.FindingStatusResolved {
		t.Errorf("status = %s, want resolved", redetected.Status)
	}

	// LastSeen and SeenCount should be updated even for resolved findings
	if !redetected.LastSeen.Equal(timestamp) {
		t.Errorf("LastSeen = %v, want %v", redetected.LastSeen, timestamp)
	}
	if redetected.SeenCount != 2 {
		t.Errorf("SeenCount = %d, want 2", redetected.SeenCount)
	}

	// State should still have finding as resolved with updated metadata
	fp := trackedFinding.Fingerprint
	if newState.Findings[fp].Status != domain.FindingStatusResolved {
		t.Errorf("state finding status = %s, want resolved", newState.Findings[fp].Status)
	}
	if newState.Findings[fp].SeenCount != 2 {
		t.Errorf("state finding SeenCount = %d, want 2", newState.Findings[fp].SeenCount)
	}
}

func TestReconcileFindings_DuplicateFindingsInInput(t *testing.T) {
	// Same fingerprint appears twice in newFindings - should only increment SeenCount once
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "SQL injection")
	trackedFinding := createTrackedFindingFromFinding(t, existingFinding, firstSeen)

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	// Same finding twice (different line numbers but same fingerprint since fingerprint excludes line)
	newFindings := []domain.Finding{
		createTestFinding("file1.go", 10, "high", "security", "SQL injection"),
		createTestFinding("file1.go", 15, "high", "security", "SQL injection"), // Same fingerprint
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	_, result := ReconcileFindings(state, newFindings, []string{"file1.go"}, "def456", timestamp)

	// Should only be counted once
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated finding, got %d", len(result.Updated))
	}

	// SeenCount should be 2 (original + 1), not 3 (original + 2)
	if result.Updated[0].SeenCount != 2 {
		t.Errorf("SeenCount = %d, want 2 (not inflated by duplicate)", result.Updated[0].SeenCount)
	}
}

func TestReconcileFindings_UnknownStatusTreatedAsOpen(t *testing.T) {
	// Finding with unknown status should be treated as open (graceful degradation).
	// This tests defensive programming for cases like:
	// - Data corruption in persisted state
	// - Schema migration from older versions with different status values
	// - Deserialization of externally-provided data
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "Issue")

	// Directly construct TrackedFinding to bypass NewTrackedFinding validation,
	// simulating corrupted or externally-loaded data with an invalid status.
	trackedFinding := domain.TrackedFinding{
		Finding:     existingFinding,
		Fingerprint: existingFinding.Fingerprint(),
		Status:      domain.FindingStatus("unknown_status"),
		FirstSeen:   firstSeen,
		LastSeen:    firstSeen,
		SeenCount:   1,
	}

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	newFindings := []domain.Finding{
		createTestFinding("file1.go", 10, "high", "security", "Issue"),
	}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, []string{"file1.go"}, "def456", timestamp)

	// Should be in Updated (treated as open)
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated finding, got %d", len(result.Updated))
	}

	// Should have recorded an error
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for unknown status, got %d", len(result.Errors))
	}

	// LastSeen and SeenCount should still be updated
	fp := trackedFinding.Fingerprint
	if newState.Findings[fp].SeenCount != 2 {
		t.Errorf("SeenCount = %d, want 2", newState.Findings[fp].SeenCount)
	}
}

func TestReconcileFindings_AutoResolveInChangedFile(t *testing.T) {
	// Open finding in a changed file is no longer detected - auto-resolve
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "SQL injection")
	trackedFinding := createTrackedFindingFromFinding(t, existingFinding, firstSeen)

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	// No findings detected in this review, but file1.go was changed
	newFindings := []domain.Finding{}
	changedFiles := []string{"file1.go"}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, changedFiles, "def456", timestamp)

	// Should be auto-resolved
	if len(result.Resolved) != 1 {
		t.Errorf("expected 1 resolved finding, got %d", len(result.Resolved))
	}

	resolved := result.Resolved[0]
	if resolved.Status != domain.FindingStatusResolved {
		t.Errorf("status = %s, want resolved", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Error("ResolvedAt should be set")
	}
	if resolved.ResolvedIn == nil || *resolved.ResolvedIn != "def456" {
		t.Errorf("ResolvedIn = %v, want def456", resolved.ResolvedIn)
	}

	// State should have finding as resolved
	fp := trackedFinding.Fingerprint
	if newState.Findings[fp].Status != domain.FindingStatusResolved {
		t.Errorf("state finding status = %s, want resolved", newState.Findings[fp].Status)
	}
}

func TestReconcileFindings_NoAutoResolveOutsideChangedFiles(t *testing.T) {
	// Open finding in a file that wasn't changed - should NOT be auto-resolved
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "SQL injection")
	trackedFinding := createTrackedFindingFromFinding(t, existingFinding, firstSeen)

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	// No findings detected, and file1.go was NOT in changed files
	newFindings := []domain.Finding{}
	changedFiles := []string{"file2.go"} // Different file

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, changedFiles, "def456", timestamp)

	// Should NOT be auto-resolved
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved findings, got %d", len(result.Resolved))
	}

	// State should still have finding as open
	fp := trackedFinding.Fingerprint
	if newState.Findings[fp].Status != domain.FindingStatusOpen {
		t.Errorf("state finding status = %s, want open", newState.Findings[fp].Status)
	}
}

func TestReconcileFindings_OnlyOpenAutoResolved(t *testing.T) {
	// Acknowledged finding in changed file should NOT be auto-resolved
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "medium", "style", "Naming")
	trackedFinding := createTrackedFindingWithStatus(t, existingFinding, domain.FindingStatusAcknowledged, firstSeen, "Intentional")

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	newFindings := []domain.Finding{}
	changedFiles := []string{"file1.go"}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, changedFiles, "def456", timestamp)

	// Should NOT be auto-resolved (only open findings are auto-resolved)
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved findings, got %d", len(result.Resolved))
	}

	// State should still have finding as acknowledged
	fp := trackedFinding.Fingerprint
	if newState.Findings[fp].Status != domain.FindingStatusAcknowledged {
		t.Errorf("state finding status = %s, want acknowledged", newState.Findings[fp].Status)
	}
}

func TestReconcileFindings_DisputedNotAutoResolved(t *testing.T) {
	// Disputed finding in changed file should NOT be auto-resolved
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	existingFinding := createTestFinding("file1.go", 10, "high", "security", "Disputed issue")
	trackedFinding := createTrackedFindingWithStatus(t, existingFinding, domain.FindingStatusDisputed, firstSeen, "False positive")

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
	}

	newFindings := []domain.Finding{}
	changedFiles := []string{"file1.go"}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, newFindings, changedFiles, "def456", timestamp)

	// Should NOT be auto-resolved (only open findings are auto-resolved)
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved findings, got %d", len(result.Resolved))
	}

	// State should still have finding as disputed
	fp := trackedFinding.Fingerprint
	if newState.Findings[fp].Status != domain.FindingStatusDisputed {
		t.Errorf("state finding status = %s, want disputed", newState.Findings[fp].Status)
	}
}

func TestReconcileFindings_MixedScenario(t *testing.T) {
	// Complex scenario with multiple findings in different states
	firstSeen := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	resolvedAt := time.Date(2025, 1, 12, 10, 0, 0, 0, time.UTC)

	openFinding := createTestFinding("file1.go", 10, "high", "security", "Open issue")
	openTracked := createTrackedFindingFromFinding(t, openFinding, firstSeen)

	ackFinding := createTestFinding("file2.go", 20, "medium", "style", "Acknowledged issue")
	ackTracked := createTrackedFindingWithStatus(t, ackFinding, domain.FindingStatusAcknowledged, firstSeen, "OK")

	resolvedFinding := createTestFinding("file3.go", 30, "high", "security", "Resolved issue")
	resolvedTracked := createResolvedTrackedFinding(t, resolvedFinding, firstSeen, resolvedAt, "prev123")

	toBeResolvedFinding := createTestFinding("file4.go", 40, "low", "docs", "Will be resolved")
	toBeResolvedTracked := createTrackedFindingFromFinding(t, toBeResolvedFinding, firstSeen)

	state := TrackingState{
		Target: ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   1,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			openTracked.Fingerprint:         openTracked,
			ackTracked.Fingerprint:          ackTracked,
			resolvedTracked.Fingerprint:     resolvedTracked,
			toBeResolvedTracked.Fingerprint: toBeResolvedTracked,
		},
	}

	// New findings: open redetected, ack redetected, resolved redetected, plus genuinely new
	newFindings := []domain.Finding{
		createTestFinding("file1.go", 15, "high", "security", "Open issue"),        // Open - will update
		createTestFinding("file2.go", 25, "medium", "style", "Acknowledged issue"), // Ack - will update, keep status
		createTestFinding("file3.go", 35, "high", "security", "Resolved issue"),    // Resolved - redetected
		createTestFinding("file5.go", 50, "critical", "security", "Brand new"),     // Genuinely new
	}
	// file4.go is in changed files but finding not detected - will be resolved
	changedFiles := []string{"file1.go", "file2.go", "file3.go", "file4.go", "file5.go"}

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	_, result := ReconcileFindings(state, newFindings, changedFiles, "def456", timestamp)

	if len(result.New) != 1 {
		t.Errorf("expected 1 new finding, got %d", len(result.New))
	}
	if len(result.Updated) != 2 {
		t.Errorf("expected 2 updated findings, got %d", len(result.Updated))
	}
	if len(result.RedetectedResolved) != 1 {
		t.Errorf("expected 1 redetected resolved, got %d", len(result.RedetectedResolved))
	}
	if len(result.Resolved) != 1 {
		t.Errorf("expected 1 auto-resolved, got %d", len(result.Resolved))
	}
}

func TestReconcileFindings_EmptyInputs(t *testing.T) {
	state := NewTrackingState(ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   1,
		HeadSHA:    "abc123",
	})

	timestamp := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	newState, result := ReconcileFindings(state, nil, nil, "def456", timestamp)

	if len(result.New) != 0 {
		t.Errorf("expected 0 new, got %d", len(result.New))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated, got %d", len(result.Updated))
	}
	if len(result.RedetectedResolved) != 0 {
		t.Errorf("expected 0 redetected, got %d", len(result.RedetectedResolved))
	}
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(result.Resolved))
	}
	if len(newState.Findings) != 0 {
		t.Errorf("expected 0 findings in state, got %d", len(newState.Findings))
	}
}

// Helper functions

func createTestFinding(file string, line int, severity, category, description string) domain.Finding {
	return domain.NewFinding(domain.FindingInput{
		File:        file,
		LineStart:   line,
		LineEnd:     line,
		Severity:    severity,
		Category:    category,
		Description: description,
		Suggestion:  "",
		Evidence:    false,
	})
}

func createTrackedFindingFromFinding(t *testing.T, f domain.Finding, timestamp time.Time) domain.TrackedFinding {
	t.Helper()
	tf, err := domain.NewTrackedFindingFromFinding(f, timestamp, "initial123")
	if err != nil {
		t.Fatalf("failed to create tracked finding: %v", err)
	}
	return tf
}

func createTrackedFindingWithStatus(t *testing.T, f domain.Finding, status domain.FindingStatus, timestamp time.Time, reason string) domain.TrackedFinding {
	t.Helper()

	input := domain.TrackedFindingInput{
		Finding:      f,
		Status:       status,
		FirstSeen:    timestamp,
		LastSeen:     timestamp,
		SeenCount:    1,
		StatusReason: reason,
		ReviewCommit: "initial123",
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

func createResolvedTrackedFinding(t *testing.T, f domain.Finding, firstSeen, resolvedAt time.Time, resolvedIn string) domain.TrackedFinding {
	t.Helper()

	input := domain.TrackedFindingInput{
		Finding:      f,
		Status:       domain.FindingStatusResolved,
		FirstSeen:    firstSeen,
		LastSeen:     resolvedAt,
		SeenCount:    1,
		StatusReason: "Fixed",
		ReviewCommit: "initial123",
		ResolvedAt:   &resolvedAt,
		ResolvedIn:   &resolvedIn,
	}

	tf, err := domain.NewTrackedFinding(input)
	if err != nil {
		t.Fatalf("failed to create tracked finding: %v", err)
	}
	return tf
}
