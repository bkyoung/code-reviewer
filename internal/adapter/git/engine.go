package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	formatdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// Engine implements the GitEngine port backed by go-git.
type Engine struct {
	repoDir string
}

// NewEngine constructs a Git engine for the provided repository directory.
func NewEngine(repoDir string) *Engine {
	return &Engine{repoDir: repoDir}
}

// GetCumulativeDiff creates a diff between the supplied refs.
func (e *Engine) GetCumulativeDiff(ctx context.Context, baseRef, targetRef string, includeUncommitted bool) (domain.Diff, error) {
	repo, err := goGit.PlainOpenWithOptions(e.repoDir, &goGit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return domain.Diff{}, fmt.Errorf("open repo: %w", err)
	}

	baseCommit, err := resolveCommit(repo, baseRef)
	if err != nil {
		return domain.Diff{}, fmt.Errorf("resolve base ref: %w", err)
	}

	targetCommit, err := resolveCommit(repo, targetRef)
	if err != nil {
		return domain.Diff{}, fmt.Errorf("resolve target ref: %w", err)
	}

	if includeUncommitted {
		fileDiffs, err := diffWithWorkingTree(ctx, e.repoDir, baseRef)
		if err != nil {
			return domain.Diff{}, err
		}
		return domain.Diff{
			FromCommitHash: baseCommit.Hash.String(),
			ToCommitHash:   targetCommit.Hash.String(),
			Files:          fileDiffs,
		}, nil
	}

	patch, err := baseCommit.Patch(targetCommit)
	if err != nil {
		return domain.Diff{}, fmt.Errorf("compute patch: %w", err)
	}

	fileDiffs := make([]domain.FileDiff, 0, len(patch.FilePatches()))
	for _, fp := range patch.FilePatches() {
		path, status := diffPathAndStatus(fp)
		patchText, err := encodeFilePatch(fp)
		if err != nil {
			return domain.Diff{}, fmt.Errorf("encode patch: %w", err)
		}
		fileDiffs = append(fileDiffs, domain.FileDiff{
			Path:   path,
			Status: status,
			Patch:  patchText,
		})
	}

	return domain.Diff{
		FromCommitHash: baseCommit.Hash.String(),
		ToCommitHash:   targetCommit.Hash.String(),
		Files:          fileDiffs,
	}, nil
}

// CurrentBranch returns the name of the checked-out branch.
func (e *Engine) CurrentBranch(ctx context.Context) (string, error) {
	repo, err := goGit.PlainOpenWithOptions(e.repoDir, &goGit.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("resolve HEAD: %w", err)
	}
	name := head.Name()
	if name.IsBranch() {
		return name.Short(), nil
	}
	return "", fmt.Errorf("detached HEAD")
}

func resolveCommit(repo *goGit.Repository, ref string) (*object.Commit, error) {
	candidates := []string{
		ref,
		fmt.Sprintf("refs/heads/%s", ref),
		fmt.Sprintf("refs/remotes/origin/%s", ref),
	}

	var lastErr error
	for _, candidate := range candidates {
		name := plumbing.Revision(candidate)
		hash, err := repo.ResolveRevision(name)
		if err != nil {
			lastErr = err
			continue
		}
		return repo.CommitObject(*hash)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("unable to resolve ref %s", ref)
}

func diffPathAndStatus(fp formatdiff.FilePatch) (string, string) {
	from, to := fp.Files()

	switch {
	case from == nil && to != nil:
		return to.Path(), domain.FileStatusAdded
	case from != nil && to == nil:
		return from.Path(), domain.FileStatusDeleted
	case from != nil && to != nil:
		return to.Path(), domain.FileStatusModified
	default:
		return "", domain.FileStatusModified
	}
}

func diffWithWorkingTree(ctx context.Context, repoDir, baseRef string) ([]domain.FileDiff, error) {
	statusOut, err := runGitCommand(ctx, repoDir, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	trimmed := strings.TrimRight(statusOut, "\r\n")
	if trimmed == "" {
		return []domain.FileDiff{}, nil
	}
	lines := strings.Split(trimmed, "\n")
	diffs := make([]domain.FileDiff, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if len(line) < 3 {
			continue
		}
		statusChar := selectStatusChar(line)
		path := extractPath(line)
		patchOut, err := runGitCommand(ctx, repoDir, "diff", baseRef, "--", path)
		if err != nil {
			return nil, fmt.Errorf("git diff %s: %w", path, err)
		}
		diffs = append(diffs, domain.FileDiff{
			Path:   path,
			Status: mapGitStatus(statusChar),
			Patch:  patchOut,
		})
	}
	return diffs, nil
}

func runGitCommand(ctx context.Context, repoDir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git %v: %w", args, ctx.Err())
		}
		if stderr.Len() > 0 {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return stdout.String(), nil
}

func selectStatusChar(line string) rune {
	if len(line) < 2 {
		return 'M'
	}
	first := rune(line[0])
	second := rune(line[1])
	switch {
	case second != ' ':
		return second
	case first != ' ':
		return first
	default:
		return 'M'
	}
}

func extractPath(line string) string {
	if len(line) <= 3 {
		return strings.TrimSpace(line)
	}
	path := strings.TrimSpace(line[3:])
	if strings.Contains(path, " -> ") {
		parts := strings.Split(path, " -> ")
		path = strings.TrimSpace(parts[len(parts)-1])
	}
	return path
}

func mapGitStatus(status rune) string {
	switch status {
	case 'A', '?':
		return domain.FileStatusAdded
	case 'D':
		return domain.FileStatusDeleted
	default:
		return domain.FileStatusModified
	}
}

func encodeFilePatch(fp formatdiff.FilePatch) (string, error) {
	var buf bytes.Buffer
	encoder := formatdiff.NewUnifiedEncoder(&buf, formatdiff.DefaultContextLines)
	if err := encoder.Encode(singlePatch{fp: fp}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type singlePatch struct {
	fp formatdiff.FilePatch
}

func (s singlePatch) FilePatches() []formatdiff.FilePatch {
	return []formatdiff.FilePatch{s.fp}
}

func (s singlePatch) Message() string {
	return ""
}
