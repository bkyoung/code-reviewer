package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bkyoung/code-reviewer/internal/adapter/cli"
	"github.com/bkyoung/code-reviewer/internal/adapter/git"
	githubadapter "github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/adapter/llm/anthropic"
	"github.com/bkyoung/code-reviewer/internal/adapter/llm/gemini"
	llmhttp "github.com/bkyoung/code-reviewer/internal/adapter/llm/http"
	"github.com/bkyoung/code-reviewer/internal/adapter/llm/ollama"
	"github.com/bkyoung/code-reviewer/internal/adapter/llm/openai"
	"github.com/bkyoung/code-reviewer/internal/adapter/llm/static"
	"github.com/bkyoung/code-reviewer/internal/adapter/observability"
	"github.com/bkyoung/code-reviewer/internal/adapter/output/json"
	"github.com/bkyoung/code-reviewer/internal/adapter/output/markdown"
	"github.com/bkyoung/code-reviewer/internal/adapter/output/sarif"
	storeAdapter "github.com/bkyoung/code-reviewer/internal/adapter/store"
	"github.com/bkyoung/code-reviewer/internal/adapter/store/sqlite"
	"github.com/bkyoung/code-reviewer/internal/adapter/tracking"
	"github.com/bkyoung/code-reviewer/internal/config"
	"github.com/bkyoung/code-reviewer/internal/determinism"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/redaction"
	usecasegithub "github.com/bkyoung/code-reviewer/internal/usecase/github"
	"github.com/bkyoung/code-reviewer/internal/usecase/merge"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
	"github.com/bkyoung/code-reviewer/internal/version"
)

func main() {
	if err := run(); err != nil {
		// Redact API keys from URLs in error messages before logging
		log.Println(llmhttp.RedactURLSecrets(err.Error()))
		os.Exit(1)
	}
}

