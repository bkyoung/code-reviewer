package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/brandon/code-reviewer/internal/adapter/cli"
	"github.com/brandon/code-reviewer/internal/adapter/git"
	"github.com/brandon/code-reviewer/internal/adapter/llm/anthropic"
	"github.com/brandon/code-reviewer/internal/adapter/llm/gemini"
	"github.com/brandon/code-reviewer/internal/adapter/llm/ollama"
	"github.com/brandon/code-reviewer/internal/adapter/llm/openai"
	"github.com/brandon/code-reviewer/internal/adapter/llm/static"
	"github.com/brandon/code-reviewer/internal/adapter/output/json"
	"github.com/brandon/code-reviewer/internal/adapter/output/markdown"
	"github.com/brandon/code-reviewer/internal/adapter/output/sarif"
	storeAdapter "github.com/brandon/code-reviewer/internal/adapter/store"
	"github.com/brandon/code-reviewer/internal/adapter/store/sqlite"
	"github.com/brandon/code-reviewer/internal/config"
	"github.com/brandon/code-reviewer/internal/determinism"
	"github.com/brandon/code-reviewer/internal/redaction"
	"github.com/brandon/code-reviewer/internal/usecase/merge"
	"github.com/brandon/code-reviewer/internal/usecase/review"
	"github.com/brandon/code-reviewer/internal/version"
)

func main() {
	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

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

	providers := buildProviders(cfg.Providers)

	merger := merge.NewService()

	// Instantiate redaction engine if enabled
	var redactor review.Redactor
	if cfg.Redaction.Enabled {
		redactor = redaction.NewEngine()
	}

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

	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git:           gitEngine,
		Providers:     providers,
		Merger:        merger,
		Markdown:      markdownWriter,
		JSON:          jsonWriter,
		SARIF:         sarifWriter,
		Redactor:      redactor,
		SeedGenerator: determinism.GenerateSeed,
		PromptBuilder: review.DefaultPromptBuilder,
		Store:         reviewStore,
	})

	root := cli.NewRootCommand(cli.Dependencies{
		BranchReviewer: orchestrator,
		DefaultOutput:  cfg.Output.Directory,
		DefaultRepo:    repoName,
		Version:        version.Value(),
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

func buildProviders(providersConfig map[string]config.ProviderConfig) map[string]review.Provider {
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
			providers["openai"] = openai.NewProvider(model, openai.NewHTTPClient(apiKey, model))
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
			providers["anthropic"] = anthropic.NewProvider(model, anthropic.NewHTTPClient(apiKey, model))
		}
	}

	// Google Gemini provider
	if cfg, ok := providersConfig["gemini"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "gemini-pro"
		}
		// TODO: Implement real HTTP client for Gemini API
		// For now, using nil client (will need stub implementation)
		providers["gemini"] = gemini.NewProvider(model, nil)
	}

	// Ollama provider (local LLM)
	if cfg, ok := providersConfig["ollama"]; ok && cfg.Enabled {
		model := cfg.Model
		if model == "" {
			model = "llama2"
		}
		// TODO: Implement real HTTP client for Ollama API
		// For now, using nil client (will need stub implementation)
		providers["ollama"] = ollama.NewProvider(model, nil)
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
