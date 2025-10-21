package review

import (
	"context"
	"errors"
	"strings"

	"github.com/brandon/code-reviewer/internal/domain"
)

// GitEngine abstracts the cumulative diff retrieval.
type GitEngine interface {
	GetCumulativeDiff(ctx context.Context, baseRef, targetRef string, includeUncommitted bool) (domain.Diff, error)
	CurrentBranch(ctx context.Context) (string, error)
}

// Provider defines the outbound port for LLM reviews.
type Provider interface {
	Review(ctx context.Context, req ProviderRequest) (domain.Review, error)
}

// MarkdownWriter persists provider output to disk.
type MarkdownWriter interface {
	Write(ctx context.Context, artifact MarkdownArtifact) (string, error)
}

// SeedFunc generates deterministic seeds per review scope.
type SeedFunc func(baseRef, targetRef string) uint64

// PromptBuilder constructs the provider request payload.
type PromptBuilder func(diff domain.Diff, req BranchRequest) (ProviderRequest, error)

// OrchestratorDeps captures the inbound dependencies for the orchestrator.
type OrchestratorDeps struct {
	Git           GitEngine
	Provider      Provider
	Markdown      MarkdownWriter
	SeedGenerator SeedFunc
	PromptBuilder PromptBuilder
}

// ProviderRequest describes the payload the LLM provider expects.
type ProviderRequest struct {
	Prompt  string
	Seed    uint64
	MaxSize int
}

// BranchRequest represents an inbound CLI request.
type BranchRequest struct {
	BaseRef            string
	TargetRef          string
	OutputDir          string
	Repository         string
	IncludeUncommitted bool
}

// MarkdownArtifact encapsulates the Markdown generation inputs.
type MarkdownArtifact struct {
	OutputDir    string
	Repository   string
	BaseRef      string
	TargetRef    string
	Diff         domain.Diff
	Review       domain.Review
	ProviderName string
}

// Result captures the orchestrator outcome.
type Result struct {
	MarkdownPath   string
	ProviderReview domain.Review
}

// Orchestrator implements the core review flow for Phase 1.
type Orchestrator struct {
	deps OrchestratorDeps
}

// NewOrchestrator wires the orchestrator dependencies.
func NewOrchestrator(deps OrchestratorDeps) *Orchestrator {
	return &Orchestrator{deps: deps}
}

// ReviewBranch executes a single-provider review for a Git branch diff.
func (o *Orchestrator) ReviewBranch(ctx context.Context, req BranchRequest) (Result, error) {
	if o.deps.Git == nil || o.deps.Provider == nil || o.deps.Markdown == nil || o.deps.PromptBuilder == nil || o.deps.SeedGenerator == nil {
		return Result{}, errors.New("orchestrator dependencies missing")
	}

	if err := validateRequest(req); err != nil {
		return Result{}, err
	}

	diff, err := o.deps.Git.GetCumulativeDiff(ctx, req.BaseRef, req.TargetRef, req.IncludeUncommitted)
	if err != nil {
		return Result{}, err
	}

	seed := o.deps.SeedGenerator(req.BaseRef, req.TargetRef)
	providerReq, err := o.deps.PromptBuilder(diff, req)
	if err != nil {
		return Result{}, err
	}
	if providerReq.Seed == 0 {
		providerReq.Seed = seed
	}

	review, err := o.deps.Provider.Review(ctx, providerReq)
	if err != nil {
		return Result{}, err
	}

	markdownPath, err := o.deps.Markdown.Write(ctx, MarkdownArtifact{
		OutputDir:    req.OutputDir,
		Repository:   req.Repository,
		BaseRef:      req.BaseRef,
		TargetRef:    req.TargetRef,
		Diff:         diff,
		Review:       review,
		ProviderName: review.ProviderName,
	})
	if err != nil {
		return Result{}, err
	}

	return Result{
		MarkdownPath:   markdownPath,
		ProviderReview: review,
	}, nil
}

// CurrentBranch returns the checked-out branch name.
func (o *Orchestrator) CurrentBranch(ctx context.Context) (string, error) {
	if o.deps.Git == nil {
		return "", errors.New("orchestrator dependencies missing")
	}
	return o.deps.Git.CurrentBranch(ctx)
}

func validateRequest(req BranchRequest) error {
	if strings.TrimSpace(req.BaseRef) == "" {
		return errors.New("base ref is required")
	}
	if strings.TrimSpace(req.TargetRef) == "" {
		return errors.New("target ref is required")
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		return errors.New("output directory is required")
	}
	return nil
}
