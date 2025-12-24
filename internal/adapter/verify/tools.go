package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/usecase/verify"
)

// MaxToolOutputLength is the maximum length of tool output before truncation.
// This prevents runaway memory usage from large files or command output.
const MaxToolOutputLength = 50000

// Tool defines the interface for verification agent tools.
// Tools provide capabilities for the agent to investigate candidate findings.
type Tool interface {
	// Name returns the tool identifier used in prompts and logs.
	Name() string

	// Description returns a human-readable description for the agent prompt.
	Description() string

	// Execute runs the tool with the given input and returns the result.
	// The context allows for cancellation and timeout.
	Execute(ctx context.Context, input string) (string, error)
}

// NewToolRegistry creates all verification tools from a repository.
func NewToolRegistry(repo verify.Repository) []Tool {
	return []Tool{
		NewReadFileTool(repo),
		NewGrepTool(repo),
		NewGlobTool(repo),
		NewBashTool(repo),
	}
}

// ReadFileTool reads file contents from the repository.
type ReadFileTool struct {
	repo verify.Repository
}

// NewReadFileTool creates a new read file tool.
func NewReadFileTool(repo verify.Repository) *ReadFileTool {
	return &ReadFileTool{repo: repo}
}

// Name returns the tool name.
func (t *ReadFileTool) Name() string {
	return "read_file"
}

// Description returns the tool description.
func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Input: file path (e.g., 'src/main.go')"
}

// Execute reads the file at the given path.
func (t *ReadFileTool) Execute(ctx context.Context, input string) (string, error) {
	path := strings.TrimSpace(input)
	if path == "" {
		return "", fmt.Errorf("file path required")
	}

	content, err := t.repo.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", path, err)
	}

	result := string(content)
	return truncateOutput(result), nil
}

// GrepTool searches for patterns in the repository.
type GrepTool struct {
	repo verify.Repository
}

// NewGrepTool creates a new grep tool.
func NewGrepTool(repo verify.Repository) *GrepTool {
	return &GrepTool{repo: repo}
}

// Name returns the tool name.
func (t *GrepTool) Name() string {
	return "grep"
}

// Description returns the tool description.
func (t *GrepTool) Description() string {
	return "Search for a pattern in the codebase. Input: search pattern (regex supported)"
}

// Execute searches for the pattern in the repository.
func (t *GrepTool) Execute(ctx context.Context, input string) (string, error) {
	pattern := strings.TrimSpace(input)
	if pattern == "" {
		return "", fmt.Errorf("search pattern required")
	}

	matches, err := t.repo.Grep(pattern)
	if err != nil {
		return "", fmt.Errorf("grep %s: %w", pattern, err)
	}

	if len(matches) == 0 {
		return "No matches found", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matches:\n", len(matches)))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", m.File, m.Line, m.Content))
	}

	return truncateOutput(sb.String()), nil
}

// GlobTool finds files matching a pattern.
type GlobTool struct {
	repo verify.Repository
}

// NewGlobTool creates a new glob tool.
func NewGlobTool(repo verify.Repository) *GlobTool {
	return &GlobTool{repo: repo}
}

// Name returns the tool name.
func (t *GlobTool) Name() string {
	return "glob"
}

// Description returns the tool description.
func (t *GlobTool) Description() string {
	return "Find files matching a pattern. Input: glob pattern (e.g., '**/*.go', 'internal/**/test_*.go')"
}

// Execute finds files matching the pattern.
func (t *GlobTool) Execute(ctx context.Context, input string) (string, error) {
	pattern := strings.TrimSpace(input)
	if pattern == "" {
		return "", fmt.Errorf("glob pattern required")
	}

	files, err := t.repo.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob %s: %w", pattern, err)
	}

	if len(files) == 0 {
		return "No files found matching pattern", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d files:\n", len(files)))
	for _, f := range files {
		sb.WriteString(f + "\n")
	}

	return truncateOutput(sb.String()), nil
}

// BashTool runs safe commands in the repository.
type BashTool struct {
	repo verify.Repository
}

