package review_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/brandon/code-reviewer/internal/domain"
	"github.com/brandon/code-reviewer/internal/usecase/review"
)

type mockGitEngine struct {
	baseRef            string
	targetRef          string
	includeUncommitted bool
	diff               domain.Diff
	err                error
	branch             string
	branchErr          error
}

func (m *mockGitEngine) GetCumulativeDiff(ctx context.Context, baseRef, targetRef string, includeUncommitted bool) (domain.Diff, error) {
	m.baseRef = baseRef
	m.targetRef = targetRef
	m.includeUncommitted = includeUncommitted
	return m.diff, m.err
}

func (m *mockGitEngine) CurrentBranch(ctx context.Context) (string, error) {
	return m.branch, m.branchErr
}

type mockProvider struct {
	requests []review.ProviderRequest
	response domain.Review
	err      error
}

func (m *mockProvider) Review(ctx context.Context, req review.ProviderRequest) (domain.Review, error) {
	m.requests = append(m.requests, req)
	return m.response, m.err
}

type mockMarkdownWriter struct {
	calls []review.MarkdownArtifact
	err   error
}

func (m *mockMarkdownWriter) Write(ctx context.Context, artifact review.MarkdownArtifact) (string, error) {
	m.calls = append(m.calls, artifact)
	if m.err != nil {
		return "", m.err
	}
	return filepath.Join(artifact.OutputDir, "review-"+artifact.ProviderName+".md"), nil
}

func TestReviewBranchWithSingleProvider(t *testing.T) {
	ctx := context.Background()
	diff := domain.Diff{
		FromCommitHash: "abc",
		ToCommitHash:   "def",
		Files: []domain.FileDiff{
			{Path: "main.go", Status: "modified", Patch: "@@ -0,0 +1,2 @@\n+package main\n+func main() {}\n"},
		},
	}
	expectedReview := domain.Review{
		ProviderName: "stub-openai",
		ModelName:    "gpt-4o",
		Summary:      "No issues found.",
		Findings: []domain.Finding{
			{
				ID:          "hash",
				File:        "main.go",
				LineStart:   1,
				LineEnd:     1,
				Severity:    "low",
				Category:    "style",
				Description: "Example finding",
				Suggestion:  "Refactor main",
				Evidence:    true,
			},
		},
	}

	gitMock := &mockGitEngine{diff: diff}
	providerMock := &mockProvider{response: expectedReview}
	writerMock := &mockMarkdownWriter{}

	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git:           gitMock,
		Provider:      providerMock,
		Markdown:      writerMock,
		SeedGenerator: func(baseRef, targetRef string) uint64 { return 42 },
		PromptBuilder: func(d domain.Diff, req review.BranchRequest) (review.ProviderRequest, error) {
			if d.ToCommitHash != diff.ToCommitHash {
				t.Fatalf("unexpected diff passed to prompt builder: %+v", d)
			}
			return review.ProviderRequest{
				Prompt:  "prompt",
				Seed:    42,
				MaxSize: 16384,
			}, nil
		},
	})

	result, err := orchestrator.ReviewBranch(ctx, review.BranchRequest{
		BaseRef:            "main",
		TargetRef:          "feature",
		OutputDir:          t.TempDir(),
		IncludeUncommitted: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if gitMock.baseRef != "main" || gitMock.targetRef != "feature" || !gitMock.includeUncommitted {
		t.Fatalf("git engine received unexpected inputs: base=%s target=%s include=%t", gitMock.baseRef, gitMock.targetRef, gitMock.includeUncommitted)
	}

	if len(providerMock.requests) != 1 {
		t.Fatalf("expected provider to be called once, got %d", len(providerMock.requests))
	}

	if providerMock.requests[0].Seed != 42 {
		t.Fatalf("expected seed of 42, got %d", providerMock.requests[0].Seed)
	}

	if len(writerMock.calls) != 1 {
		t.Fatalf("expected markdown writer to be called once, got %d", len(writerMock.calls))
	}

	if writerMock.calls[0].Review.Summary != expectedReview.Summary {
		t.Fatalf("markdown writer received wrong review summary: %s", writerMock.calls[0].Review.Summary)
	}

	if result.MarkdownPath == "" {
		t.Fatalf("expected markdown path to be populated")
	}
}

func TestCurrentBranchDelegatesToGitEngine(t *testing.T) {
	ctx := context.Background()
	gitMock := &mockGitEngine{branch: "main"}
	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git:           gitMock,
		Provider:      &mockProvider{},
		Markdown:      &mockMarkdownWriter{},
		SeedGenerator: func(_, _ string) uint64 { return 0 },
		PromptBuilder: func(domain.Diff, review.BranchRequest) (review.ProviderRequest, error) {
			return review.ProviderRequest{}, nil
		},
	})

	branch, err := orchestrator.CurrentBranch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "main" {
		t.Fatalf("expected branch main, got %s", branch)
	}
}
