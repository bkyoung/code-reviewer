package github

import (
	"fmt"
	"sort"
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

// =============================================================================
// Programmatic Summary Builder
// =============================================================================

// BuildProgrammaticSummary generates a structured code review summary from findings.
// This replaces the LLM-generated summary with a consistent, programmatic format.
//
// The summary includes:
//   - Badge line with file count and severity counts (only in-diff findings)
//   - Files Requiring Attention section (files with severities that trigger REQUEST_CHANGES)
//   - Findings by Category table
//
// The actions parameter determines which severities appear in "Files Requiring Attention".
// Any severity configured to trigger REQUEST_CHANGES will be included.
// If actions is empty/default, critical and high severities are included.
func BuildProgrammaticSummary(findings []PositionedFinding, d domain.Diff, actions ReviewActions) string {
	fileCount := len(d.Files)

	// Filter to only in-diff findings for counting
	inDiffFindings := filterInDiff(findings)

	// Count findings by severity
	counts := countBySeverity(inDiffFindings)
	totalFindings := counts["critical"] + counts["high"] + counts["medium"] + counts["low"]

	// Clean code case
	if totalFindings == 0 {
		return fmt.Sprintf("✅ **No issues found.** Reviewed %d files.", fileCount)
	}

	var sb strings.Builder

	// Badge line
	sb.WriteString(formatBadgeLine(fileCount, counts))
	sb.WriteString("\n\n")

	// Files requiring attention (based on configured blocking severities)
	attentionSeverities := getAttentionSeverities(actions)
	if section := formatFilesRequiringAttention(inDiffFindings, attentionSeverities); section != "" {
		sb.WriteString(section)
		sb.WriteString("\n")
	}

	// Category breakdown table
	categoryGroups := groupByCategory(inDiffFindings)
	if table := formatCategoryTable(categoryGroups); table != "" {
		sb.WriteString(table)
	}

	return strings.TrimRight(sb.String(), "\n")
}

// countBySeverity returns counts for each severity level.
func countBySeverity(findings []PositionedFinding) map[string]int {
	counts := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
	for _, pf := range findings {
		severity := strings.ToLower(pf.Finding.Severity)
		if _, ok := counts[severity]; ok {
			counts[severity]++
		}
	}
	return counts
}

// formatBadgeLine creates the emoji badge summary line.
// Example: 📊 **Reviewed 12 files** | 🔴 2 critical | 🟠 5 high | 🟡 3 medium | 🟢 1 low
func formatBadgeLine(fileCount int, counts map[string]int) string {
	parts := []string{
		fmt.Sprintf("📊 **Reviewed %d files**", fileCount),
	}

	// Always show all severity levels for consistency
	parts = append(parts, fmt.Sprintf("🔴 %d critical", counts["critical"]))
	parts = append(parts, fmt.Sprintf("🟠 %d high", counts["high"]))
	parts = append(parts, fmt.Sprintf("🟡 %d medium", counts["medium"]))
	parts = append(parts, fmt.Sprintf("🟢 %d low", counts["low"]))

	return strings.Join(parts, " | ")
}

// getAttentionSeverities returns a map of severities that should appear in
// the "Files Requiring Attention" section. These are severities configured
// to trigger REQUEST_CHANGES.
func getAttentionSeverities(actions ReviewActions) map[string]bool {
	result := make(map[string]bool)

	checkAction := func(severity, action string, defaultBlocking bool) {
		if action == "" {
			// Use default behavior
			if defaultBlocking {
				result[severity] = true
			}
			return
		}
		if event, valid := NormalizeAction(action); valid && event == EventRequestChanges {
			result[severity] = true
		}
	}

	// Default: critical and high trigger request_changes
	checkAction("critical", actions.OnCritical, true)
	checkAction("high", actions.OnHigh, true)
	checkAction("medium", actions.OnMedium, false)
	checkAction("low", actions.OnLow, false)

	return result
}

// severityOrder defines the display order for severity levels (highest first).
var severityOrder = []string{"critical", "high", "medium", "low"}

// formatFilesRequiringAttention creates the "Files Requiring Attention" section.
// Only includes files with findings at attention-worthy severities.
func formatFilesRequiringAttention(findings []PositionedFinding, attentionSeverities map[string]bool) string {
	if len(attentionSeverities) == 0 {
		return ""
	}

	// Group findings by file, counting by severity (map-based approach)
	fileFindings := make(map[string]map[string]int)

	for _, pf := range findings {
		severity := strings.ToLower(pf.Finding.Severity)
		if !attentionSeverities[severity] {
			continue
		}

		if fileFindings[pf.Finding.File] == nil {
			fileFindings[pf.Finding.File] = make(map[string]int)
		}
		fileFindings[pf.Finding.File][severity]++
	}

	if len(fileFindings) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Files Requiring Attention\n\n")

	// Sort files for deterministic output
	var files []string
	for file := range fileFindings {
		files = append(files, file)
	}
	sort.Strings(files)

	for _, file := range files {
		counts := fileFindings[file]

		var badges []string
		for _, severity := range severityOrder {
			if count := counts[severity]; count > 0 {
				badges = append(badges, fmt.Sprintf("%d %s", count, severity))
			}
		}

		sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", file, strings.Join(badges, ", ")))
	}

	return sb.String()
}

// groupByCategory groups findings by their category.
func groupByCategory(findings []PositionedFinding) map[string]int {
	groups := make(map[string]int)
	for _, pf := range findings {
		category := pf.Finding.Category
		if category == "" {
			category = "general"
		}
		groups[category]++
	}
	return groups
}

// formatCategoryTable creates the "Findings by Category" table.
func formatCategoryTable(categoryCounts map[string]int) string {
	if len(categoryCounts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Findings by Category\n\n")
	sb.WriteString("| Category | Count |\n")
	sb.WriteString("|----------|-------|\n")

	// Sort categories for deterministic output
	var categories []string
	for cat := range categoryCounts {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", cat, categoryCounts[cat]))
	}

	return sb.String()
}
