package review

import (
	"context"
	"errors"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// mockGitEngine is a test double for GitEngine.
type mockGitEngine struct {
	cumulativeDiff     domain.Diff
	cumulativeDiffErr  error
	incrementalDiff    domain.Diff
	incrementalDiffErr error
	commitExistsMap    map[string]bool
	commitExistsErr    error // If set, CommitExists returns this error
}

func (m *mockGitEngine) GetCumulativeDiff(ctx context.Context, baseRef, targetRef string, includeUncommitted bool) (domain.Diff, error) {
	return m.cumulativeDiff, m.cumulativeDiffErr
}

func (m *mockGitEngine) GetIncrementalDiff(ctx context.Context, fromCommit, toCommit string) (domain.Diff, error) {
	return m.incrementalDiff, m.incrementalDiffErr
}

func (m *mockGitEngine) CommitExists(ctx context.Context, commitSHA string) (bool, error) {
	if m.commitExistsErr != nil {
		return false, m.commitExistsErr
	}
	if m.commitExistsMap == nil {
		return false, nil
	}
	return m.commitExistsMap[commitSHA], nil
}

func (m *mockGitEngine) CurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}

func TestDiffComputer_NoTrackingState_ReturnsFullDiff(t *testing.T) {
	ctx := context.Background()
	expectedDiff := domain.Diff{
		FromCommitHash: "base123",
		ToCommitHash:   "head456",
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
		},
	}

	git := &mockGitEngine{cumulativeDiff: expectedDiff}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: "head456",
	}

	diff, err := computer.ComputeDiffForReview(ctx, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff.ToCommitHash != expectedDiff.ToCommitHash {
		t.Errorf("ToCommitHash = %s, want %s", diff.ToCommitHash, expectedDiff.ToCommitHash)
	}
	if len(diff.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(diff.Files))
	}
}

func TestDiffComputer_EmptyReviewedCommits_ReturnsFullDiff(t *testing.T) {
	ctx := context.Background()
	expectedDiff := domain.Diff{
		FromCommitHash: "base123",
		ToCommitHash:   "head456",
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
		},
	}

	git := &mockGitEngine{cumulativeDiff: expectedDiff}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: "head456",
	}

	// State with empty reviewed commits (first review)
	state := &TrackingState{
		ReviewedCommits: []string{},
	}

	diff, err := computer.ComputeDiffForReview(ctx, req, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff.ToCommitHash != expectedDiff.ToCommitHash {
		t.Errorf("ToCommitHash = %s, want %s", diff.ToCommitHash, expectedDiff.ToCommitHash)
	}
}

func TestDiffComputer_HasReviewedCommit_ReturnsIncrementalDiff(t *testing.T) {
	ctx := context.Background()
	lastReviewedCommit := "reviewed123"
	currentHead := "head456"

	fullDiff := domain.Diff{
		FromCommitHash: "base",
		ToCommitHash:   currentHead,
		Files: []domain.FileDiff{
			{Path: "old_change.go", Status: domain.FileStatusModified},
			{Path: "new_change.go", Status: domain.FileStatusAdded},
		},
	}

	incrementalDiff := domain.Diff{
		FromCommitHash: lastReviewedCommit,
		ToCommitHash:   currentHead,
		Files: []domain.FileDiff{
			{Path: "new_change.go", Status: domain.FileStatusAdded},
		},
	}

	git := &mockGitEngine{
		cumulativeDiff:  fullDiff,
		incrementalDiff: incrementalDiff,
		commitExistsMap: map[string]bool{lastReviewedCommit: true},
	}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: currentHead,
	}

	state := &TrackingState{
		ReviewedCommits: []string{"older123", lastReviewedCommit},
	}

	diff, err := computer.ComputeDiffForReview(ctx, req, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return incremental diff, not full diff
	if diff.FromCommitHash != lastReviewedCommit {
		t.Errorf("FromCommitHash = %s, want %s (incremental)", diff.FromCommitHash, lastReviewedCommit)
	}
	if len(diff.Files) != 1 {
		t.Errorf("Files count = %d, want 1 (only new changes)", len(diff.Files))
	}
	if len(diff.Files) > 0 && diff.Files[0].Path != "new_change.go" {
		t.Errorf("File path = %s, want new_change.go", diff.Files[0].Path)
	}
}

