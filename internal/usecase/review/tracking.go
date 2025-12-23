package review

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// ReviewTarget identifies the scope being reviewed.
// It supports both PR-based reviews (GitHub) and branch-based reviews (CLI).
type ReviewTarget struct {
	// Repository identifier in "owner/repo" format.
	Repository string

	// PRNumber is the pull request number. Zero for non-PR reviews.
	PRNumber int

	// Branch is the target branch name (for CLI reviews or additional context).
	Branch string

	// BaseSHA is the base commit for diff comparison.
	BaseSHA string

	// HeadSHA is the head commit being reviewed.
	HeadSHA string
}

// Validate checks that required target fields are populated.
func (rt ReviewTarget) Validate() error {
	if rt.Repository == "" {
		return errors.New("repository is required")
	}
	if rt.HeadSHA == "" {
		return errors.New("head SHA is required")
	}
	// BaseSHA can be empty for first review
	// PRNumber can be 0 for CLI reviews
	// Branch can be empty when PRNumber is set
	return nil
}

// Key generates a unique storage key for this target.
// For PRs, key is based on repo and PR number.
// For branch reviews, key is based on repo and branch name.
func (rt ReviewTarget) Key() string {
	if rt.PRNumber > 0 {
		return fmt.Sprintf("%s:pr:%d", rt.Repository, rt.PRNumber)
	}
	return fmt.Sprintf("%s:branch:%s", rt.Repository, rt.Branch)
}

// TrackingState captures the current state of findings for a review target.
type TrackingState struct {
	// Target identifies what is being tracked.
	Target ReviewTarget

	// ReviewedCommits contains SHAs of commits that have been reviewed.
	ReviewedCommits []string

	// Findings maps fingerprints to tracked findings.
	Findings map[domain.FindingFingerprint]domain.TrackedFinding

	// LastUpdated is the timestamp of the last state update.
	LastUpdated time.Time

	// ReviewStatus indicates the lifecycle state of the current review.
	// Use ReviewStatusInProgress when the review is running, ReviewStatusCompleted when done.
	ReviewStatus domain.ReviewStatus
}

// NewTrackingState creates a new empty tracking state for a target.
// The default ReviewStatus is Completed for backward compatibility.
func NewTrackingState(target ReviewTarget) TrackingState {
	return TrackingState{
		Target:          target,
		ReviewedCommits: []string{},
		Findings:        make(map[domain.FindingFingerprint]domain.TrackedFinding),
		LastUpdated:     time.Time{},
		ReviewStatus:    domain.ReviewStatusCompleted,
	}
}

// NewTrackingStateInProgress creates a tracking state for a review that is currently running.
// This is used to post a "review in progress" comment before the LLM analysis begins.
func NewTrackingStateInProgress(target ReviewTarget, timestamp time.Time) TrackingState {
	return TrackingState{
		Target:          target,
		ReviewedCommits: []string{},
		Findings:        make(map[domain.FindingFingerprint]domain.TrackedFinding),
		LastUpdated:     timestamp,
		ReviewStatus:    domain.ReviewStatusInProgress,
	}
}

// HasBeenReviewed returns true if the given commit has already been reviewed.
func (ts TrackingState) HasBeenReviewed(commitSHA string) bool {
	for _, sha := range ts.ReviewedCommits {
		if sha == commitSHA {
			return true
		}
	}
	return false
}

// ActiveFindings returns all findings that are still active (not resolved).
func (ts TrackingState) ActiveFindings() []domain.TrackedFinding {
	var active []domain.TrackedFinding
	for _, f := range ts.Findings {
		if f.IsActive() {
			active = append(active, f)
		}
	}
	return active
}

// LatestReviewedCommit returns the most recently reviewed commit SHA.
// Returns empty string if no commits have been reviewed.
func (ts TrackingState) LatestReviewedCommit() string {
	if len(ts.ReviewedCommits) == 0 {
		return ""
	}
	return ts.ReviewedCommits[len(ts.ReviewedCommits)-1]
}

// TrackingStore manages persistence of finding tracking state.
// Implementations can store state in GitHub PR comments, SQLite, or in-memory.
type TrackingStore interface {
	// Load retrieves the current tracking state for a target.
	// Returns an empty state (not error) if no prior state exists.
	// The returned state will have Target set to the input target.
	Load(ctx context.Context, target ReviewTarget) (TrackingState, error)

	// Save persists the tracking state for a target.
	// The state.Target field determines where the state is stored.
	Save(ctx context.Context, state TrackingState) error

	// Clear removes all tracking state for a target.
	// This is typically called when a PR is merged or closed.
	// Clearing a non-existent target is not an error.
	Clear(ctx context.Context, target ReviewTarget) error
}
