package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

// ErrVersionRequested indicates the user requested the CLI version and no further work should be done.
var ErrVersionRequested = errors.New("version requested")

// BranchReviewer defines the dependency required to run the branch command.
type BranchReviewer interface {
	ReviewBranch(ctx context.Context, req review.BranchRequest) (review.Result, error)
	CurrentBranch(ctx context.Context) (string, error)
}

// Arguments encapsulates IO writers injected from the host process.
type Arguments struct {
	OutWriter io.Writer
	ErrWriter io.Writer
}

// DefaultReviewActions holds default review action configuration from config.
type DefaultReviewActions struct {
	OnCritical string
	OnHigh     string
	OnMedium   string
	OnLow      string
	OnClean    string
}

// Dependencies captures the collaborators for the CLI.
type Dependencies struct {
	BranchReviewer       BranchReviewer
	Args                 Arguments
	DefaultOutput        string
	DefaultRepo          string
	DefaultInstructions  string // From config review.instructions
	DefaultReviewActions DefaultReviewActions
	DefaultBotUsername   string // Bot username for auto-dismissing stale reviews
	Version              string
}

// NewRootCommand constructs the root Cobra command.
func NewRootCommand(deps Dependencies) *cobra.Command {
	versionString := deps.Version
	if versionString == "" {
		versionString = "v0.0.0"
	}

	root := &cobra.Command{
		Use:   "cr",
		Short: "Multi-LLM code review CLI",
	}
	root.SilenceUsage = true
	root.SilenceErrors = true

	outWriter := deps.Args.OutWriter
	if outWriter == nil {
		outWriter = os.Stdout
	}
	errWriter := deps.Args.ErrWriter
	if errWriter == nil {
		errWriter = os.Stderr
	}
	root.SetOut(outWriter)
	root.SetErr(errWriter)

	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Run a code review",
	}
	reviewCmd.AddCommand(branchCommand(deps.BranchReviewer, deps.DefaultOutput, deps.DefaultRepo, deps.DefaultInstructions, deps.DefaultReviewActions, deps.DefaultBotUsername))
	root.AddCommand(reviewCmd)

	var showVersion bool
	root.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Show version and exit")
	versionHandler := func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Fprintln(cmd.OutOrStdout(), versionString)
			return ErrVersionRequested
		}
		return nil
	}
	root.PersistentPreRunE = versionHandler
	root.PreRunE = versionHandler
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if err := versionHandler(cmd, args); err != nil {
			return err
		}
		return cmd.Help()
	}

	return root
}

