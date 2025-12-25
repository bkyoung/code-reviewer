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
	OnCritical    string
	OnHigh        string
	OnMedium      string
	OnLow         string
	OnClean       string
	OnNonBlocking string
}

// DefaultVerification holds default verification configuration from config.
type DefaultVerification struct {
	Enabled            bool
	Depth              string
	CostCeiling        float64
	ConfidenceDefault  int
	ConfidenceCritical int
	ConfidenceHigh     int
	ConfidenceMedium   int
	ConfidenceLow      int
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
	DefaultVerification  DefaultVerification
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
	reviewCmd.AddCommand(branchCommand(deps.BranchReviewer, deps.DefaultOutput, deps.DefaultRepo, deps.DefaultInstructions, deps.DefaultReviewActions, deps.DefaultBotUsername, deps.DefaultVerification))
	root.AddCommand(reviewCmd)

	var showVersion bool
	root.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Show version and exit")
	versionHandler := func(cmd *cobra.Command, args []string) error {
		if showVersion {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), versionString)
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

func branchCommand(branchReviewer BranchReviewer, defaultOutput, defaultRepo, defaultInstructions string, defaultActions DefaultReviewActions, defaultBotUsername string, defaultVerification DefaultVerification) *cobra.Command {
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
	var actionNonBlocking string

	// Verification flags
	var verify bool
	var noVerify bool
	var verificationDepth string
	var verificationCostCeiling float64
	var confidenceDefault int
	var confidenceCritical int
	var confidenceHigh int
	var confidenceMedium int
	var confidenceLow int

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
			resolvedActionNonBlocking := resolveAction(actionNonBlocking, defaultActions.OnNonBlocking)

			// Resolve bot username for auto-dismiss feature
			// "none" (case-insensitive) explicitly disables auto-dismiss; empty uses default
			resolvedBotUsername := strings.TrimSpace(defaultBotUsername)
			if resolvedBotUsername == "" {
				resolvedBotUsername = "github-actions[bot]"
			} else if strings.EqualFold(resolvedBotUsername, "none") {
				// Explicit opt-out: pass empty to poster (which skips dismissal)
				resolvedBotUsername = ""
			}

			// Resolve verification settings: CLI flags override config defaults
			// --no-verify takes precedence, then --verify, then config
			resolvedVerifyEnabled := resolveVerifyEnabled(cmd, verify, noVerify, defaultVerification.Enabled)
			resolvedDepth := resolveVerificationDepth(cmd, verificationDepth, defaultVerification.Depth)
			resolvedCostCeiling := resolveFloat64(cmd, "verification-cost-ceiling", verificationCostCeiling, defaultVerification.CostCeiling)
			resolvedConfDefault := resolveInt(cmd, "confidence-default", confidenceDefault, defaultVerification.ConfidenceDefault)
			resolvedConfCritical := resolveInt(cmd, "confidence-critical", confidenceCritical, defaultVerification.ConfidenceCritical)
			resolvedConfHigh := resolveInt(cmd, "confidence-high", confidenceHigh, defaultVerification.ConfidenceHigh)
			resolvedConfMedium := resolveInt(cmd, "confidence-medium", confidenceMedium, defaultVerification.ConfidenceMedium)
			resolvedConfLow := resolveInt(cmd, "confidence-low", confidenceLow, defaultVerification.ConfidenceLow)

			_, err := branchReviewer.ReviewBranch(ctx, review.BranchRequest{
				BaseRef:             baseRef,
				TargetRef:           targetRef,
				OutputDir:           outputDir,
				Repository:          repository,
				IncludeUncommitted:  includeUncommitted,
				CustomInstructions:  customInstructions,
				ContextFiles:        contextFiles,
				NoArchitecture:      noArchitecture,
				NoAutoContext:       noAutoContext,
				Interactive:         interactive,
				PostToGitHub:        postGitHubReview,
				GitHubOwner:         githubOwner,
				GitHubRepo:          githubRepo,
				PRNumber:            prNumber,
				CommitSHA:           commitSHA,
				ActionOnCritical:    resolvedActionCritical,
				ActionOnHigh:        resolvedActionHigh,
				ActionOnMedium:      resolvedActionMedium,
				ActionOnLow:         resolvedActionLow,
				ActionOnClean:       resolvedActionClean,
				ActionOnNonBlocking: resolvedActionNonBlocking,
				BotUsername:         resolvedBotUsername,
				SkipVerification:    !resolvedVerifyEnabled,
				VerificationConfig: review.VerificationSettings{
					Depth:              resolvedDepth,
					CostCeiling:        resolvedCostCeiling,
					ConfidenceDefault:  resolvedConfDefault,
					ConfidenceCritical: resolvedConfCritical,
					ConfidenceHigh:     resolvedConfHigh,
					ConfidenceMedium:   resolvedConfMedium,
					ConfidenceLow:      resolvedConfLow,
				},
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
	cmd.Flags().StringVar(&actionNonBlocking, "action-non-blocking", "", "Review action when findings exist but none block (approve, comment)")

	// Verification flags
	cmd.Flags().BoolVar(&verify, "verify", false, "Enable agent-based verification of findings (overrides config)")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Skip agent-based verification of findings (faster, but may include more false positives)")
	cmd.Flags().StringVar(&verificationDepth, "verification-depth", "", "Verification depth: minimal, medium, or thorough (default from config)")
	cmd.Flags().Float64Var(&verificationCostCeiling, "verification-cost-ceiling", 0, "Max cost in dollars for verification (0 uses config default)")
	cmd.Flags().IntVar(&confidenceDefault, "confidence-default", 0, "Default confidence threshold (0 uses config default)")
	cmd.Flags().IntVar(&confidenceCritical, "confidence-critical", 0, "Confidence threshold for critical findings (0 uses config default)")
	cmd.Flags().IntVar(&confidenceHigh, "confidence-high", 0, "Confidence threshold for high severity findings (0 uses config default)")
	cmd.Flags().IntVar(&confidenceMedium, "confidence-medium", 0, "Confidence threshold for medium severity findings (0 uses config default)")
	cmd.Flags().IntVar(&confidenceLow, "confidence-low", 0, "Confidence threshold for low severity findings (0 uses config default)")

	return cmd
}

// resolveAction returns the override value if non-empty, otherwise the default.
func resolveAction(override, defaultValue string) string {
	if override != "" {
		return override
	}
	return defaultValue
}

// resolveVerifyEnabled determines whether verification is enabled based on CLI flags and config.
// Priority: --no-verify (disables) > --verify (enables) > config default
func resolveVerifyEnabled(cmd *cobra.Command, verify, noVerify, configDefault bool) bool {
	// --no-verify explicitly disables verification
	if cmd.Flags().Changed("no-verify") && noVerify {
		return false
	}
	// --verify explicitly enables verification
	if cmd.Flags().Changed("verify") && verify {
		return true
	}
	// Fall back to config default
	return configDefault
}

// resolveVerificationDepth validates and resolves the verification depth setting.
// Returns the CLI value if set and valid, otherwise the config default.
// Invalid values trigger a warning and fall back to the config default.
func resolveVerificationDepth(cmd *cobra.Command, cliValue, configDefault string) string {
	if !cmd.Flags().Changed("verification-depth") || cliValue == "" {
		return configDefault
	}

	validDepths := map[string]bool{"minimal": true, "medium": true, "thorough": true}
	if validDepths[cliValue] {
		return cliValue
	}

	// Warn and fall back to config default for invalid values
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: invalid verification depth %q, using config default %q\n", cliValue, configDefault)
	return configDefault
}

// resolveFloat64 returns the CLI value if the flag was explicitly set,
// otherwise returns the config default. Validates the value is non-negative.
func resolveFloat64(cmd *cobra.Command, flagName string, cliValue, configDefault float64) float64 {
	if !cmd.Flags().Changed(flagName) {
		return configDefault
	}
	// Validate non-negative (cost ceiling, etc. should not be negative)
	if cliValue < 0 {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: negative value %.2f for --%s, using config default %.2f\n", cliValue, flagName, configDefault)
		return configDefault
	}
	return cliValue
}

// resolveInt returns the CLI value if the flag was explicitly set,
// otherwise returns the config default. For confidence flags (0-100), validates the range.
func resolveInt(cmd *cobra.Command, flagName string, cliValue, configDefault int) int {
	if !cmd.Flags().Changed(flagName) {
		return configDefault
	}
	// Confidence values must be in 0-100 range
	if strings.HasPrefix(flagName, "confidence-") {
		if cliValue < 0 || cliValue > 100 {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: confidence value %d out of range (0-100) for --%s, using config default %d\n", cliValue, flagName, configDefault)
			return configDefault
		}
	} else if cliValue < 0 {
		// Other int flags should be non-negative
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: negative value %d for --%s, using config default %d\n", cliValue, flagName, configDefault)
		return configDefault
	}
	return cliValue
}