// NewBashTool creates a new bash tool.
func NewBashTool(repo verify.Repository) *BashTool {
	return &BashTool{repo: repo}
}

// Name returns the tool name.
func (t *BashTool) Name() string {
	return "bash"
}

// Description returns the tool description.
func (t *BashTool) Description() string {
	return "Run a safe command (go build, go vet, go test, git diff, etc.). Input: command and arguments"
}

// safeCommands defines which commands are allowed and their safe subcommands.
// Commands with nil subcommand lists are allowed with any arguments.
// Commands with non-nil subcommand lists are only allowed with those specific subcommands.
var safeCommands = map[string][]string{
	// Go commands - only safe read-only and build operations
	"go": {"build", "test", "vet", "list", "version", "env", "mod"},
	// Git commands - only read-only operations
	"git": {"status", "log", "show", "diff", "branch", "rev-parse", "describe", "ls-files"},
	// Read-only utilities
	"echo": nil, // Any args OK for echo
	"cat":  nil, // Any args OK for cat
	"head": nil, // Any args OK
	"tail": nil, // Any args OK
	"wc":   nil, // Any args OK
	"grep": nil, // Any args OK
	"find": nil, // Any args OK
	"ls":   nil, // Any args OK
}

// dangerousPatterns are patterns that should never be allowed.
var dangerousPatterns = []string{
	// File deletion/modification
	"rm ",
	"rm\t",
	"rmdir",
	"mv ",
	"mv\t",
	"dd ",
	"dd\t",
	// Network access
	"curl",
	"wget",
	"nc ",
	"nc\t",
	"netcat",
	"ssh",
	"scp",
	"rsync",
	// Privilege escalation
	"chmod",
	"chown",
	"sudo",
	"su ",
	"su\t",
	// Code execution
	"eval",
	"exec",
	"xargs",
	"env ",
	"env\t",
	// Shell spawning
	"sh ",
	"sh\t",
	"bash",
	"zsh",
	"python",
	"python3",
	"ruby",
	"perl",
	"node",
	// Shell metacharacters
	">",   // Redirect output
	">>",  // Append output
	"|",   // Pipe (could be used to bypass restrictions)
	";",   // Command chaining
	"&&",  // Command chaining
	"||",  // Command chaining
	"`",   // Command substitution
	"$(",  // Command substitution
	"${",  // Variable expansion (could be exploited)
	"\\n", // Newline escape (could inject commands)
}

// Execute runs the command if it's in the allowlist.
func (t *BashTool) Execute(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("command required")
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if strings.Contains(input, pattern) {
			return "", fmt.Errorf("command contains forbidden pattern: %s", pattern)
		}
	}

	// Parse command
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := parts[0]
	args := parts[1:]

	// Check allowlist
	allowedSubcmds, cmdAllowed := safeCommands[cmd]
	if !cmdAllowed {
		return "", fmt.Errorf("command %q not in allowlist", cmd)
	}

	// If subcommands are restricted, verify the first argument is allowed
	if allowedSubcmds != nil {
		if len(args) == 0 {
			return "", fmt.Errorf("command %q requires a subcommand", cmd)
		}
		subcmd := args[0]
		allowed := false
		for _, s := range allowedSubcmds {
			if s == subcmd {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("subcommand %q not allowed for %q (allowed: %v)", subcmd, cmd, allowedSubcmds)
		}
	}

	result, err := t.repo.RunCommand(ctx, cmd, args...)
	if err != nil {
		return "", fmt.Errorf("running command: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Exit code: %d\n", result.ExitCode))

	if result.Stdout != "" {
		sb.WriteString("Stdout:\n")
		sb.WriteString(result.Stdout)
		sb.WriteString("\n")
	}

	if result.Stderr != "" {
		sb.WriteString("Stderr:\n")
		sb.WriteString(result.Stderr)
		sb.WriteString("\n")
	}

	return truncateOutput(sb.String()), nil
}

// truncateOutput truncates output that exceeds MaxToolOutputLength.
func truncateOutput(s string) string {
	if len(s) <= MaxToolOutputLength {
		return s
	}
	return s[:MaxToolOutputLength] + "\n... [output truncated]"
}