func branchCommand(branchReviewer BranchReviewer, defaultOutput, defaultRepo, defaultInstructions string, defaultActions DefaultReviewActions, defaultBotUsername string) *cobra.Command {
	var baseRef string
	var targetRef string
	var outputDir string
	var repository string
	var includeUncommitted bool
	var detectTarget bool
	var customInstructions string
	var contextFiles []string
	var interactive bool
	var noPlanning bool
	var planOnly bool
	var noArchitecture bool
	var noAutoContext bool

	// GitHub integration flags
	var postGitHubReview bool
	var githubOwner string
	var githubRepo string
	var prNumber int
	var commitSHA string

	// Review action override flags
	var actionCritical string
	var actionHigh string
	var actionMedium string
	var actionLow string
	var actionClean string

	cmd := &cobra.Command{
		Use:   "branch [target]",
		Short: "Review a branch against a base reference",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				targetRef = args[0]
			}
			ctx := cmd.Context()
			if targetRef == "" && detectTarget {
				resolved, err := branchReviewer.CurrentBranch(ctx)
				if err != nil {
					return fmt.Errorf("detect target branch: %w", err)
				}
				targetRef = resolved
			}
			if targetRef == "" {
				return fmt.Errorf("target branch not specified; pass as an argument, use --target, or disable --detect-target")
			}

			// Use config instructions as fallback if --instructions flag not provided
			if customInstructions == "" {
				customInstructions = defaultInstructions
			}

			// Validate GitHub flags if posting to GitHub
			if postGitHubReview {
				if githubOwner == "" || githubRepo == "" {
					return fmt.Errorf("--github-owner and --github-repo are required when --post-github-review is set")
				}
				if prNumber <= 0 {
					return fmt.Errorf("--pr-number must be a positive integer when --post-github-review is set")
				}
				if commitSHA == "" {
					return fmt.Errorf("--commit-sha is required when --post-github-review is set")
				}
			}

			// Resolve review actions: CLI flags override defaults from config
			resolvedActionCritical := resolveAction(actionCritical, defaultActions.OnCritical)
			resolvedActionHigh := resolveAction(actionHigh, defaultActions.OnHigh)
			resolvedActionMedium := resolveAction(actionMedium, defaultActions.OnMedium)
			resolvedActionLow := resolveAction(actionLow, defaultActions.OnLow)
			resolvedActionClean := resolveAction(actionClean, defaultActions.OnClean)

			// Resolve bot username for auto-dismiss feature
			// "none" (case-insensitive) explicitly disables auto-dismiss; empty uses default
			resolvedBotUsername := strings.TrimSpace(defaultBotUsername)
			if resolvedBotUsername == "" {
				resolvedBotUsername = "github-actions[bot]"
			} else if strings.EqualFold(resolvedBotUsername, "none") {
				// Explicit opt-out: pass empty to poster (which skips dismissal)
				resolvedBotUsername = ""
			}

			_, err := branchReviewer.ReviewBranch(ctx, review.BranchRequest{
				BaseRef:            baseRef,
				TargetRef:          targetRef,
				OutputDir:          outputDir,
				Repository:         repository,
				IncludeUncommitted: includeUncommitted,
				CustomInstructions: customInstructions,
				ContextFiles:       contextFiles,
				NoArchitecture:     noArchitecture,
				NoAutoContext:      noAutoContext,
				Interactive:        interactive,
				PostToGitHub:       postGitHubReview,
				GitHubOwner:        githubOwner,
				GitHubRepo:         githubRepo,
				PRNumber:           prNumber,
				CommitSHA:          commitSHA,
				ActionOnCritical:   resolvedActionCritical,
				ActionOnHigh:       resolvedActionHigh,
				ActionOnMedium:     resolvedActionMedium,
				ActionOnLow:        resolvedActionLow,
				ActionOnClean:      resolvedActionClean,
				BotUsername:        resolvedBotUsername,
			})
			return err
		},
	}

	cmd.Flags().StringVar(&baseRef, "base", "main", "Base reference to diff against")
	cmd.Flags().StringVar(&targetRef, "target", "", "Target branch to review (overrides positional)")
	if defaultOutput == "" {
		defaultOutput = "out"
	}
	cmd.Flags().StringVar(&outputDir, "output", defaultOutput, "Directory to write review artifacts")
	cmd.Flags().StringVar(&repository, "repository", defaultRepo, "Optional repository name override")
	cmd.Flags().BoolVar(&includeUncommitted, "include-uncommitted", false, "Include uncommitted changes on the target branch")
	cmd.Flags().BoolVar(&detectTarget, "detect-target", true, "Automatically detect the checked out branch when no target is provided")
	cmd.Flags().StringVar(&customInstructions, "instructions", "", "Custom instructions to include in review prompts")
	cmd.Flags().StringSliceVar(&contextFiles, "context", []string{}, "Additional context files to include in prompts")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "Enable interactive planning mode (asks clarifying questions before review)")
	cmd.Flags().BoolVar(&noPlanning, "no-planning", false, "Skip planning in interactive mode")
	_ = cmd.Flags().MarkHidden("no-planning") // Not yet implemented
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Dry-run showing what context would be gathered")
	_ = cmd.Flags().MarkHidden("plan-only") // Not yet implemented
	cmd.Flags().BoolVar(&noArchitecture, "no-architecture", false, "Skip loading ARCHITECTURE.md")
	cmd.Flags().BoolVar(&noAutoContext, "no-auto-context", false, "Disable automatic context gathering (design docs, relevant docs)")

	// GitHub integration flags
	cmd.Flags().BoolVar(&postGitHubReview, "post-github-review", false, "Post review as GitHub PR review with inline comments")
	cmd.Flags().StringVar(&githubOwner, "github-owner", "", "GitHub repository owner (required with --post-github-review)")
	cmd.Flags().StringVar(&githubRepo, "github-repo", "", "GitHub repository name (required with --post-github-review)")
	cmd.Flags().IntVar(&prNumber, "pr-number", 0, "Pull request number (required with --post-github-review)")
	cmd.Flags().StringVar(&commitSHA, "commit-sha", "", "Head commit SHA (required with --post-github-review)")

	// Review action configuration flags (override config file values)
	cmd.Flags().StringVar(&actionCritical, "action-critical", "", "Review action for critical severity (approve, comment, request_changes)")
	cmd.Flags().StringVar(&actionHigh, "action-high", "", "Review action for high severity (approve, comment, request_changes)")
	cmd.Flags().StringVar(&actionMedium, "action-medium", "", "Review action for medium severity (approve, comment, request_changes)")
	cmd.Flags().StringVar(&actionLow, "action-low", "", "Review action for low severity (approve, comment, request_changes)")
	cmd.Flags().StringVar(&actionClean, "action-clean", "", "Review action when no findings (approve, comment, request_changes)")

	return cmd
}

// resolveAction returns the override value if non-empty, otherwise the default.
func resolveAction(override, defaultValue string) string {
	if override != "" {
		return override
	}
	return defaultValue
}