func TestDiffComputer_ForcePush_FallsBackToFullDiff(t *testing.T) {
	ctx := context.Background()
	lastReviewedCommit := "reviewed123"
	currentHead := "head456"

	fullDiff := domain.Diff{
		FromCommitHash: "base",
		ToCommitHash:   currentHead,
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
		},
	}

	git := &mockGitEngine{
		cumulativeDiff: fullDiff,
		// Commit no longer exists (force push)
		commitExistsMap: map[string]bool{lastReviewedCommit: false},
	}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: currentHead,
	}

	state := &TrackingState{
		ReviewedCommits: []string{lastReviewedCommit},
	}

	diff, err := computer.ComputeDiffForReview(ctx, req, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to full diff since commit doesn't exist
	if diff.FromCommitHash != fullDiff.FromCommitHash {
		t.Errorf("FromCommitHash = %s, want %s (full diff)", diff.FromCommitHash, fullDiff.FromCommitHash)
	}
}

func TestDiffComputer_CumulativeDiffError_ReturnsError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("git error")

	git := &mockGitEngine{cumulativeDiffErr: expectedErr}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: "head456",
	}

	_, err := computer.ComputeDiffForReview(ctx, req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestDiffComputer_IncrementalDiffError_ReturnsError(t *testing.T) {
	ctx := context.Background()
	lastReviewedCommit := "reviewed123"
	expectedErr := errors.New("incremental diff error")

	git := &mockGitEngine{
		incrementalDiffErr: expectedErr,
		commitExistsMap:    map[string]bool{lastReviewedCommit: true},
	}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: "head456",
	}

	state := &TrackingState{
		ReviewedCommits: []string{lastReviewedCommit},
	}

	_, err := computer.ComputeDiffForReview(ctx, req, state)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestDiffComputer_SameCommitAlreadyReviewed_ReturnsEmptyDiff(t *testing.T) {
	ctx := context.Background()
	currentHead := "head456"

	git := &mockGitEngine{
		commitExistsMap: map[string]bool{currentHead: true},
		// Incremental diff from same commit to same commit should be empty
		incrementalDiff: domain.Diff{
			FromCommitHash: currentHead,
			ToCommitHash:   currentHead,
			Files:          []domain.FileDiff{},
		},
	}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: currentHead,
	}

	// Current head was already reviewed
	state := &TrackingState{
		ReviewedCommits: []string{currentHead},
	}

	diff, err := computer.ComputeDiffForReview(ctx, req, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty diff (nothing new to review)
	if len(diff.Files) != 0 {
		t.Errorf("Files count = %d, want 0 (nothing new)", len(diff.Files))
	}
}

func TestDiffComputer_CommitExistsError_FallsBackToFullDiff(t *testing.T) {
	ctx := context.Background()
	lastReviewedCommit := "reviewed123"
	currentHead := "head456"

	fullDiff := domain.Diff{
		FromCommitHash: "base",
		ToCommitHash:   currentHead,
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
		},
	}

	git := &mockGitEngine{
		cumulativeDiff: fullDiff,
		// Simulate error checking commit existence (e.g., repo access issue)
		commitExistsErr: errors.New("failed to open repo"),
	}
	computer := NewDiffComputer(git)

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		CommitSHA: currentHead,
	}

	state := &TrackingState{
		ReviewedCommits: []string{lastReviewedCommit},
	}

	diff, err := computer.ComputeDiffForReview(ctx, req, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to full diff when CommitExists returns an error
	if diff.FromCommitHash != fullDiff.FromCommitHash {
		t.Errorf("FromCommitHash = %s, want %s (full diff fallback)", diff.FromCommitHash, fullDiff.FromCommitHash)
	}
}