func run() error {
	// Create cancellable context with signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: defaultConfigPaths(),
		FileName:    "cr",
		EnvPrefix:   "CR",
	})
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	repoDir := cfg.Git.RepositoryDir
	if repoDir == "" {
		repoDir = "."
	}

	repoName := repositoryName(repoDir)
	gitEngine := git.NewEngine(repoDir)

	// Timestamp function for deterministic output file naming
	nowFunc := func() string {
		return time.Now().UTC().Format("20060102T150405Z")
	}

	markdownWriter := markdown.NewWriter(nowFunc)
	jsonWriter := json.NewWriter(nowFunc)
	sarifWriter := sarif.NewWriter(nowFunc)

	// Build observability components
	obs := buildObservability(cfg.Observability)

	// Create review logger adapter if logging is enabled
	var reviewLogger review.Logger
	if obs.logger != nil {
		reviewLogger = observability.NewReviewLogger(obs.logger)
	}

	providers := buildProviders(cfg.Providers, cfg.HTTP, obs)

	// Initialize store if enabled
	var reviewStore review.Store
	if cfg.Store.Enabled {
		// Create store directory if it doesn't exist
		storeDir := filepath.Dir(cfg.Store.Path)
		if err := os.MkdirAll(storeDir, 0755); err != nil {
			log.Printf("warning: failed to create store directory: %v", err)
		} else {
			// Initialize SQLite store
			sqliteStore, err := sqlite.NewStore(cfg.Store.Path)
			if err != nil {
				log.Printf("warning: failed to initialize store: %v", err)
			} else {
				// Wrap in adapter bridge
				reviewStore = storeAdapter.NewBridge(sqliteStore)
				// Ensure store is closed on exit
				defer reviewStore.Close()
			}
		}
	}

	// Use intelligent merger for better finding aggregation
	// Note: Pass nil for store for now - precision priors will use defaults
	// TODO: Wire up store adapter when precision prior tracking is needed
	merger := merge.NewIntelligentMerger(nil)

	// Wire up LLM-based summary synthesis (Phase 3.5)
	// Use OpenAI gpt-4o-mini for synthesis (cheap, fast, good at summarization)
	// TODO: Make this configurable via merge.useLLM, merge.provider, merge.model config fields
	if synthProvider, ok := providers["openai"]; ok {
		// Wrap the provider to adapt review.Provider to merge.ReviewProvider
		wrapped := &providerWrapper{provider: synthProvider}
		synthAdapter := merge.NewSynthesisAdapter(wrapped)
		merger.WithSynthesisProvider(synthAdapter)
	}

	// Use enhanced prompt builder for richer context
	promptBuilder := review.NewEnhancedPromptBuilder()

	// Instantiate redaction engine if enabled
	var redactor review.Redactor
	if cfg.Redaction.Enabled {
		redactor = redaction.NewEngine()
	}

	// Create planning agent if configured and enabled
	//
	// Planning agent workflow:
	// 1. If planning.model is specified, create a dedicated provider instance for that model
	//    (e.g., use gpt-4o-mini for planning while using o3 for reviews)
	// 2. If no planning.model specified, reuse the existing provider from the providers map
	//    (maintains backward compatibility with simpler configurations)
	// 3. If provider creation fails (missing API key, etc.), planning is disabled with a warning
	//    and code review continues without the planning phase
	var planningAgent *review.PlanningAgent
	if cfg.Planning.Enabled && cfg.Planning.Provider != "" {
		planningProvider := createPlanningProvider(&cfg, providers, obs)

		if planningProvider != nil {
			// Parse timeout (default to 30s)
			timeout := 30 * time.Second
			if cfg.Planning.Timeout != "" {
				if parsed, err := time.ParseDuration(cfg.Planning.Timeout); err == nil {
					timeout = parsed
				} else {
					log.Printf("warning: invalid planning timeout %q, using default 30s", cfg.Planning.Timeout)
				}
			}

			// Max questions (default to 5)
			maxQuestions := cfg.Planning.MaxQuestions
			if maxQuestions == 0 {
				maxQuestions = 5
			}

			planningAgent = review.NewPlanningAgent(
				planningProvider,
				review.PlanningConfig{
					MaxQuestions: maxQuestions,
					Timeout:      timeout,
				},
				os.Stdin,
				os.Stdout,
			)
		}
	}

	// Create GitHub poster and tracking store if token is available
	var githubPoster review.GitHubPoster
	var trackingStore review.TrackingStore
	if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		githubClient := githubadapter.NewClient(githubToken)
		reviewPoster := usecasegithub.NewReviewPoster(githubClient)
		githubPoster = &githubPosterAdapter{poster: reviewPoster}

		// Enable finding deduplication and incremental reviews
		trackingStore = tracking.NewGitHubStore(githubToken)
	}

	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git:           gitEngine,
		Providers:     providers,
		Merger:        merger,
		Markdown:      markdownWriter,
		JSON:          jsonWriter,
		SARIF:         sarifWriter,
		Redactor:      redactor,
		SeedGenerator: determinism.GenerateSeed,
		PromptBuilder: promptBuilder.Build,
		Store:         reviewStore,
		Logger:        reviewLogger,
		PlanningAgent: planningAgent,
		RepoDir:       repoDir,
		GitHubPoster:  githubPoster,
		TrackingStore: trackingStore,
	})

	root := cli.NewRootCommand(cli.Dependencies{
		BranchReviewer:      orchestrator,
		DefaultOutput:       cfg.Output.Directory,
		DefaultRepo:         repoName,
		DefaultInstructions: cfg.Review.Instructions,
		DefaultReviewActions: cli.DefaultReviewActions{
			OnCritical:    cfg.Review.Actions.OnCritical,
			OnHigh:        cfg.Review.Actions.OnHigh,
			OnMedium:      cfg.Review.Actions.OnMedium,
			OnLow:         cfg.Review.Actions.OnLow,
			OnClean:       cfg.Review.Actions.OnClean,
			OnNonBlocking: cfg.Review.Actions.OnNonBlocking,
		},
		DefaultBotUsername: cfg.Review.BotUsername,
		Version:            version.Value(),
	})

	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, cli.ErrVersionRequested) {
			return nil
		}
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

func repositoryName(repoDir string) string {
	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return "unknown"
	}
	return filepath.Base(abs)
}

func defaultConfigPaths() []string {
	paths := []string{"."}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "cr"))
	}
	return paths
}

// observabilityComponents holds shared observability instances
type observabilityComponents struct {
	logger  llmhttp.Logger
	metrics llmhttp.Metrics
	pricing llmhttp.Pricing
}

// buildObservability creates observability components based on configuration
func buildObservability(cfg config.ObservabilityConfig) observabilityComponents {
	var logger llmhttp.Logger
	var metrics llmhttp.Metrics
	var pricing llmhttp.Pricing

	// Create logger if enabled
	if cfg.Logging.Enabled {
		logLevel := llmhttp.LogLevelInfo
		switch cfg.Logging.Level {
		case "debug":
			logLevel = llmhttp.LogLevelDebug
		case "error":
			logLevel = llmhttp.LogLevelError
		}

		logFormat := llmhttp.LogFormatHuman
		if cfg.Logging.Format == "json" {
			logFormat = llmhttp.LogFormatJSON
		}

		logger = llmhttp.NewDefaultLogger(logLevel, logFormat, cfg.Logging.RedactAPIKeys)
	}

	// Create metrics tracker if enabled
	if cfg.Metrics.Enabled {
		metrics = llmhttp.NewDefaultMetrics()
	}

	// Always create pricing calculator (used for cost tracking)
	pricing = llmhttp.NewDefaultPricing()

	return observabilityComponents{
		logger:  logger,
		metrics: metrics,
		pricing: pricing,
	}
}

