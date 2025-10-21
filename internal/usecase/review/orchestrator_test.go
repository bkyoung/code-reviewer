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

type mockMerger struct {
	calls [][]domain.Review
}

func (m *mockMerger) Merge(reviews []domain.Review) domain.Review {
	m.calls = append(m.calls, reviews)
	return domain.Review{ProviderName: "merged", ModelName: "consensus"}
}

func (m *mockProvider) Review(ctx context.Context, req review.ProviderRequest) (domain.Review, error) {
	m.requests = append(m.requests, req)
	return m.response, m.err
}

type mockMarkdownWriter struct {
	calls []domain.MarkdownArtifact
	err   error
}

type mockJSONWriter struct {
	calls []domain.JSONArtifact
	err   error
}

type mockSARIFWriter struct {
	calls []review.SARIFArtifact
	err   error
}

func (m *mockJSONWriter) Write(ctx context.Context, artifact domain.JSONArtifact) (string, error) {
	m.calls = append(m.calls, artifact)
	if m.err != nil {
		return "", m.err
	}
	return filepath.Join(artifact.OutputDir, "review-"+artifact.ProviderName+".json"), nil
}

func (m *mockMarkdownWriter) Write(ctx context.Context, artifact domain.MarkdownArtifact) (string, error) {
	m.calls = append(m.calls, artifact)
	if m.err != nil {
		return "", m.err
	}
	return filepath.Join(artifact.OutputDir, "review-"+artifact.ProviderName+".md"), nil
}

func (m *mockSARIFWriter) Write(ctx context.Context, artifact review.SARIFArtifact) (string, error) {
	m.calls = append(m.calls, artifact)
	if m.err != nil {
		return "", m.err
	}
	return filepath.Join(artifact.OutputDir, "review-"+artifact.ProviderName+".sarif"), nil
}

func TestReviewBranchWithSingleProvider(t *testing.T) {
	ctx := context.Background()
	diff := domain.Diff{
		FromCommitHash: "abc",
		ToCommitHash:   "def",
		Files: []domain.FileDiff{
			{Path: "main.go", Status: "modified", Patch: "@@ -0,0 +1,2 @@\n+package main\n+func main() {}"},
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
	mergerMock := &mockMerger{}
	jsonWriterMock := &mockJSONWriter{}
	sarifWriterMock := &mockSARIFWriter{}

	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git: gitMock,
		Providers: map[string]review.Provider{
			"stub-openai": providerMock,
		},
		Merger:        mergerMock,
		Markdown:      writerMock,
		JSON:          jsonWriterMock,
		SARIF:         sarifWriterMock,
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

	if len(writerMock.calls) != 2 {
		t.Fatalf("expected markdown writer to be called twice (1 provider + 1 merged), got %d", len(writerMock.calls))
	}

	if writerMock.calls[0].Review.Summary != expectedReview.Summary {
		t.Fatalf("markdown writer received wrong review summary: %s", writerMock.calls[0].Review.Summary)
	}

	if result.MarkdownPaths["stub-openai"] == "" {
		t.Fatalf("expected markdown path to be populated for stub-openai")
	}

	if result.MarkdownPaths["merged"] == "" {
		t.Fatalf("expected markdown path to be populated for merged review")
	}

	if len(jsonWriterMock.calls) != 2 {
		t.Fatalf("expected json writer to be called twice (1 provider + 1 merged), got %d", len(jsonWriterMock.calls))
	}

	if result.JSONPaths["stub-openai"] == "" {
		t.Fatalf("expected json path to be populated for stub-openai")
	}

	if result.JSONPaths["merged"] == "" {
		t.Fatalf("expected json path to be populated for merged review")
	}

	if len(result.Reviews) != 2 {
		t.Fatalf("expected 2 reviews (1 provider + 1 merged), got %d", len(result.Reviews))
	}

	if result.Reviews[0].ProviderName != "stub-openai" {
		t.Fatalf("expected review from stub-openai, got %s", result.Reviews[0].ProviderName)
	}
}

func TestReviewBranchWithMultipleProviders(t *testing.T) {
	ctx := context.Background()
	diff := domain.Diff{
		FromCommitHash: "abc",
		ToCommitHash:   "def",
		Files:          []domain.FileDiff{{Path: "main.go", Status: "modified", Patch: "@@ -0,0 +1,2 @@\n+package main\n+func main() {}"}},
	}

	review1 := domain.Review{ProviderName: "provider1", ModelName: "model1", Summary: "Review 1"}
	review2 := domain.Review{ProviderName: "provider2", ModelName: "model2", Summary: "Review 2"}

	provider1 := &mockProvider{response: review1}
	provider2 := &mockProvider{response: review2}
	gitMock := &mockGitEngine{diff: diff}
	writerMock := &mockMarkdownWriter{}
	mergerMock := &mockMerger{}
	jsonWriterMock := &mockJSONWriter{}
	sarifWriterMock := &mockSARIFWriter{}

	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git: gitMock,
		Providers: map[string]review.Provider{
			"provider1": provider1,
			"provider2": provider2,
		},
		Merger:        mergerMock,
		Markdown:      writerMock,
		JSON:          jsonWriterMock,
		SARIF:         sarifWriterMock,
		SeedGenerator: func(_, _ string) uint64 { return 42 },
		PromptBuilder: func(d domain.Diff, req review.BranchRequest) (review.ProviderRequest, error) {
			return review.ProviderRequest{Prompt: "prompt", Seed: 42, MaxSize: 16384}, nil
		},
	})

	result, err := orchestrator.ReviewBranch(ctx, review.BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature",
		OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(provider1.requests) != 1 {
		t.Fatalf("expected provider1 to be called once, got %d", len(provider1.requests))
	}
	if len(provider2.requests) != 1 {
		t.Fatalf("expected provider2 to be called once, got %d", len(provider2.requests))
	}

	if len(writerMock.calls) != 3 {
		t.Fatalf("expected markdown writer to be called three times (2 providers + 1 merged), got %d", len(writerMock.calls))
	}

	if len(result.Reviews) != 3 {
		t.Fatalf("expected 3 reviews (2 provider + 1 merged), got %d", len(result.Reviews))
	}

	if result.MarkdownPaths["provider1"] == "" {
		t.Fatalf("expected markdown path for provider1")
	}
	if result.MarkdownPaths["provider2"] == "" {
		t.Fatalf("expected markdown path for provider2")
	}
	if result.MarkdownPaths["merged"] == "" {
		t.Fatalf("expected markdown path for merged review")
	}

	if len(jsonWriterMock.calls) != 3 {
		t.Fatalf("expected json writer to be called three times (2 providers + 1 merged), got %d", len(jsonWriterMock.calls))
	}

	if result.JSONPaths["provider1"] == "" {
		t.Fatalf("expected json path for provider1")
	}
	if result.JSONPaths["provider2"] == "" {
		t.Fatalf("expected json path for provider2")
	}
	if result.JSONPaths["merged"] == "" {
		t.Fatalf("expected json path for merged review")
	}

	if len(mergerMock.calls) != 1 {
		t.Fatalf("expected merger to be called once, got %d", len(mergerMock.calls))
	}
}

func TestCurrentBranchDelegatesToGitEngine(t *testing.T) {
	ctx := context.Background()
	gitMock := &mockGitEngine{branch: "main"}
	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git:           gitMock,
		Providers:     map[string]review.Provider{"mock": &mockProvider{}},
		Merger:        &mockMerger{},
		Markdown:      &mockMarkdownWriter{},
		JSON:          &mockJSONWriter{},
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
