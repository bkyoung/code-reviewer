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
	"github.com/brandon/code-reviewer/internal/adapter/llm/openai"
	"github.com/brandon/code-reviewer/internal/adapter/output/markdown"
	"github.com/brandon/code-reviewer/internal/config"
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
	markdownWriter := markdown.NewWriter(func() string {
		return time.Now().UTC().Format("20060102T150405Z")
	})

	providerModel, providerClient := buildProvider(cfg.Providers)
	provider := openai.NewProvider(providerModel, providerClient)

	orchestrator := review.NewOrchestrator(review.OrchestratorDeps{
		Git:           gitEngine,
		Provider:      provider,
		Markdown:      markdownWriter,
		SeedGenerator: deterministicSeed,
		PromptBuilder: review.DefaultPromptBuilder,
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

func deterministicSeed(baseRef, targetRef string) uint64 {
	key := fmt.Sprintf("%s::%s", baseRef, targetRef)
	var hash uint64 = 1469598103934665603 // FNV offset basis
	const prime uint64 = 1099511628211
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= prime
	}
	return hash
}

func buildProvider(providers map[string]config.ProviderConfig) (string, openai.Client) {
	if cfg, ok := providers["openai"]; ok && cfg.Model != "" {
		return cfg.Model, openai.NewStaticClient()
	}
	return "gpt-4o-mini", openai.NewStaticClient()
}

var _ review.GitEngine = (*git.Engine)(nil)
var _ review.Provider = (*openai.Provider)(nil)
var _ review.MarkdownWriter = (*markdown.Writer)(nil)
