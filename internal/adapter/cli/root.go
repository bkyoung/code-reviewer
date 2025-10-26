package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

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

// Dependencies captures the collaborators for the CLI.
type Dependencies struct {
	BranchReviewer BranchReviewer
	Args           Arguments
	DefaultOutput  string
	DefaultRepo    string
	Version        string
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
	reviewCmd.AddCommand(branchCommand(deps.BranchReviewer, deps.DefaultOutput, deps.DefaultRepo))
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

func branchCommand(branchReviewer BranchReviewer, defaultOutput, defaultRepo string) *cobra.Command {
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
	cmd.Flags().BoolVar(&interactive, "interactive", false, "Interactive mode with planning")
	_ = cmd.Flags().MarkHidden("interactive") // Not yet implemented
	cmd.Flags().BoolVar(&noPlanning, "no-planning", false, "Skip planning in interactive mode")
	_ = cmd.Flags().MarkHidden("no-planning") // Not yet implemented
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Dry-run showing what context would be gathered")
	_ = cmd.Flags().MarkHidden("plan-only") // Not yet implemented
	cmd.Flags().BoolVar(&noArchitecture, "no-architecture", false, "Skip loading ARCHITECTURE.md")
	cmd.Flags().BoolVar(&noAutoContext, "no-auto-context", false, "Disable automatic context gathering (design docs, relevant docs)")

	return cmd
}
