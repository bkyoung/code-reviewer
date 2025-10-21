package git_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/brandon/code-reviewer/internal/adapter/git"
	"github.com/brandon/code-reviewer/internal/domain"
)

func TestEngineGetCumulativeDiffForBranch(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()

	repo, err := goGit.PlainInit(tmp, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	writeFile(t, tmp, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	if _, err := worktree.Add("main.go"); err != nil {
		t.Fatalf("add error: %v", err)
	}
	_, err = worktree.Commit("initial", &goGit.CommitOptions{
		Author: defaultSignature(),
	})
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if err := checkoutBranch(worktree, "feature"); err != nil {
		t.Fatalf("checkout error: %v", err)
	}

	writeFile(t, tmp, "main.go", "package main\n\nfunc main() {\n\tprintln(\"feature\")\n}\n")
	if _, err := worktree.Add("main.go"); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if _, err := worktree.Commit("feature change", &goGit.CommitOptions{
		Author: defaultSignature(),
	}); err != nil {
		t.Fatalf("feature commit error: %v", err)
	}

	engine := git.NewEngine(tmp)
	diff, err := engine.GetCumulativeDiff(ctx, "master", "feature", false)
	if err != nil {
		t.Fatalf("GetCumulativeDiff returned error: %v", err)
	}

	if diff.FromCommitHash == "" || diff.ToCommitHash == "" {
		t.Fatalf("expected commit hashes to be populated: %+v", diff)
	}

	if len(diff.Files) != 1 {
		t.Fatalf("expected 1 file diff, got %d", len(diff.Files))
	}

	if diff.Files[0].Status != domain.FileStatusModified {
		t.Fatalf("expected modified status, got %s", diff.Files[0].Status)
	}

	if !contains(diff.Files[0].Patch, "feature") {
		t.Fatalf("expected patch to include change: %s", diff.Files[0].Patch)
	}
}

func TestEngineIncludesUncommittedChanges(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()

	repo, err := goGit.PlainInit(tmp, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	writeFile(t, tmp, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	if _, err := worktree.Add("main.go"); err != nil {
		t.Fatalf("add error: %v", err)
	}
	if _, err := worktree.Commit("initial", &goGit.CommitOptions{Author: defaultSignature()}); err != nil {
		t.Fatalf("commit error: %v", err)
	}

	// Modify without committing.
	writeFile(t, tmp, "main.go", "package main\n\nfunc main() {\n\tprintln(\"working tree change\")\n}\n")

	engine := git.NewEngine(tmp)
	diff, err := engine.GetCumulativeDiff(ctx, "master", "master", true)
	if err != nil {
		t.Fatalf("GetCumulativeDiff returned error: %v", err)
	}

	if len(diff.Files) != 1 {
		t.Fatalf("expected 1 file diff, got %d", len(diff.Files))
	}
	if !contains(diff.Files[0].Patch, "working tree change") {
		t.Fatalf("expected patch to include working tree change, got %s", diff.Files[0].Patch)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write file error: %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func defaultSignature() *object.Signature {
	return &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Unix(0, 0),
	}
}

func checkoutBranch(worktree *goGit.Worktree, branch string) error {
	return worktree.Checkout(&goGit.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Create: true,
	})
}
