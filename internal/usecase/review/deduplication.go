package review

import (
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// ReconciliationResult captures the outcome of reconciling new findings against
// existing tracked findings. Each slice represents a different category of findings
// that the caller can handle appropriately.
type ReconciliationResult struct {
	// New contains findings that have never been seen before.
	// These should be posted to GitHub and added to the tracking state.
	New []domain.Finding

	// Updated contains findings that were already tracked and seen again.
	// Their LastSeen and SeenCount have been updated.
	// Status is preserved (open, acknowledged, disputed stay as-is).
	Updated []domain.TrackedFinding

	// RedetectedResolved contains findings that were previously resolved but
	// detected again in this review. They remain in resolved status but are
	// reported so the caller can surface a warning to the user.
	RedetectedResolved []domain.TrackedFinding

	// Resolved contains findings that were automatically resolved because
	// they were open, in a changed file, and no longer detected.
	Resolved []domain.TrackedFinding
}

// ReconcileFindings compares new findings from a review against the existing
// tracking state and returns a categorized result plus an updated state.
//
// The function:
//  1. Identifies genuinely new findings (no matching fingerprint)
//  2. Updates existing findings that are seen again (preserving status)
//  3. Reports resolved findings that are re-detected (stays resolved, but warned)
//  4. Auto-resolves open findings in changed files that are no longer detected
//
// Parameters:
//   - state: Current tracking state (not mutated)
//   - newFindings: Findings from the current review
//   - changedFiles: Files modified in the current diff (for auto-resolve scope)
//   - commitSHA: Current commit (for ResolvedIn tracking)
//   - timestamp: Current time (for deterministic testing)
//
// Returns:
//   - TrackingState: Updated state with modified findings (new findings NOT added)
//   - ReconciliationResult: Categorized findings for caller to process
//
// Note: New findings are returned but NOT added to the state. The caller is
// responsible for creating TrackedFindings from them and adding to state.
// This keeps the function focused on reconciliation logic.
func ReconcileFindings(
	state TrackingState,
	newFindings []domain.Finding,
	changedFiles []string,
	commitSHA string,
	timestamp time.Time,
) (TrackingState, ReconciliationResult) {
	// Create a copy of the state to avoid mutating input
	newState := copyTrackingState(state)

	// Build lookup for changed files
	changedFileSet := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedFileSet[f] = true
	}

	// Build lookup for current findings by fingerprint
	currentFingerprintSet := make(map[domain.FindingFingerprint]bool, len(newFindings))
	for _, f := range newFindings {
		currentFingerprintSet[f.Fingerprint()] = true
	}

	var result ReconciliationResult

	// Process each new finding
	for _, finding := range newFindings {
		fp := finding.Fingerprint()

		existing, exists := newState.Findings[fp]
		if !exists {
			// Genuinely new finding
			result.New = append(result.New, finding)
			continue
		}

		// Finding exists - check its status
		switch existing.Status {
		case domain.FindingStatusResolved:
			// Re-detected while resolved - report but keep resolved
			result.RedetectedResolved = append(result.RedetectedResolved, existing)

		case domain.FindingStatusOpen, domain.FindingStatusAcknowledged, domain.FindingStatusDisputed:
			// Update LastSeen and SeenCount, preserve status
			existing.MarkSeen(timestamp)
			newState.Findings[fp] = existing
			result.Updated = append(result.Updated, existing)
		}
	}

	// Auto-resolve: find open findings in changed files that weren't detected
	for fp, tracked := range newState.Findings {
		// Only auto-resolve open findings
		if tracked.Status != domain.FindingStatusOpen {
			continue
		}

		// Only if the file was in the changed set
		if !changedFileSet[tracked.Finding.File] {
			continue
		}

		// Only if not detected in current findings
		if currentFingerprintSet[fp] {
			continue
		}

		// Auto-resolve this finding
		if err := tracked.UpdateStatus(domain.FindingStatusResolved, "Finding no longer present in review", commitSHA, timestamp); err != nil {
			// This shouldn't happen with valid status, but if it does, skip
			continue
		}
		newState.Findings[fp] = tracked
		result.Resolved = append(result.Resolved, tracked)
	}

	return newState, result
}

// copyTrackingState creates a copy of the tracking state.
// The Findings map entries are copied so modifications don't affect the original.
// Note: TrackedFinding is a value type, so map entry copies are independent.
func copyTrackingState(state TrackingState) TrackingState {
	newFindings := make(map[domain.FindingFingerprint]domain.TrackedFinding, len(state.Findings))
	for fp, tf := range state.Findings {
		newFindings[fp] = tf
	}

	// Copy ReviewedCommits slice
	var newReviewedCommits []string
	if len(state.ReviewedCommits) > 0 {
		newReviewedCommits = make([]string, len(state.ReviewedCommits))
		copy(newReviewedCommits, state.ReviewedCommits)
	}

	return TrackingState{
		Target:          state.Target,
		ReviewedCommits: newReviewedCommits,
		Findings:        newFindings,
		LastUpdated:     state.LastUpdated,
	}
}
