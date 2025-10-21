package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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

// Merger defines the outbound port for merging reviews.
type Merger interface {
	Merge(reviews []domain.Review) domain.Review
}

// MarkdownWriter persists provider output to disk.
type MarkdownWriter interface {
	Write(ctx context.Context, artifact domain.MarkdownArtifact) (string, error)
}

// JSONWriter persists provider output to disk.
type JSONWriter interface {
	Write(ctx context.Context, artifact domain.JSONArtifact) (string, error)
}

// SARIFWriter persists provider output to disk in SARIF format.
type SARIFWriter interface {
	Write(ctx context.Context, artifact SARIFArtifact) (string, error)
}

// SARIFArtifact encapsulates the SARIF generation inputs.
type SARIFArtifact struct {
	OutputDir    string
	Repository   string
	BaseRef      string
	TargetRef    string
	Review       domain.Review
	ProviderName string
}

// SeedFunc generates deterministic seeds per review scope.
type SeedFunc func(baseRef, targetRef string) uint64

// PromptBuilder constructs the provider request payload.
type PromptBuilder func(diff domain.Diff, req BranchRequest) (ProviderRequest, error)

// Redactor defines the outbound port for secret redaction.
type Redactor interface {
	Redact(input string) (string, error)
}

// Store defines the outbound port for persisting review history.
type Store interface {
	CreateRun(ctx context.Context, run StoreRun) error
	SaveReview(ctx context.Context, review StoreReview) error
	SaveFindings(ctx context.Context, findings []StoreFinding) error
	Close() error
}

// StoreRun represents a review run for persistence.
type StoreRun struct {
	RunID      string
	Timestamp  time.Time
	Scope      string
	ConfigHash string
	TotalCost  float64
	BaseRef    string
	TargetRef  string
	Repository string
}

// StoreReview represents a review record for persistence.
type StoreReview struct {
	ReviewID  string
	RunID     string
	Provider  string
	Model     string
	Summary   string
	CreatedAt time.Time
}

// StoreFinding represents a finding record for persistence.
type StoreFinding struct {
	FindingID   string
	ReviewID    string
	FindingHash string
	File        string
	LineStart   int
	LineEnd     int
	Category    string
	Severity    string
	Description string
	Suggestion  string
	Evidence    bool
}

