// Package skip provides skip trigger detection for code reviews.
// It allows users to bypass code review by including specific patterns
// in commit messages or PR descriptions.
package skip

import (
	"regexp"
	"strings"
)

// skipTriggerPattern matches [skip code-review] or [skip-code-review] (case-insensitive).
var skipTriggerPattern = regexp.MustCompile(`(?i)\[skip[ -]code-review\]`)

// ContainsSkipTrigger checks if text contains a skip trigger pattern.
// Supported patterns:
//   - [skip code-review]
//   - [skip-code-review]
//
// Matching is case-insensitive.
func ContainsSkipTrigger(text string) bool {
	return skipTriggerPattern.MatchString(text)
}

// CheckRequest contains the inputs to check for skip triggers.
type CheckRequest struct {
	CommitMessages []string // Commit messages in the PR (optional)
	PRTitle        string   // PR title (optional)
	PRDescription  string   // PR description/body (optional)
}

// CheckResult contains the result of checking for skip triggers.
type CheckResult struct {
	ShouldSkip bool   // True if a skip trigger was found
	Reason     string // Source where trigger was found ("commit message", "PR title", "PR description")
}

// Check examines commit messages and PR metadata for skip triggers.
// It checks in order: commit messages, PR title, PR description.
// Returns the first match found.
func Check(req CheckRequest) CheckResult {
	// Check commit messages first
	for _, msg := range req.CommitMessages {
		if ContainsSkipTrigger(msg) {
			return CheckResult{
				ShouldSkip: true,
				Reason:     "commit message",
			}
		}
	}

	// Check PR title
	if ContainsSkipTrigger(strings.TrimSpace(req.PRTitle)) {
		return CheckResult{
			ShouldSkip: true,
			Reason:     "PR title",
		}
	}

	// Check PR description
	if ContainsSkipTrigger(req.PRDescription) {
		return CheckResult{
			ShouldSkip: true,
			Reason:     "PR description",
		}
	}

	return CheckResult{
		ShouldSkip: false,
		Reason:     "",
	}
}
