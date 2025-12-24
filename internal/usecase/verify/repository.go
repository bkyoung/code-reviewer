package verify

import "context"

// Repository provides access to the codebase for verification.
// This interface abstracts filesystem and command execution, enabling
// the verification agent to investigate candidate findings.
type Repository interface {
	// ReadFile reads the contents of a file at the given path.
	// Returns the file contents or an error if the file cannot be read.
	ReadFile(path string) ([]byte, error)

	// FileExists checks if a file exists at the given path.
	// Returns false for directories, permission errors, or any other non-file.
	FileExists(path string) bool

	// Glob returns file paths matching the given pattern.
	// Uses standard filepath.Glob syntax (e.g., "**/*.go", "internal/**/test_*.go").
	Glob(pattern string) ([]string, error)

	// Grep searches for a pattern in the specified files.
	// If no paths are provided, searches the entire repository.
	// Returns matching lines with file, line number, and content.
	Grep(pattern string, paths ...string) ([]GrepMatch, error)

	// RunCommand executes a command in the repository context.
	// The context allows for cancellation and timeout.
	// Returns the command result including stdout, stderr, and exit code.
	RunCommand(ctx context.Context, cmd string, args ...string) (CommandResult, error)
}

// GrepMatch represents a single match from a grep operation.
type GrepMatch struct {
	File    string `json:"file"`    // Path to the file containing the match
	Line    int    `json:"line"`    // Line number (1-indexed)
	Content string `json:"content"` // The matching line content
}

// CommandResult captures the output of a command execution.
type CommandResult struct {
	Stdout   string `json:"stdout"`   // Standard output
	Stderr   string `json:"stderr"`   // Standard error
	ExitCode int    `json:"exitCode"` // Exit code (0 = success)
}

// Success returns true if the command exited with code 0.
func (r CommandResult) Success() bool {
	return r.ExitCode == 0
}
