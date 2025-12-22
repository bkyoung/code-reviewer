package github

import (
	"github.com/bkyoung/code-reviewer/internal/diff"
	"github.com/bkyoung/code-reviewer/internal/domain"
)

// MapFindings enriches domain findings with GitHub diff positions.
// Findings are mapped to their corresponding position in the unified diff,
// which is required for creating inline PR review comments.
//
// If a finding's line is not in the diff (e.g., unchanged code, deleted line,
// or line outside diff hunks), DiffPosition will be nil.
//
// This function is pure and does not modify the input findings.
func MapFindings(findings []domain.Finding, d domain.Diff) []PositionedFinding {
	if len(findings) == 0 {
		return []PositionedFinding{}
	}

	// Build a map of file path -> parsed diff for O(1) lookup
	parsedDiffs := make(map[string]diff.ParsedDiff, len(d.Files))
	for _, fileDiff := range d.Files {
		parsed, err := diff.Parse(fileDiff.Patch)
		if err != nil {
			// Skip files with unparseable diffs
			continue
		}
		parsedDiffs[fileDiff.Path] = parsed
	}

	// Map each finding to its positioned version
	result := make([]PositionedFinding, len(findings))
	for i, finding := range findings {
		pf := PositionedFinding{
			Finding: finding,
		}

		// Look up the diff for this finding's file
		if parsed, ok := parsedDiffs[finding.File]; ok {
			// Get position for the finding's start line
			pf.DiffPosition = parsed.FindPosition(finding.LineStart)
		}

		result[i] = pf
	}

	return result
}