// createPlanningProvider creates a dedicated provider instance for the planning agent.
// If a specific planning model is configured, it creates a new provider instance for that model.
// Otherwise, it reuses the existing provider from the providers map.
//
// This allows using a cheaper/faster model for planning (e.g., gpt-4o-mini) while using
// more powerful models for the actual code review.
//
// Returns nil if the provider cannot be created (missing config, API key, etc.).
func createPlanningProvider(cfg *config.Config, providers map[string]review.Provider, obs observabilityComponents) review.Provider {
	providerName := cfg.Planning.Provider
	model := cfg.Planning.Model

	// If a specific planning model is configured, create a dedicated provider instance
	if model != "" {
		// Get the provider config for API key and other settings
		providerCfg, ok := cfg.Providers[providerName]
		if !ok {
			log.Printf("warning: planning provider %q not configured in providers section, planning disabled. Add a '%s' provider configuration to enable planning.", providerName, providerName)
			return nil
		}

		// Create provider based on type
		switch providerName {
		case "openai":
			if providerCfg.APIKey == "" {
				log.Printf("warning: planning provider %q missing API key (set OPENAI_API_KEY or providers.openai.apiKey), planning disabled", providerName)
				return nil
			}
			client := openai.NewHTTPClient(providerCfg.APIKey, model, providerCfg, cfg.HTTP)
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			return openai.NewProvider(model, client)

		case "anthropic":
			if providerCfg.APIKey == "" {
				log.Printf("warning: planning provider %q missing API key (set ANTHROPIC_API_KEY or providers.anthropic.apiKey), planning disabled", providerName)
				return nil
			}
			client := anthropic.NewHTTPClient(providerCfg.APIKey, model, providerCfg, cfg.HTTP)
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			return anthropic.NewProvider(model, client)

		case "gemini":
			if providerCfg.APIKey == "" {
				log.Printf("warning: planning provider %q missing API key (set GEMINI_API_KEY or providers.gemini.apiKey), planning disabled", providerName)
				return nil
			}
			client := gemini.NewHTTPClient(providerCfg.APIKey, model, providerCfg, cfg.HTTP)
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			return gemini.NewProvider(model, client)

		case "ollama":
			// Ollama doesn't require API key, uses host instead
			host := os.Getenv("OLLAMA_HOST")
			if host == "" {
				host = "http://localhost:11434"
			}
			client := ollama.NewHTTPClient(host, model, providerCfg, cfg.HTTP)
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			return ollama.NewProvider(model, client)

		default:
			log.Printf("warning: unsupported planning provider %q, planning disabled. Supported providers: openai, anthropic, gemini, ollama", providerName)
			return nil
		}
	}

	// Reuse existing provider if no specific model configured
	planningProvider, ok := providers[providerName]
	if !ok {
		log.Printf("warning: planning provider %q not found in enabled providers, planning disabled. Enable the provider in your configuration or set planning.model to use a dedicated model.", providerName)
		return nil
	}
	return planningProvider
}

func buildProviders(providersConfig map[string]config.ProviderConfig, httpConfig config.HTTPConfig, obs observabilityComponents) map[string]review.Provider {
	providers := make(map[string]review.Provider)

	// OpenAI provider
	if cfg, ok := providersConfig["openai"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "gpt-4o-mini"
		}
		// Use real HTTP client if API key is provided
		apiKey := cfg.APIKey
		if apiKey == "" {
			// Fallback to static client if no API key
			log.Println("OpenAI: No API key provided, using static client")
			providers["openai"] = openai.NewProvider(model, openai.NewStaticClient())
		} else {
			client := openai.NewHTTPClient(apiKey, model, cfg, httpConfig)
			// Wire up observability
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			providers["openai"] = openai.NewProvider(model, client)
		}
	}

	// Anthropic/Claude provider
	if cfg, ok := providersConfig["anthropic"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "claude-3-5-sonnet-20241022"
		}
		// Use real HTTP client if API key is provided
		apiKey := cfg.APIKey
		if apiKey == "" {
			log.Println("Anthropic: No API key provided, skipping provider")
		} else {
			client := anthropic.NewHTTPClient(apiKey, model, cfg, httpConfig)
			// Wire up observability
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			providers["anthropic"] = anthropic.NewProvider(model, client)
		}
	}

	// Google Gemini provider
	if cfg, ok := providersConfig["gemini"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "gemini-1.5-pro"
		}
		// Use real HTTP client if API key is provided
		apiKey := cfg.APIKey
		if apiKey == "" {
			log.Println("Gemini: No API key provided, skipping provider")
		} else {
			client := gemini.NewHTTPClient(apiKey, model, cfg, httpConfig)
			// Wire up observability
			if obs.logger != nil {
				client.SetLogger(obs.logger)
			}
			if obs.metrics != nil {
				client.SetMetrics(obs.metrics)
			}
			if obs.pricing != nil {
				client.SetPricing(obs.pricing)
			}
			providers["gemini"] = gemini.NewProvider(model, client)
		}
	}

	// Ollama provider (local LLM)
	if cfg, ok := providersConfig["ollama"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "codellama"
		}
		// Use configured host or default to localhost
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://localhost:11434"
		}
		client := ollama.NewHTTPClient(host, model, cfg, httpConfig)
		// Wire up observability
		if obs.logger != nil {
			client.SetLogger(obs.logger)
		}
		if obs.metrics != nil {
			client.SetMetrics(obs.metrics)
		}
		if obs.pricing != nil {
			client.SetPricing(obs.pricing)
		}
		providers["ollama"] = ollama.NewProvider(model, client)
	}

	// Static provider (for testing)
	if cfg, ok := providersConfig["static"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "static-model"
		}
		providers["static"] = static.NewProvider(model)
	}

	return providers
}

