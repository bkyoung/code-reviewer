package review

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// GitEngine abstracts git operations for code review.
type GitEngine interface {
	// GetCumulativeDiff returns the diff between two refs (branches or commits).
	GetCumulativeDiff(ctx context.Context, baseRef, targetRef string, includeUncommitted bool) (domain.Diff, error)

	// GetIncrementalDiff returns the diff between two specific commits.
	// Used for incremental reviews where we only want changes since the last reviewed commit.
	GetIncrementalDiff(ctx context.Context, fromCommit, toCommit string) (domain.Diff, error)

	// CommitExists checks if a commit SHA exists in the repository.
	// Used for force-push detection - if the last reviewed commit no longer exists,
	// we fall back to full diff.
	// Returns (false, nil) if the commit genuinely doesn't exist.
	// Returns (false, error) if there was an error checking (e.g., repo access failure).
	CommitExists(ctx context.Context, commitSHA string) (bool, error)

	// CurrentBranch returns the name of the checked-out branch.
	CurrentBranch(ctx context.Context) (string, error)
}

// Provider defines the outbound port for LLM reviews.
type Provider interface {
	Review(ctx context.Context, req ProviderRequest) (domain.Review, error)
}

// Merger defines the outbound port for merging reviews.
type Merger interface {
	Merge(ctx context.Context, reviews []domain.Review) domain.Review
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

// PromptBuilder constructs the provider request payload with project context.
type PromptBuilder func(ctx ProjectContext, diff domain.Diff, req BranchRequest, providerName string) (ProviderRequest, error)

// Redactor defines the outbound port for secret redaction.
type Redactor interface {
	Redact(input string) (string, error)
}

// Store defines the outbound port for persisting review history.
type Store interface {
	CreateRun(ctx context.Context, run StoreRun) error
	UpdateRunCost(ctx context.Context, runID string, totalCost float64) error
	SaveReview(ctx context.Context, review StoreReview) error
	SaveFindings(ctx context.Context, findings []StoreFinding) error
	GetPrecisionPriors(ctx context.Context) (map[string]map[string]StorePrecisionPrior, error)
	Close() error
}

// GitHubPoster defines the outbound port for posting reviews to GitHub PRs.
type GitHubPoster interface {
	PostReview(ctx context.Context, req GitHubPostRequest) (*GitHubPostResult, error)
}

// GitHubPostRequest contains all data needed to post a review to GitHub.
type GitHubPostRequest struct {
	Owner     string
	Repo      string
	PRNumber  int
	CommitSHA string
	Review    domain.Review
	Diff      domain.Diff // For calculating diff positions

	// ReviewActions configures the GitHub review action for each severity level.
	// Empty values use sensible defaults.
	ActionOnCritical    string
	ActionOnHigh        string
	ActionOnMedium      string
	ActionOnLow         string
	ActionOnClean       string
	ActionOnNonBlocking string

	// BotUsername is the bot username for auto-dismissing stale reviews.
	// If set, previous reviews from this user are dismissed AFTER the new
	// review posts successfully. This ensures the PR always has review signal.
	BotUsername string
}

// GitHubPostResult contains the result of posting a review.
type GitHubPostResult struct {
	ReviewID        int64
	CommentsPosted  int
	CommentsSkipped int
	HTMLURL         string
}

// StorePrecisionPrior represents precision tracking for a provider/category combination.
type StorePrecisionPrior struct {
	Provider string
	Category string
	Alpha    float64
	Beta     float64
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
	Store         Store          // Optional: persistence layer for review history
	Logger        Logger         // Optional: structured logging for warnings and info
	PlanningAgent *PlanningAgent // Optional: interactive planning agent (only works in TTY mode)
	RepoDir       string         // Repository directory for context gathering (optional)
	GitHubPoster  GitHubPoster   // Optional: posts review to GitHub PR with inline comments

	// Incremental review support (Epic #53)
	TrackingStore TrackingStore // Optional: tracks reviewed commits for incremental reviews
	DiffComputer  *DiffComputer // Optional: computes incremental vs full diffs (auto-created if nil)
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
	CustomInstructions string   // Optional: custom review instructions
	ContextFiles       []string // Optional: additional context files to include
	NoArchitecture     bool     // Skip loading ARCHITECTURE.md
	NoAutoContext      bool     // Disable automatic context gathering (design docs, relevant docs)
	Interactive        bool     // Enable interactive planning mode (requires TTY)

	// GitHub integration fields (for posting inline review comments)
	PostToGitHub bool   // Enable posting review to GitHub PR
	GitHubOwner  string // Repository owner (user or org)
	GitHubRepo   string // Repository name
	PRNumber     int    // Pull request number
	CommitSHA    string // Head commit SHA for the review

	// Review action configuration (configures GitHub review action per severity)
	// Values: "approve", "comment", "request_changes" (case-insensitive)
	ActionOnCritical    string // Action for critical severity findings
	ActionOnHigh        string // Action for high severity findings
	ActionOnMedium      string // Action for medium severity findings
	ActionOnLow         string // Action for low severity findings
	ActionOnClean       string // Action when no findings in diff
	ActionOnNonBlocking string // Action when findings exist but none block

	// BotUsername is the bot username for auto-dismissing stale reviews.
	// If set, previous reviews from this user are dismissed AFTER the new
	// review posts successfully. This ensures the PR always has review signal.
	// Set to empty string to disable auto-dismiss (use "none" in config).
	// Default: "github-actions[bot]"
	BotUsername string
}

// Result captures the orchestrator outcome.
type Result struct {
	MarkdownPaths map[string]string
	JSONPaths     map[string]string
	SARIFPaths    map[string]string
	Reviews       []domain.Review
	GitHubResult  *GitHubPostResult // Set when PostToGitHub is enabled
}

// Orchestrator implements the core review flow for Phase 1.
type Orchestrator struct {
	deps OrchestratorDeps
}

// NewOrchestrator wires the orchestrator dependencies.
// If DiffComputer is not provided but Git is, it will be auto-created.
func NewOrchestrator(deps OrchestratorDeps) *Orchestrator {
	// Auto-wire DiffComputer if not provided
	if deps.DiffComputer == nil && deps.Git != nil {
		deps.DiffComputer = NewDiffComputer(deps.Git)
	}
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
	if o.deps.DiffComputer == nil {
		return errors.New("diff computer is required (use NewOrchestrator for auto-wiring)")
	}
	// Redactor is optional
	// Store is optional
	// TrackingStore is optional
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

	// Load tracking state for incremental reviews (GitHub mode only)
	var trackingState *TrackingState
	if o.deps.TrackingStore != nil && req.PRNumber > 0 {
		target := ReviewTarget{
			Repository: fmt.Sprintf("%s/%s", req.GitHubOwner, req.GitHubRepo),
			PRNumber:   req.PRNumber,
			Branch:     req.TargetRef,
			BaseSHA:    req.BaseRef,
			HeadSHA:    req.CommitSHA,
		}

		state, err := o.deps.TrackingStore.Load(ctx, target)
		if err != nil {
			// Log warning but continue - tracking failures shouldn't break reviews
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to load tracking state", map[string]interface{}{
					"error":    err.Error(),
					"prNumber": req.PRNumber,
				})
			} else {
				log.Printf("warning: failed to load tracking state: %v\n", err)
			}
		} else {
			trackingState = &state
		}

		// Post "in-progress" tracking comment BEFORE running the review.
		// This ensures the tracking comment appears first in the PR timeline,
		// before any inline comments from the review.
		inProgressState := NewTrackingStateInProgress(target, time.Now())
		// Preserve state from previous tracking to prevent data loss if review crashes
		if trackingState != nil {
			inProgressState.ReviewedCommits = trackingState.ReviewedCommits
			inProgressState.Findings = trackingState.Findings
		}

		if err := o.deps.TrackingStore.Save(ctx, inProgressState); err != nil {
			// Log warning but continue - tracking failures shouldn't break reviews
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to post in-progress tracking comment", map[string]interface{}{
					"error":    err.Error(),
					"prNumber": req.PRNumber,
				})
			} else {
				log.Printf("warning: failed to post in-progress tracking comment: %v\n", err)
			}
		} else {
			if o.deps.Logger != nil {
				o.deps.Logger.LogInfo(ctx, "posted in-progress tracking comment", map[string]interface{}{
					"prNumber": req.PRNumber,
				})
			}
		}
	}

	// Compute diff (incremental if tracking state available, full otherwise)
	// DiffComputer is auto-wired in NewOrchestrator when Git is provided
	diff, err := o.deps.DiffComputer.ComputeDiffForReview(ctx, req, trackingState)
	if err != nil {
		return Result{}, err
	}

	// Gather project context if RepoDir is configured
	projectContext := ProjectContext{}
	if o.deps.RepoDir != "" {
		gatherer := NewContextGatherer(o.deps.RepoDir)

		// Load architecture documentation (unless disabled)
		if !req.NoArchitecture {
			if architecture, err := gatherer.loadFile("ARCHITECTURE.md"); err == nil {
				projectContext.Architecture = architecture
			}
		}

		// Load README and design docs (unless auto-context is disabled)
		if !req.NoAutoContext {
			// Load README
			if readme, err := gatherer.loadFile("README.md"); err == nil {
				projectContext.README = readme
			}

			// Load design documents
			if designDocs, err := gatherer.loadDesignDocs(); err == nil {
				projectContext.DesignDocs = designDocs
			}

			// Detect change types and find relevant docs
			projectContext.ChangeTypes = gatherer.detectChangeTypes(diff)
			projectContext.ChangedPaths = make([]string, 0, len(diff.Files))
			for _, file := range diff.Files {
				projectContext.ChangedPaths = append(projectContext.ChangedPaths, file.Path)
			}

			if relevantDocs, err := gatherer.findRelevantDocs(projectContext.ChangedPaths, projectContext.ChangeTypes); err == nil {
				projectContext.RelevantDocs = relevantDocs
			}
		}

		// Load custom context files from request (always, regardless of flags)
		if len(req.ContextFiles) > 0 {
			contextFiles := make([]string, 0, len(req.ContextFiles))
			for _, file := range req.ContextFiles {
				content, err := gatherer.loadFile(file)
				if err != nil {
					return Result{}, fmt.Errorf("failed to load context file %s: %w", file, err)
				}
				contextFiles = append(contextFiles, fmt.Sprintf("=== %s ===\n%s", file, content))
			}
			projectContext.CustomContextFiles = contextFiles
		}
	}

	// Always set custom instructions from request (even if RepoDir is not configured)
	projectContext.CustomInstructions = req.CustomInstructions

	// Planning Phase: Interactive clarifying questions (optional, only in TTY mode)
	if req.Interactive && IsInteractive() && o.deps.PlanningAgent != nil {
		planningResult, err := o.deps.PlanningAgent.Plan(ctx, projectContext, diff)
		if err != nil {
			// Planning failure shouldn't block the review - log warning and continue
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "planning phase failed", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				log.Printf("warning: planning phase failed: %v\n", err)
			}
		} else {
			// Use enhanced context from planning
			projectContext = planningResult.EnhancedContext
		}
	}

	// Generate run ID for potential store usage
	now := time.Now()
	var runID string
	if o.deps.Store != nil {
		runID = generateRunID(now, req.BaseRef, req.TargetRef)
	}

	seed := o.deps.SeedGenerator(req.BaseRef, req.TargetRef)

	// Create run record BEFORE launching provider goroutines so that reviews can reference it
	if o.deps.Store != nil && runID != "" {
		run := StoreRun{
			RunID:      runID,
			Timestamp:  now,
			Scope:      fmt.Sprintf("%s..%s", req.BaseRef, req.TargetRef),
			ConfigHash: calculateConfigHash(req),
			TotalCost:  0.0, // Will be updated after all reviews complete
			BaseRef:    req.BaseRef,
			TargetRef:  req.TargetRef,
			Repository: req.Repository,
		}

		if err := o.deps.Store.CreateRun(ctx, run); err != nil {
			// Log warning but continue - store failures shouldn't break reviews
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to create run record", map[string]interface{}{
					"runID": runID,
					"error": err.Error(),
				})
			} else {
				log.Printf("warning: failed to create run record: %v\n", err)
			}
		}
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

			// Filter binary files before building prompt (saves tokens, prevents impossible findings)
			textDiff, binaryFiles := FilterBinaryFiles(diff)
			if len(binaryFiles) > 0 {
				log.Printf("[%s] Filtered %d binary file(s) from review", name, len(binaryFiles))
			}

			// Build provider-specific prompt using filtered diff
			providerReq, err := o.deps.PromptBuilder(projectContext, textDiff, req, name)
			if err != nil {
				resultsChan <- struct {
					review    domain.Review
					path      string
					jsonPath  string
					sarifPath string
					err       error
				}{err: fmt.Errorf("prompt building failed for %s: %w", name, err)}
				return
			}
			if providerReq.Seed == 0 {
				providerReq.Seed = seed
			}

			// Apply redaction if redactor is available
			if o.deps.Redactor != nil {
				redactedPrompt, err := o.deps.Redactor.Redact(providerReq.Prompt)
				if err != nil {
					resultsChan <- struct {
						review    domain.Review
						path      string
						jsonPath  string
						sarifPath string
						err       error
					}{err: fmt.Errorf("redaction failed for %s: %w", name, err)}
					return
				}
				providerReq.Prompt = redactedPrompt
			}

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
					if o.deps.Logger != nil {
						o.deps.Logger.LogWarning(ctx, "failed to save review to store", map[string]interface{}{
							"runID":    runID,
							"provider": name,
							"error":    err.Error(),
						})
					} else {
						log.Printf("warning: failed to save review to store: %v\n", err)
					}
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
	var totalCost float64

	for res := range resultsChan {
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			reviews = append(reviews, res.review)
			markdownPaths[res.review.ProviderName] = res.path
			jsonPaths[res.review.ProviderName] = res.jsonPath
			sarifPaths[res.review.ProviderName] = res.sarifPath
			totalCost += res.review.Cost
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

	// Update run record with total cost now that all reviews are complete
	if o.deps.Store != nil && runID != "" {
		if err := o.deps.Store.UpdateRunCost(ctx, runID, totalCost); err != nil {
			// Log warning but continue - store failures shouldn't break reviews
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to update run cost", map[string]interface{}{
					"runID":     runID,
					"totalCost": totalCost,
					"error":     err.Error(),
				})
			} else {
				log.Printf("warning: failed to update run cost: %v\n", err)
			}
		}
	}

	mergedReview := o.deps.Merger.Merge(ctx, reviews)
	mergedReview.Cost = totalCost // Merged review gets total cost from all providers

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
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to save merged review to store", map[string]interface{}{
					"runID":    runID,
					"provider": "merged",
					"error":    err.Error(),
				})
			} else {
				log.Printf("warning: failed to save merged review to store: %v\n", err)
			}
		}
	}

	markdownPaths["merged"] = mergedMarkdownPath
	jsonPaths["merged"] = mergedJSONPath
	sarifPaths["merged"] = mergedSARIFPath

	// Post review to GitHub if enabled
	var githubResult *GitHubPostResult
	var reconciledState *TrackingState // Will hold updated state from reconciliation
	if req.PostToGitHub && o.deps.GitHubPoster != nil {
		// Determine which findings to post
		// If tracking is enabled, apply deduplication to only post NEW findings
		findingsToPost := mergedReview.Findings

		if o.deps.TrackingStore != nil && req.PRNumber > 0 && req.CommitSHA != "" {
			// Single timestamp for all operations in this reconciliation cycle
			reconcileTime := time.Now()

			// Detect first review: nil state OR empty Findings map (handles empty-but-non-nil case)
			isFirstReview := trackingState == nil || len(trackingState.Findings) == 0

			// ReconciliationResult errors are logged inside reconcileAndFilterFindings
			newFindings, updatedState, _ := o.reconcileAndFilterFindings(
				ctx,
				trackingState,
				mergedReview.Findings,
				diff,
				req.CommitSHA,
				reconcileTime,
			)
			findingsToPost = newFindings

			// Store the reconciled state for later persistence
			// For first review (no prior findings), initialize a new state with all findings
			if isFirstReview && len(mergedReview.Findings) > 0 {
				target := ReviewTarget{
					Repository: fmt.Sprintf("%s/%s", req.GitHubOwner, req.GitHubRepo),
					PRNumber:   req.PRNumber,
					Branch:     req.TargetRef,
					BaseSHA:    req.BaseRef,
					HeadSHA:    req.CommitSHA,
				}
				newState := NewTrackingState(target)
				// Add all findings as tracked (first review = all are new)
				var skippedCount int
				for _, f := range mergedReview.Findings {
					tracked, err := domain.NewTrackedFindingFromFinding(f, reconcileTime, req.CommitSHA)
					if err != nil {
						skippedCount++
						if o.deps.Logger != nil {
							o.deps.Logger.LogWarning(ctx, "failed to track finding on first review", map[string]interface{}{
								"file":  f.File,
								"line":  f.LineStart,
								"error": err.Error(),
							})
						}
						continue
					}
					newState.Findings[tracked.Fingerprint] = tracked
				}
				if skippedCount > 0 && o.deps.Logger != nil {
					o.deps.Logger.LogWarning(ctx, "some findings could not be tracked", map[string]interface{}{
						"skipped": skippedCount,
						"total":   len(mergedReview.Findings),
					})
				}
				reconciledState = &newState
			} else if updatedState.Target.Repository != "" {
				// Non-empty Repository indicates reconciliation produced valid state
				reconciledState = &updatedState
			} else if trackingState != nil && len(trackingState.Findings) > 0 {
				// Edge case: had prior findings but reconciliation returned invalid state
				// This shouldn't happen with well-formed state; log for debugging
				if o.deps.Logger != nil {
					o.deps.Logger.LogWarning(ctx, "reconciliation returned invalid state, tracking may be incomplete", map[string]interface{}{
						"priorFindingsCount": len(trackingState.Findings),
						"targetRepository":   updatedState.Target.Repository,
					})
				}
			}

			// TODO(#60): Detect status updates from PR comment replies and reactions
			// Before reconciliation, scan for user replies that indicate status changes
			// (e.g., "acknowledged", "won't fix", "disputed") and update finding statuses
		}

		// Create review with potentially filtered findings for posting
		reviewToPost := domain.Review{
			ProviderName: mergedReview.ProviderName,
			ModelName:    mergedReview.ModelName,
			Summary:      mergedReview.Summary,
			Findings:     findingsToPost, // Only NEW findings get inline comments
			Cost:         mergedReview.Cost,
		}

		result, err := o.deps.GitHubPoster.PostReview(ctx, GitHubPostRequest{
			Owner:               req.GitHubOwner,
			Repo:                req.GitHubRepo,
			PRNumber:            req.PRNumber,
			CommitSHA:           req.CommitSHA,
			Review:              reviewToPost,
			Diff:                diff,
			ActionOnCritical:    req.ActionOnCritical,
			ActionOnHigh:        req.ActionOnHigh,
			ActionOnMedium:      req.ActionOnMedium,
			ActionOnLow:         req.ActionOnLow,
			ActionOnClean:       req.ActionOnClean,
			ActionOnNonBlocking: req.ActionOnNonBlocking,
			BotUsername:         req.BotUsername,
		})
		if err != nil {
			// Log warning but don't fail the review
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to post review to GitHub", map[string]interface{}{
					"owner":    req.GitHubOwner,
					"repo":     req.GitHubRepo,
					"prNumber": req.PRNumber,
					"error":    err.Error(),
				})
			} else {
				log.Printf("warning: failed to post review to GitHub: %v\n", err)
			}
		} else {
			githubResult = result
			if o.deps.Logger != nil {
				o.deps.Logger.LogInfo(ctx, "posted review to GitHub", map[string]interface{}{
					"reviewID":        result.ReviewID,
					"commentsPosted":  result.CommentsPosted,
					"commentsSkipped": result.CommentsSkipped,
					"url":             result.HTMLURL,
				})
			} else {
				log.Printf("Posted review to GitHub: %d comments (%d skipped) - %s\n",
					result.CommentsPosted, result.CommentsSkipped, result.HTMLURL)
			}
		}
	}

	// Save tracking state after successful review
	// Priority: use reconciled state (from deduplication) if available, else original tracking state
	if o.deps.TrackingStore != nil && req.PRNumber > 0 && req.CommitSHA != "" {
		var stateToSave *TrackingState

		if reconciledState != nil {
			// Use the reconciled state which includes new findings
			stateToSave = reconciledState
		} else if trackingState != nil {
			// Fallback to original tracking state (non-reconciliation path)
			stateToSave = trackingState
		}

		if stateToSave != nil {
			// Add the current commit to reviewed commits (deduplicate to prevent growth on re-runs)
			commitAlreadyReviewed := false
			for _, c := range stateToSave.ReviewedCommits {
				if c == req.CommitSHA {
					commitAlreadyReviewed = true
					break
				}
			}
			if !commitAlreadyReviewed {
				stateToSave.ReviewedCommits = append(stateToSave.ReviewedCommits, req.CommitSHA)
			}
			stateToSave.LastUpdated = time.Now()
			// Mark review as completed (updates the "in-progress" comment to show findings)
			stateToSave.ReviewStatus = domain.ReviewStatusCompleted

			// TODO(#61): Use collapsible sections in tracking comment for better UX
			// The tracking comment should use <details> tags to collapse the findings
			// summary and make the PR conversation cleaner

			if err := o.deps.TrackingStore.Save(ctx, *stateToSave); err != nil {
				// Log warning but don't fail the review
				if o.deps.Logger != nil {
					o.deps.Logger.LogWarning(ctx, "failed to save tracking state", map[string]interface{}{
						"error":     err.Error(),
						"prNumber":  req.PRNumber,
						"commitSHA": req.CommitSHA,
					})
				} else {
					log.Printf("warning: failed to save tracking state: %v\n", err)
				}
			} else {
				if o.deps.Logger != nil {
					o.deps.Logger.LogInfo(ctx, "saved tracking state", map[string]interface{}{
						"prNumber":        req.PRNumber,
						"commitSHA":       req.CommitSHA,
						"reviewedCommits": len(stateToSave.ReviewedCommits),
						"totalFindings":   len(stateToSave.Findings),
					})
				}
			}
		} else {
			// No state to save - this can happen if no findings were generated
			if o.deps.Logger != nil {
				o.deps.Logger.LogInfo(ctx, "tracking state save skipped: no state to save", map[string]interface{}{
					"reconciledStateNil": reconciledState == nil,
					"trackingStateNil":   trackingState == nil,
				})
			}
		}
	}

	return Result{
		MarkdownPaths: markdownPaths,
		JSONPaths:     jsonPaths,
		SARIFPaths:    sarifPaths,
		Reviews:       append(reviews, mergedReview),
		GitHubResult:  githubResult,
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

// FilterBinaryFiles separates a diff into text files and binary files.
// The text diff is suitable for sending to LLMs (excludes binary files to save tokens).
// The full diff (with binary files) should be used for GitHub posting so binary
// file changes are visible in the summary.
func FilterBinaryFiles(diff domain.Diff) (textDiff domain.Diff, binaryFiles []domain.FileDiff) {
	textFiles := make([]domain.FileDiff, 0, len(diff.Files))
	binaryFiles = make([]domain.FileDiff, 0)

	for _, f := range diff.Files {
		if f.IsBinary {
			binaryFiles = append(binaryFiles, f)
		} else {
			textFiles = append(textFiles, f)
		}
	}

	textDiff = domain.Diff{
		FromCommitHash: diff.FromCommitHash,
		ToCommitHash:   diff.ToCommitHash,
		Files:          textFiles,
	}

	return textDiff, binaryFiles
}

// reconcileAndFilterFindings applies deduplication logic to findings and returns
// only genuinely new findings for posting, along with the updated tracking state.
//
// This function:
// 1. Extracts changed file paths from the diff
// 2. Calls ReconcileFindings to categorize findings (new, updated, resolved)
// 3. Creates TrackedFindings for genuinely new findings
// 4. Returns the new findings for inline comments and the updated state for persistence
//
// If trackingState is nil (first review), all findings are considered new.
func (o *Orchestrator) reconcileAndFilterFindings(
	ctx context.Context,
	trackingState *TrackingState,
	allFindings []domain.Finding,
	diff domain.Diff,
	commitSHA string,
	timestamp time.Time,
) (newFindings []domain.Finding, updatedState TrackingState, result ReconciliationResult) {
	// Handle first review (no tracking state) - all findings are new
	if trackingState == nil {
		return allFindings, TrackingState{}, ReconciliationResult{New: allFindings}
	}

	// Extract changed file paths from diff for auto-resolve scope
	changedFiles := make([]string, 0, len(diff.Files))
	for _, file := range diff.Files {
		changedFiles = append(changedFiles, file.Path)
	}

	// Reconcile findings (deduplication + auto-resolve)
	updatedState, result = ReconcileFindings(
		*trackingState,
		allFindings,
		changedFiles,
		commitSHA,
		timestamp,
	)

	// Create TrackedFindings for genuinely new findings and add to state
	for _, f := range result.New {
		tracked, err := domain.NewTrackedFindingFromFinding(f, timestamp, commitSHA)
		if err != nil {
			// Log but continue - don't fail entire review for one bad finding
			if o.deps.Logger != nil {
				o.deps.Logger.LogWarning(ctx, "failed to create tracked finding", map[string]interface{}{
					"file":  f.File,
					"line":  f.LineStart,
					"error": err.Error(),
				})
			} else {
				log.Printf("warning: failed to create tracked finding: %v\n", err)
			}
			continue
		}
		updatedState.Findings[tracked.Fingerprint] = tracked
	}

	// Log reconciliation results
	if o.deps.Logger != nil {
		o.deps.Logger.LogInfo(ctx, "reconciled findings", map[string]interface{}{
			"total":              len(allFindings),
			"new":                len(result.New),
			"updated":            len(result.Updated),
			"resolved":           len(result.Resolved),
			"redetectedResolved": len(result.RedetectedResolved),
			"errors":             len(result.Errors),
		})

		// Warn if resolved findings were re-detected
		if len(result.RedetectedResolved) > 0 {
			o.deps.Logger.LogWarning(ctx, "resolved findings re-detected (staying resolved)", map[string]interface{}{
				"count": len(result.RedetectedResolved),
			})
		}

		// Log individual reconciliation errors for debugging
		for _, err := range result.Errors {
			o.deps.Logger.LogWarning(ctx, "reconciliation error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return result.New, updatedState, result
}
