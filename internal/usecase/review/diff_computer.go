package review

import (
	"context"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// DiffComputer determines the appropriate diff for a review request.
// It encapsulates the logic for deciding between full and incremental diffs.
type DiffComputer struct {
	git GitEngine
}

// NewDiffComputer creates a DiffComputer with the given git engine.
func NewDiffComputer(git GitEngine) *DiffComputer {
	return &DiffComputer{git: git}
}

// ComputeDiffForReview determines the appropriate diff for a review request.
//
// Decision logic:
//   - If trackingState is nil (CLI mode or first-time setup): full diff
//   - If no commits have been reviewed yet (first PR review): full diff
//   - If the last reviewed commit no longer exists (force push): full diff
//   - If CommitExists returns an error: fall back to full diff (safe default)
//   - Otherwise: incremental diff from last reviewed commit to current head
//
// Note on race conditions: There is a TOCTOU (time-of-check-time-of-use) window
// between CommitExists and GetIncrementalDiff. If a force push occurs in this
// window, GetIncrementalDiff will fail with an error, which is acceptable since
// the caller will surface the error appropriately.
func (dc *DiffComputer) ComputeDiffForReview(
	ctx context.Context,
	req BranchRequest,
	trackingState *TrackingState,
) (domain.Diff, error) {
	// No tracking state means CLI mode or GitHub mode without prior reviews
	if trackingState == nil {
		return dc.git.GetCumulativeDiff(ctx, req.BaseRef, req.TargetRef, req.IncludeUncommitted)
	}

	// First review on this PR - no commits reviewed yet
	lastReviewed := trackingState.LatestReviewedCommit()
	if lastReviewed == "" {
		return dc.git.GetCumulativeDiff(ctx, req.BaseRef, req.TargetRef, req.IncludeUncommitted)
	}

	// Force push detection: does the last reviewed commit still exist?
	exists, err := dc.git.CommitExists(ctx, lastReviewed)
	if err != nil {
		// Error checking commit existence - fall back to full diff as safe default
		// This could be due to repo access issues, context cancellation, etc.
		return dc.git.GetCumulativeDiff(ctx, req.BaseRef, req.TargetRef, req.IncludeUncommitted)
	}
	if !exists {
		// Commit is gone (force push, rebase, branch deletion)
		// Fall back to full diff as the safe option
		return dc.git.GetCumulativeDiff(ctx, req.BaseRef, req.TargetRef, req.IncludeUncommitted)
	}

	// Incremental diff: only review changes since last reviewed commit
	return dc.git.GetIncrementalDiff(ctx, lastReviewed, req.CommitSHA)
}
