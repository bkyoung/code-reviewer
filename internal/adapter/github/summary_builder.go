package github

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// BuildSummaryAppendix creates structured appendix sections for edge cases.
// Returns an empty string if there are no edge cases to report.
// The appendix includes sections for:
// - Findings outside diff (deleted lines, lines not in hunks)
// - Binary files changed
// - Renamed files
func BuildSummaryAppendix(findings []PositionedFinding, d domain.Diff) string {
	var sections []string

	// Section 1: Findings outside diff
	outOfDiff := FilterOutOfDiff(findings)
	if len(outOfDiff) > 0 {
		sections = append(sections, formatOutOfDiffSection(outOfDiff))
	}

	// Section 2: Binary files changed
	binaryFiles := FilterBinaryFiles(d.Files)
	if len(binaryFiles) > 0 {
		sections = append(sections, formatBinaryFilesSection(binaryFiles))
	}

	// Section 3: Renamed files
	renamedFiles := FilterRenamedFiles(d.Files)
	if len(renamedFiles) > 0 {
		sections = append(sections, formatRenamedFilesSection(renamedFiles))
	}

	if len(sections) == 0 {
		return ""
	}

	return "\n\n---\n\n" + strings.Join(sections, "\n\n")
}

// AppendSections appends the summary appendix to the original summary.
// If the appendix is empty, returns the original summary unchanged.
func AppendSections(originalSummary, appendix string) string {
	if appendix == "" {
		return originalSummary
	}
	return originalSummary + appendix
}

// FilterOutOfDiff returns findings that are not in the diff (DiffPosition == nil).
func FilterOutOfDiff(findings []PositionedFinding) []PositionedFinding {
	var result []PositionedFinding
	for _, pf := range findings {
		if !pf.InDiff() {
			result = append(result, pf)
		}
	}
	return result
}

// FilterBinaryFiles returns files that are binary.
func FilterBinaryFiles(files []domain.FileDiff) []domain.FileDiff {
	var result []domain.FileDiff
	for _, f := range files {
		if f.IsBinary {
			result = append(result, f)
		}
	}
	return result
}

// FilterRenamedFiles returns files that were renamed.
func FilterRenamedFiles(files []domain.FileDiff) []domain.FileDiff {
	var result []domain.FileDiff
	for _, f := range files {
		if f.Status == domain.FileStatusRenamed {
			result = append(result, f)
		}
	}
	return result
}

// formatOutOfDiffSection formats the "Findings Outside Diff" section.
func formatOutOfDiffSection(findings []PositionedFinding) string {
	var sb strings.Builder

	sb.WriteString("## Findings Outside Diff\n\n")
	sb.WriteString("The following findings are on lines not included in this diff ")
	sb.WriteString("(e.g., deleted lines or unchanged context):\n\n")

	for _, pf := range findings {
		f := pf.Finding
		sb.WriteString(fmt.Sprintf("- **%s** in `%s` (line %d): %s\n",
			f.Severity, f.File, f.LineStart, f.Description))
	}

	return sb.String()
}

// formatBinaryFilesSection formats the "Binary Files Changed" section.
func formatBinaryFilesSection(files []domain.FileDiff) string {
	var sb strings.Builder

	sb.WriteString("## Binary Files Changed\n\n")
	sb.WriteString("The following binary files were changed and excluded from review:\n\n")

	for _, f := range files {
		sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", f.Path, f.Status))
	}

	return sb.String()
}

// formatRenamedFilesSection formats the "Files Renamed" section.
func formatRenamedFilesSection(files []domain.FileDiff) string {
	var sb strings.Builder

	sb.WriteString("## Files Renamed\n\n")

	for _, f := range files {
		sb.WriteString(fmt.Sprintf("- `%s` → `%s`\n", f.OldPath, f.Path))
	}

	return sb.String()
}
