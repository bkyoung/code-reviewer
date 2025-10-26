package review

import (
	"os"

	"golang.org/x/term"
)

// IsTTY checks if the given file descriptor is a terminal.
// This is useful for detecting whether the application is running
// in an interactive environment (e.g., a user's terminal) or
// in a non-interactive environment (e.g., CI/CD pipeline, piped input).
//
// Example:
//
//	if IsTTY(os.Stdin.Fd()) {
//	    // Running in interactive terminal
//	} else {
//	    // Running in CI/CD or with piped input
//	}
func IsTTY(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// IsInteractive checks if stdin is a TTY, indicating that the user
// can provide interactive input. This is the primary check for
// determining whether to enable interactive features like the planning agent.
//
// Returns false in CI/CD environments, when input is piped, or when
// running as a background process.
//
// Example:
//
//	if IsInteractive() {
//	    // Enable interactive planning mode
//	    runPlanningAgent()
//	}
func IsInteractive() bool {
	return IsTTY(os.Stdin.Fd())
}

// IsOutputTerminal checks if stdout is a TTY, indicating that output
// is being displayed directly to a user's terminal rather than being
// piped or redirected.
//
// This can be used to enable colored output, progress bars, or other
// terminal-specific formatting.
//
// Example:
//
//	if IsOutputTerminal() {
//	    // Enable colored output and progress indicators
//	    fmt.Println("\033[32mSuccess!\033[0m")
//	}
func IsOutputTerminal() bool {
	return IsTTY(os.Stdout.Fd())
}
