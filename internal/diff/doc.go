// Package diff provides utilities for parsing unified diff format
// and mapping file line numbers to diff positions for GitHub PR review comments.
//
// The primary use case is to convert absolute file line numbers (from LLM
// findings) to GitHub's diff position format, which is required for creating
// inline PR review comments.
//
// Position in GitHub's API is 1-indexed from the first @@ hunk header,
// counting all lines in the diff (context, additions, and deletions).
package diff
