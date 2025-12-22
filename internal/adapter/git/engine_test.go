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

	"github.com/bkyoung/code-reviewer/internal/adapter/git"
	"github.com/bkyoung/code-reviewer/internal/domain"
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

func TestIsBinaryPatch(t *testing.T) {
	tests := []struct {
		name     string
		patch    string
		expected bool
	}{
		{
			name:     "binary files differ",
			patch:    "Binary files a/image.png and b/image.png differ\n",
			expected: true,
		},
		{
			name:     "GIT binary patch",
			patch:    "GIT binary patch\nliteral 1234\n...",
			expected: true,
		},
		{
			name:     "normal text diff",
			patch:    "@@ -1,3 +1,4 @@\n context\n+added\n",
			expected: false,
		},
		{
			name:     "empty patch",
			patch:    "",
			expected: false,
		},
		{
			name:     "patch mentioning binary in content",
			patch:    "@@ -1,1 +1,1 @@\n-// Binary files are not supported\n+// Binary files are now supported\n",
			expected: false, // Fixed: only matches when "Binary files " starts a line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.IsBinaryPatch(tt.patch)
			if got != tt.expected {
				t.Errorf("IsBinaryPatch(%q) = %v, want %v", tt.patch, got, tt.expected)
			}
		})
	}
}

func TestExtractPathAndOldPath(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantPath    string
		wantOldPath string
	}{
		{
			name:        "modified file",
			line:        "M  main.go",
			wantPath:    "main.go",
			wantOldPath: "",
		},
		{
			name:        "added file",
			line:        "A  new_file.go",
			wantPath:    "new_file.go",
			wantOldPath: "",
		},
		{
			name:        "renamed file",
			line:        "R  old_name.go -> new_name.go",
			wantPath:    "new_name.go",
			wantOldPath: "old_name.go",
		},
		{
			name:        "renamed file with spaces in path",
			line:        "R  old name.go -> new name.go",
			wantPath:    "new name.go",
			wantOldPath: "old name.go",
		},
		{
			name:        "short line returns trimmed input",
			line:        "M ",
			wantPath:    "M", // Edge case: returns trimmed whole line (caller filters short lines)
			wantOldPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotOldPath := git.ExtractPathAndOldPath(tt.line)
			if gotPath != tt.wantPath {
				t.Errorf("ExtractPathAndOldPath(%q) path = %q, want %q", tt.line, gotPath, tt.wantPath)
			}
			if gotOldPath != tt.wantOldPath {
				t.Errorf("ExtractPathAndOldPath(%q) oldPath = %q, want %q", tt.line, gotOldPath, tt.wantOldPath)
			}
		})
	}
}

func TestMapGitStatus(t *testing.T) {
	tests := []struct {
		status   rune
		expected string
	}{
		{'A', domain.FileStatusAdded},
		{'?', domain.FileStatusAdded},
		{'D', domain.FileStatusDeleted},
		{'R', domain.FileStatusRenamed},
		{'M', domain.FileStatusModified},
		{'U', domain.FileStatusModified}, // Unknown defaults to modified
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := git.MapGitStatus(tt.status)
			if got != tt.expected {
				t.Errorf("MapGitStatus(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}