// OrchestratorDeps captures the inbound dependencies for the orchestrator.
type OrchestratorDeps struct {
	Git           GitEngine
	Providers     map[string]Provider
	Merger        Merger
	Markdown      MarkdownWriter
	JSON          JSONWriter
	SARIF         SARIFWriter
	Redactor      Redactor
	SeedGenerator SeedFunc
	PromptBuilder PromptBuilder
	Store         Store // Optional: persistence layer for review history
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

// Result captures the orchestrator outcome.
type Result struct {
	MarkdownPaths map[string]string
	JSONPaths     map[string]string
	SARIFPaths    map[string]string
	Reviews       []domain.Review
}

// Orchestrator implements the core review flow for Phase 1.
type Orchestrator struct {
	deps OrchestratorDeps
}

// NewOrchestrator wires the orchestrator dependencies.
func NewOrchestrator(deps OrchestratorDeps) *Orchestrator {
	return &Orchestrator{deps: deps}
}

// validateDependencies checks that all required dependencies are present.
func (o *Orchestrator) validateDependencies() error {
	if o.deps.Git == nil {
		return errors.New("git engine is required")
	}
	if o.deps.Providers == nil || len(o.deps.Providers) == 0 {
		return errors.New("at least one provider is required")
	}
	if o.deps.Merger == nil {
		return errors.New("merger is required")
	}
	if o.deps.Markdown == nil {
		return errors.New("markdown writer is required")
	}
	if o.deps.JSON == nil {
		return errors.New("json writer is required")
	}
	if o.deps.SARIF == nil {
		return errors.New("sarif writer is required")
	}
	if o.deps.PromptBuilder == nil {
		return errors.New("prompt builder is required")
	}
	if o.deps.SeedGenerator == nil {
		return errors.New("seed generator is required")
	}
	// Redactor is optional
	// Store is optional
	return nil
}

// ReviewBranch executes a multi-provider review for a Git branch diff.
func (o *Orchestrator) ReviewBranch(ctx context.Context, req BranchRequest) (Result, error) {
	if err := o.validateDependencies(); err != nil {
		return Result{}, err
	}

	if err := validateRequest(req); err != nil {
		return Result{}, err
	}

	diff, err := o.deps.Git.GetCumulativeDiff(ctx, req.BaseRef, req.TargetRef, req.IncludeUncommitted)
	if err != nil {
		return Result{}, err
	}

	// Create run record if store is available
	var runID string
	if o.deps.Store != nil {
		now := time.Now()
		runID = generateRunID(now, req.BaseRef, req.TargetRef)
		run := StoreRun{
			RunID:      runID,
			Timestamp:  now,
			Scope:      fmt.Sprintf("%s..%s", req.BaseRef, req.TargetRef),
			ConfigHash: calculateConfigHash(req),
			TotalCost:  0.0, // Cost tracking not yet implemented
			BaseRef:    req.BaseRef,
			TargetRef:  req.TargetRef,
			Repository: req.Repository,
		}

		if err := o.deps.Store.CreateRun(ctx, run); err != nil {
			// Log warning but continue - store failures shouldn't break reviews
			// In production, this would use proper logging
			fmt.Printf("warning: failed to create run record: %v\n", err)
		}
	}

	seed := o.deps.SeedGenerator(req.BaseRef, req.TargetRef)
	providerReq, err := o.deps.PromptBuilder(diff, req)
	if err != nil {
		return Result{}, err
	}
	if providerReq.Seed == 0 {
		providerReq.Seed = seed
	}

	// Apply redaction if redactor is available
	if o.deps.Redactor != nil {
		redactedPrompt, err := o.deps.Redactor.Redact(providerReq.Prompt)
		if err != nil {
			return Result{}, fmt.Errorf("redaction failed: %w", err)
		}
		providerReq.Prompt = redactedPrompt
	}

	var wg sync.WaitGroup
	resultsChan := make(chan struct {
		review    domain.Review
		path      string
		jsonPath  string
		sarifPath string
		err       error
	}, len(o.deps.Providers))

	for name, provider := range o.deps.Providers {
		wg.Add(1)
		go func(name string, provider Provider, runID string) {
			defer func() {
				if r := recover(); r != nil {
					resultsChan <- struct {
						review    domain.Review
						path      string
						jsonPath  string
						sarifPath string
						err       error
					}{err: fmt.Errorf("provider %s panicked: %v", name, r)}
				}
				wg.Done()
			}()

			review, err := provider.Review(ctx, providerReq)
			if err != nil {
				resultsChan <- struct {
					review    domain.Review
					path      string
					jsonPath  string
					sarifPath string
					err       error
				}{err: fmt.Errorf("provider %s failed: %w", name, err)}
				return
			}

			markdownPath, err := o.deps.Markdown.Write(ctx, domain.MarkdownArtifact{
				OutputDir:    req.OutputDir,
				Repository:   req.Repository,
				BaseRef:      req.BaseRef,
				TargetRef:    req.TargetRef,
				Diff:         diff,
				Review:       review,
				ProviderName: review.ProviderName,
			})
			if err != nil {
				resultsChan <- struct {
					review    domain.Review
					path      string
					jsonPath  string
					sarifPath string
					err       error
				}{err: fmt.Errorf("markdown write failed for %s: %w", name, err)}
				return
			}

			jsonPath, err := o.deps.JSON.Write(ctx, domain.JSONArtifact{
				OutputDir:    req.OutputDir,
				Repository:   req.Repository,
				BaseRef:      req.BaseRef,
				TargetRef:    req.TargetRef,
				Review:       review,
				ProviderName: review.ProviderName,
			})
			if err != nil {
				resultsChan <- struct {
					review    domain.Review
					path      string
					jsonPath  string
					sarifPath string
					err       error
				}{err: fmt.Errorf("json write failed for %s: %w", name, err)}
				return
			}

			sarifPath, err := o.deps.SARIF.Write(ctx, SARIFArtifact{
				OutputDir:    req.OutputDir,
				Repository:   req.Repository,
				BaseRef:      req.BaseRef,
				TargetRef:    req.TargetRef,
				Review:       review,
				ProviderName: review.ProviderName,
			})
			if err != nil {
				resultsChan <- struct {
					review    domain.Review
					path      string
					jsonPath  string
					sarifPath string
					err       error
				}{err: fmt.Errorf("sarif write failed for %s: %w", name, err)}
				return
			}

			// Save review to store if available
			if runID != "" {
				if err := o.SaveReviewToStore(ctx, runID, review); err != nil {
					// Log warning but continue
					fmt.Printf("warning: failed to save review to store: %v\n", err)
				}
			}

			resultsChan <- struct {
				review    domain.Review
				path      string
				jsonPath  string
				sarifPath string
				err       error
			}{review: review, path: markdownPath, jsonPath: jsonPath, sarifPath: sarifPath}
		}(name, provider, runID)
	}

	wg.Wait()
	close(resultsChan)

	var reviews []domain.Review
	markdownPaths := make(map[string]string)
	jsonPaths := make(map[string]string)
	sarifPaths := make(map[string]string)
	var errs []error

	for res := range resultsChan {
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			reviews = append(reviews, res.review)
			markdownPaths[res.review.ProviderName] = res.path
			jsonPaths[res.review.ProviderName] = res.jsonPath
			sarifPaths[res.review.ProviderName] = res.sarifPath
		}
	}

	if len(errs) > 0 {
		// Aggregate all errors into a single error message
		var errMsgs []string
		for _, err := range errs {
			errMsgs = append(errMsgs, err.Error())
		}
		return Result{}, fmt.Errorf("%d provider(s) failed: %s", len(errs), strings.Join(errMsgs, "; "))
	}

	mergedReview := o.deps.Merger.Merge(reviews)

	mergedMarkdownPath, err := o.deps.Markdown.Write(ctx, domain.MarkdownArtifact{
		OutputDir:    req.OutputDir,
		Repository:   req.Repository,
		BaseRef:      req.BaseRef,
		TargetRef:    req.TargetRef,
		Diff:         diff,
		Review:       mergedReview,
		ProviderName: mergedReview.ProviderName,
	})
	if err != nil {
		return Result{}, fmt.Errorf("markdown write failed for merged review: %w", err)
	}

	mergedJSONPath, err := o.deps.JSON.Write(ctx, domain.JSONArtifact{
		OutputDir:    req.OutputDir,
		Repository:   req.Repository,
		BaseRef:      req.BaseRef,
		TargetRef:    req.TargetRef,
		Review:       mergedReview,
		ProviderName: mergedReview.ProviderName,
	})
	if err != nil {
		return Result{}, fmt.Errorf("json write failed for merged review: %w", err)
	}

	mergedSARIFPath, err := o.deps.SARIF.Write(ctx, SARIFArtifact{
		OutputDir:    req.OutputDir,
		Repository:   req.Repository,
		BaseRef:      req.BaseRef,
		TargetRef:    req.TargetRef,
		Review:       mergedReview,
		ProviderName: mergedReview.ProviderName,
	})
	if err != nil {
		return Result{}, fmt.Errorf("sarif write failed for merged review: %w", err)
	}

	// Save merged review to store if available
	if runID != "" {
		if err := o.SaveReviewToStore(ctx, runID, mergedReview); err != nil {
			// Log warning but continue
			fmt.Printf("warning: failed to save merged review to store: %v\n", err)
		}
	}

	markdownPaths["merged"] = mergedMarkdownPath
	jsonPaths["merged"] = mergedJSONPath
	sarifPaths["merged"] = mergedSARIFPath

	return Result{
		MarkdownPaths: markdownPaths,
		JSONPaths:     jsonPaths,
		SARIFPaths:    sarifPaths,
		Reviews:       append(reviews, mergedReview),
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