// providerWrapper adapts review.Provider to merge.ReviewProvider.
// This is needed because the types are structurally identical but defined in different packages.
type providerWrapper struct {
	provider review.Provider
}

func (w *providerWrapper) Review(ctx context.Context, req merge.ProviderRequest) (domain.Review, error) {
	// Convert merge.ProviderRequest to review.ProviderRequest
	reviewReq := review.ProviderRequest{
		Prompt:  req.Prompt,
		Seed:    req.Seed,
		MaxSize: req.MaxSize,
	}
	return w.provider.Review(ctx, reviewReq)
}

// Compile-time interface compliance checks
var _ review.GitEngine = (*git.Engine)(nil)
var _ review.Provider = (*openai.Provider)(nil)
var _ review.Provider = (*anthropic.Provider)(nil)
var _ review.Provider = (*gemini.Provider)(nil)
var _ review.Provider = (*ollama.Provider)(nil)
var _ review.Provider = (*static.Provider)(nil)
var _ review.Merger = (*merge.Service)(nil)
var _ review.MarkdownWriter = (*markdown.Writer)(nil)
var _ review.JSONWriter = (*json.Writer)(nil)
var _ review.SARIFWriter = (*sarif.Writer)(nil)
var _ review.Redactor = (*redaction.Engine)(nil)
var _ review.GitHubPoster = (*githubPosterAdapter)(nil)

// githubPosterAdapter bridges review.GitHubPoster to the underlying GitHub client.
// It handles diff position calculation and maps between usecase types.
type githubPosterAdapter struct {
	poster *usecasegithub.ReviewPoster
}

// PostReview implements review.GitHubPoster.
func (a *githubPosterAdapter) PostReview(ctx context.Context, req review.GitHubPostRequest) (*review.GitHubPostResult, error) {
	// Map findings to positioned findings with diff positions
	positionedFindings := githubadapter.MapFindings(req.Review.Findings, req.Diff)

	// Build review actions config for determining attention severities
	reviewActions := githubadapter.ReviewActions{
		OnCritical:    req.ActionOnCritical,
		OnHigh:        req.ActionOnHigh,
		OnMedium:      req.ActionOnMedium,
		OnLow:         req.ActionOnLow,
		OnClean:       req.ActionOnClean,
		OnNonBlocking: req.ActionOnNonBlocking,
	}

	// Build programmatic summary (replaces LLM-generated summary)
	programmaticSummary := githubadapter.BuildProgrammaticSummary(positionedFindings, req.Diff, reviewActions)

	// Build summary appendix for edge cases (out-of-diff findings, binary files, renames)
	appendix := githubadapter.BuildSummaryAppendix(positionedFindings, req.Diff)

	// Combine programmatic summary with appendix
	finalSummary := githubadapter.AppendSections(programmaticSummary, appendix)

	// Create review with programmatic summary
	enhancedReview := domain.Review{
		ProviderName: req.Review.ProviderName,
		ModelName:    req.Review.ModelName,
		Summary:      finalSummary,
		Findings:     req.Review.Findings,
		Cost:         req.Review.Cost,
	}

	// Build the post request with review action configuration
	postReq := usecasegithub.PostReviewRequest{
		Owner:         req.Owner,
		Repo:          req.Repo,
		PullNumber:    req.PRNumber,
		CommitSHA:     req.CommitSHA,
		Review:        enhancedReview,
		Findings:      positionedFindings,
		ReviewActions: reviewActions,
		BotUsername:   req.BotUsername,
	}

	// Post the review
	result, err := a.poster.PostReview(ctx, postReq)
	if err != nil {
		return nil, err
	}

	return &review.GitHubPostResult{
		ReviewID:        result.ReviewID,
		CommentsPosted:  result.CommentsPosted,
		CommentsSkipped: result.CommentsSkipped,
		HTMLURL:         result.HTMLURL,
	}, nil
}
