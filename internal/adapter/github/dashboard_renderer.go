package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

// dashboardMarker is the HTML comment that identifies unified dashboard comments.
// This must be unique enough to avoid false matches with user comments.
const dashboardMarker = "<!-- CODE_REVIEWER_DASHBOARD_V1 -->"

// dashboardMetadataStart marks the beginning of the embedded base64-encoded JSON metadata.
const dashboardMetadataStart = "<!-- DASHBOARD_METADATA_B64"

// dashboardMetadataEnd marks the end of the embedded metadata.
const dashboardMetadataEnd = "-->"

// DashboardRenderer renders unified dashboard comments for GitHub PRs.
// It combines tracking state, review results, and metadata into a single
// editable comment that serves as the "source of truth" for a PR review.
type DashboardRenderer struct{}

// NewDashboardRenderer creates a new dashboard renderer.
func NewDashboardRenderer() *DashboardRenderer {
	return &DashboardRenderer{}
}

// RenderDashboard renders a complete dashboard comment body.
// The rendering varies based on ReviewStatus:
//   - InProgress: Shows "Review In Progress" with minimal info
//   - Completed: Shows full findings table, review summary, costs, etc.
func (r *DashboardRenderer) RenderDashboard(data review.DashboardData) (string, error) {
	// Validate required fields
	if err := data.Target.Validate(); err != nil {
		return "", fmt.Errorf("invalid target: %w", err)
	}

	var sb strings.Builder

	// Marker for identification
	sb.WriteString(dashboardMarker)
	sb.WriteString("\n\n")

	if data.ReviewStatus == domain.ReviewStatusInProgress {
		r.renderInProgress(&sb, data)
	} else {
		r.renderCompleted(&sb, data)
	}

	// Embedded metadata (hidden from rendered view)
	if err := r.embedMetadata(&sb, data); err != nil {
		return "", fmt.Errorf("failed to embed metadata: %w", err)
	}

	return sb.String(), nil
}

// IsDashboardComment returns true if the comment body contains the dashboard marker.
func IsDashboardComment(body string) bool {
	return strings.Contains(body, dashboardMarker)
}

// renderInProgress renders the simplified "in progress" view.
func (r *DashboardRenderer) renderInProgress(sb *strings.Builder, data review.DashboardData) {
	sb.WriteString("## 🔄 Code Review In Progress\n\n")
	sb.WriteString("The code review is currently running. This comment will be updated with findings when complete.\n\n")

	// Show commit being reviewed
	if data.Target.HeadSHA != "" {
		shortSHA := data.Target.HeadSHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		sb.WriteString(fmt.Sprintf("**Reviewing commit:** `%s`\n\n", shortSHA))
	}

	// Last updated timestamp
	r.renderLastUpdated(sb, data.LastUpdated)
}

// renderCompleted renders the full dashboard with findings, costs, and metadata.
// Section order follows incremental disclosure: summary → open findings → resolved → metadata.
func (r *DashboardRenderer) renderCompleted(sb *strings.Builder, data review.DashboardData) {
	// Status header (most important - immediately visible)
	r.renderStatusHeader(sb, data)

	// Status counts table (quick overview)
	r.renderStatusTable(sb, data)

	// Severity badges (one-line summary)
	r.renderSeverityBadges(sb, data)

	// Files requiring attention (blocking issues - high priority)
	r.renderFilesRequiringAttention(sb, data)

	// Findings by severity with expandable details (main content)
	r.renderFindingsBySeverity(sb, data)

	// Resolved findings (collapsed, secondary importance)
	r.renderResolvedFindings(sb, data)

	// Horizontal rule to separate findings from meta sections
	sb.WriteString("---\n\n")

	// Instructions for updating finding status
	r.renderInstructions(sb, data)

	// Edge cases appendix (out-of-diff, binary, renames)
	r.renderAppendix(sb, data)

	// Review metadata (cost, provider) - bottom, collapsed
	r.renderReviewMetadata(sb, data)

	// Reviewed commits - bottom, collapsed
	r.renderReviewedCommits(sb, data)

	// Last updated timestamp
	r.renderLastUpdated(sb, data.LastUpdated)
}

// renderStatusHeader shows the review status with appropriate icon.
func (r *DashboardRenderer) renderStatusHeader(sb *strings.Builder, data review.DashboardData) {
	counts := data.CountByStatus()

	if counts.Open == 0 && counts.Total == 0 {
		sb.WriteString("## ✅ No Issues Found\n\n")
	} else if data.HasBlockingFindings() {
		sb.WriteString("## 🔴 Changes Requested\n\n")
	} else if counts.Open > 0 {
		sb.WriteString("## ✅ Approved with Suggestions\n\n")
	} else {
		sb.WriteString("## ✅ Code Review Complete\n\n")
	}
}

// renderStatusTable shows finding counts by status.
func (r *DashboardRenderer) renderStatusTable(sb *strings.Builder, data review.DashboardData) {
	counts := data.CountByStatus()

	sb.WriteString("| Status | Count |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| 🔴 Open | %d |\n", counts.Open))
	sb.WriteString(fmt.Sprintf("| ✅ Resolved | %d |\n", counts.Resolved))
	sb.WriteString(fmt.Sprintf("| 💬 Acknowledged | %d |\n", counts.Acknowledged))
	sb.WriteString(fmt.Sprintf("| ⚠️ Disputed | %d |\n", counts.Disputed))
	sb.WriteString("\n")
}

// renderSeverityBadges shows open finding counts by severity.
func (r *DashboardRenderer) renderSeverityBadges(sb *strings.Builder, data review.DashboardData) {
	counts := data.CountBySeverity()

	if counts.Total == 0 {
		return
	}

	fileCount := 0
	if data.Diff != nil {
		fileCount = len(data.Diff.Files)
	}

	sb.WriteString(fmt.Sprintf("📊 **Reviewed %d files** | ", fileCount))
	sb.WriteString(fmt.Sprintf("🔴 %d critical | ", counts.Critical))
	sb.WriteString(fmt.Sprintf("🟠 %d high | ", counts.High))
	sb.WriteString(fmt.Sprintf("🟡 %d medium | ", counts.Medium))
	sb.WriteString(fmt.Sprintf("🟢 %d low\n\n", counts.Low))
}

// renderFilesRequiringAttention lists files with blocking issues.
func (r *DashboardRenderer) renderFilesRequiringAttention(sb *strings.Builder, data review.DashboardData) {
	if len(data.AttentionSeverities) == 0 {
		return
	}

	// Group findings by file
	fileFindings := make(map[string]map[string]int)
	for _, f := range data.Findings {
		if f.Status != domain.FindingStatusOpen {
			continue
		}
		severity := f.Finding.Severity
		if !data.AttentionSeverities[severity] {
			continue
		}

		if fileFindings[f.Finding.File] == nil {
			fileFindings[f.Finding.File] = make(map[string]int)
		}
		fileFindings[f.Finding.File][severity]++
	}

	if len(fileFindings) == 0 {
		return
	}

	sb.WriteString("### Files Requiring Attention\n\n")

	// Sort files for deterministic output
	var files []string
	for file := range fileFindings {
		files = append(files, file)
	}
	sort.Strings(files)

	severityOrder := []string{"critical", "high", "medium", "low"}
	for _, file := range files {
		counts := fileFindings[file]
		var badges []string
		for _, severity := range severityOrder {
			if count := counts[severity]; count > 0 {
				badges = append(badges, fmt.Sprintf("%d %s", count, severity))
			}
		}
		sb.WriteString(fmt.Sprintf("- `%s` (%s)\n", escapeMarkdownInlineCode(file), strings.Join(badges, ", ")))
	}
	sb.WriteString("\n")
}

// renderFindingsBySeverity shows findings grouped by severity in collapsible sections.
// Each severity section contains a summary table followed by expandable individual finding details.
func (r *DashboardRenderer) renderFindingsBySeverity(sb *strings.Builder, data review.DashboardData) {
	// Group open findings by severity
	bySeverity := make(map[string][]domain.TrackedFinding)
	for _, f := range data.Findings {
		if f.Status != domain.FindingStatusOpen {
			continue
		}
		bySeverity[f.Finding.Severity] = append(bySeverity[f.Finding.Severity], f)
	}

	if len(bySeverity) == 0 {
		return
	}

	sb.WriteString("### Findings Requiring Attention\n\n")

	severityOrder := []string{"critical", "high", "medium", "low"}
	severityEmoji := map[string]string{
		"critical": "🔴",
		"high":     "🟠",
		"medium":   "🟡",
		"low":      "🟢",
	}

	for _, severity := range severityOrder {
		findings := bySeverity[severity]
		if len(findings) == 0 {
			continue
		}

		// Sort findings by file/line for deterministic output
		sort.Slice(findings, func(i, j int) bool {
			if findings[i].Finding.File != findings[j].Finding.File {
				return findings[i].Finding.File < findings[j].Finding.File
			}
			return findings[i].Finding.LineStart < findings[j].Finding.LineStart
		})

		emoji := severityEmoji[severity]
		title := titleCase(severity)

		// Expand critical and high by default
		openAttr := ""
		if severity == "critical" || severity == "high" {
			openAttr = " open"
		}

		sb.WriteString(fmt.Sprintf("<details%s>\n", openAttr))
		fileCount := countUniqueFiles(findings)
		fileWord := "files"
		if fileCount == 1 {
			fileWord = "file"
		}
		sb.WriteString(fmt.Sprintf("<summary><strong>%s</strong> - %s in <code>%d %s</code></summary>\n\n",
			title, emoji, fileCount, fileWord))

		// Summary table
		sb.WriteString("| File | Line | Category | Description |\n")
		sb.WriteString("|------|------|----------|-------------|\n")
		for _, f := range findings {
			line := fmt.Sprintf("%d", f.Finding.LineStart)
			if f.Finding.LineEnd > f.Finding.LineStart {
				line = fmt.Sprintf("%d-%d", f.Finding.LineStart, f.Finding.LineEnd)
			}
			desc := TruncateDescription(f.Finding.Description, 80)
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n",
				escapeMarkdownInlineCode(f.Finding.File),
				line,
				escapeMarkdownTableCell(f.Finding.Category),
				escapeMarkdownTableCell(desc),
			))
		}

		// Individual expandable findings for full details
		sb.WriteString("\n---\n\n")
		for _, f := range findings {
			renderIndividualFinding(sb, f, emoji)
			sb.WriteString("\n")
		}

		sb.WriteString("</details>\n\n")
	}
}

// countUniqueFiles returns the count of unique files in the findings.
func countUniqueFiles(findings []domain.TrackedFinding) int {
	files := make(map[string]struct{})
	for _, f := range findings {
		files[f.Finding.File] = struct{}{}
	}
	return len(files)
}

// renderResolvedFindings shows resolved findings in a collapsed section with strikethrough.
func (r *DashboardRenderer) renderResolvedFindings(sb *strings.Builder, data review.DashboardData) {
	// Collect resolved findings
	var resolved []domain.TrackedFinding
	for _, f := range data.Findings {
		if f.Status == domain.FindingStatusResolved {
			resolved = append(resolved, f)
		}
	}

	if len(resolved) == 0 {
		return
	}

	// Sort by severity (highest first), then by file/line
	// Unknown severities sort last (rank 99)
	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	getSeverityRank := func(severity string) int {
		if rank, ok := severityOrder[severity]; ok {
			return rank
		}
		return 99 // Unknown severities sort last
	}
	sort.Slice(resolved, func(i, j int) bool {
		si := getSeverityRank(resolved[i].Finding.Severity)
		sj := getSeverityRank(resolved[j].Finding.Severity)
		if si != sj {
			return si < sj
		}
		if resolved[i].Finding.File != resolved[j].Finding.File {
			return resolved[i].Finding.File < resolved[j].Finding.File
		}
		return resolved[i].Finding.LineStart < resolved[j].Finding.LineStart
	})

	// Collapsed section
	sb.WriteString("<details>\n")
	sb.WriteString(fmt.Sprintf("<summary>📋 <strong>Resolved Findings</strong> (%d)</summary>\n\n", len(resolved)))

	// Table with strikethrough for resolved items
	sb.WriteString("| Status | Severity | File | Description |\n")
	sb.WriteString("|--------|----------|------|-------------|\n")
	for _, f := range resolved {
		desc := TruncateDescription(f.Finding.Description, 60)

		// Format resolved info
		resolvedInfo := "Fixed"
		if f.ResolvedIn != nil {
			shortSHA := *f.ResolvedIn
			if len(shortSHA) > 7 {
				shortSHA = shortSHA[:7]
			}
			resolvedInfo = fmt.Sprintf("*in %s*", shortSHA)
		}

		// Strikethrough for resolved items
		sb.WriteString(fmt.Sprintf("| ~~%s~~ | ~~%s~~ | ~~`%s`~~ | ~~%s~~ |\n",
			resolvedInfo,
			escapeMarkdownTableCell(f.Finding.Severity),
			escapeMarkdownInlineCode(f.Finding.File),
			escapeMarkdownTableCell(desc),
		))
	}
	sb.WriteString("\n</details>\n\n")
}

// renderAppendix shows edge cases (findings outside diff, binary files, renames).
func (r *DashboardRenderer) renderAppendix(sb *strings.Builder, data review.DashboardData) {
	if data.Diff == nil || len(data.Diff.Files) == 0 {
		return
	}

	var sections []string

	// Binary files
	var binaryFiles []string
	for _, f := range data.Diff.Files {
		if f.IsBinary {
			binaryFiles = append(binaryFiles, f.Path)
		}
	}
	if len(binaryFiles) > 0 {
		var binarySB strings.Builder
		binarySB.WriteString("<details>\n")
		binarySB.WriteString("<summary>📦 Binary Files Changed</summary>\n\n")
		for _, f := range binaryFiles {
			binarySB.WriteString(fmt.Sprintf("- `%s`\n", escapeMarkdownInlineCode(f)))
		}
		binarySB.WriteString("\n</details>")
		sections = append(sections, binarySB.String())
	}

	// Renamed files
	var renamedFiles []domain.FileDiff
	for _, f := range data.Diff.Files {
		if f.Status == domain.FileStatusRenamed {
			renamedFiles = append(renamedFiles, f)
		}
	}
	if len(renamedFiles) > 0 {
		var renameSB strings.Builder
		renameSB.WriteString("<details>\n")
		renameSB.WriteString("<summary>📝 Files Renamed</summary>\n\n")
		for _, f := range renamedFiles {
			renameSB.WriteString(fmt.Sprintf("- `%s` → `%s`\n",
				escapeMarkdownInlineCode(f.OldPath),
				escapeMarkdownInlineCode(f.Path),
			))
		}
		renameSB.WriteString("\n</details>")
		sections = append(sections, renameSB.String())
	}

	if len(sections) > 0 {
		sb.WriteString(strings.Join(sections, "\n\n"))
		sb.WriteString("\n\n")
	}
}

// renderReviewMetadata shows cost, provider, and timing info.
func (r *DashboardRenderer) renderReviewMetadata(sb *strings.Builder, data review.DashboardData) {
	if data.Review == nil {
		return
	}

	sb.WriteString("<details>\n")
	sb.WriteString("<summary>📊 Review Metadata</summary>\n\n")

	// Provider and model
	sb.WriteString(fmt.Sprintf("- **Provider:** %s\n", data.Review.ProviderName))
	if data.Review.ModelName != "" {
		sb.WriteString(fmt.Sprintf("- **Model:** %s\n", data.Review.ModelName))
	}

	// Cost with appropriate precision
	if data.Review.Cost > 0 {
		costStr := FormatCost(data.Review.Cost)
		sb.WriteString(fmt.Sprintf("- **Cost:** %s\n", costStr))
	}

	sb.WriteString("\n</details>\n\n")
}

// renderReviewedCommits shows the list of reviewed commits in a collapsible section.
func (r *DashboardRenderer) renderReviewedCommits(sb *strings.Builder, data review.DashboardData) {
	if len(data.ReviewedCommits) == 0 {
		return
	}

	sb.WriteString("<details>\n")
	sb.WriteString("<summary>📋 Reviewed Commits</summary>\n\n")

	for _, sha := range data.ReviewedCommits {
		shortSHA := sha
		if len(sha) > 7 {
			shortSHA = sha[:7]
		}
		sb.WriteString(fmt.Sprintf("- `%s`\n", shortSHA))
	}

	sb.WriteString("\n</details>\n\n")
}

// renderLastUpdated shows the last updated timestamp.
func (r *DashboardRenderer) renderLastUpdated(sb *strings.Builder, timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	sb.WriteString(fmt.Sprintf("*Last updated: %s*\n\n", timestamp.Format(time.RFC3339)))
}

// renderInstructions shows how to update finding status via replies.
// Only rendered when there are findings to interact with.
func (r *DashboardRenderer) renderInstructions(sb *strings.Builder, data review.DashboardData) {
	// Don't show instructions if there are no findings
	if len(data.Findings) == 0 {
		return
	}

	sb.WriteString("<details>\n")
	sb.WriteString("<summary>💡 How to Update Finding Status</summary>\n\n")

	sb.WriteString("**Reply to an inline comment** with one of these keywords:\n\n")

	sb.WriteString("| Action | Keywords |\n")
	sb.WriteString("|--------|----------|\n")
	sb.WriteString("| Acknowledge (won't fix, intentional) | `acknowledged`, `won't fix`, `intentional` |\n")
	sb.WriteString("| Dispute (false positive) | `disputed`, `false positive` |\n")
	sb.WriteString("\n")

	sb.WriteString("**Auto-resolution:** Findings are auto-resolved when the code is fixed and ")
	sb.WriteString("the issue is no longer detected in subsequent reviews.\n\n")

	sb.WriteString("</details>\n\n")
}

// embedMetadata embeds the tracking state as base64-encoded JSON.
func (r *DashboardRenderer) embedMetadata(sb *strings.Builder, data review.DashboardData) error {
	// Convert to the tracking state JSON format for persistence
	state := review.TrackingState{
		Target:          data.Target,
		ReviewedCommits: data.ReviewedCommits,
		Findings:        data.Findings,
		LastUpdated:     data.LastUpdated,
		ReviewStatus:    data.ReviewStatus,
	}

	// Serialize to JSON
	jsonBytes, err := json.MarshalIndent(dashboardStateToJSON(state), "", "  ")
	if err != nil {
		return err
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)

	sb.WriteString(dashboardMetadataStart)
	sb.WriteString("\n")
	sb.WriteString(encoded)
	sb.WriteString("\n")
	sb.WriteString(dashboardMetadataEnd)

	return nil
}

// dashboardStateJSON mirrors trackingStateJSON for dashboard metadata.
// We reuse the same schema for compatibility.
type dashboardStateJSON struct {
	Version         int                    `json:"version"`
	Repository      string                 `json:"repository"`
	PRNumber        int                    `json:"pr_number"`
	Branch          string                 `json:"branch"`
	BaseSHA         string                 `json:"base_sha"`
	HeadSHA         string                 `json:"head_sha"`
	ReviewedCommits []string               `json:"reviewed_commits"`
	Findings        []dashboardFindingJSON `json:"findings"`
	LastUpdated     time.Time              `json:"last_updated"`
	ReviewStatus    string                 `json:"review_status,omitempty"`
}

type dashboardFindingJSON struct {
	Fingerprint  string    `json:"fingerprint"`
	Status       string    `json:"status"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	SeenCount    int       `json:"seen_count"`
	StatusReason string    `json:"status_reason,omitempty"`
	ReviewCommit string    `json:"review_commit,omitempty"`
	ResolvedAt   *string   `json:"resolved_at,omitempty"`
	ResolvedIn   *string   `json:"resolved_in,omitempty"`
	FindingID    string    `json:"finding_id"`
	File         string    `json:"file"`
	LineStart    int       `json:"line_start"`
	LineEnd      int       `json:"line_end"`
	Severity     string    `json:"severity"`
	Category     string    `json:"category"`
	Description  string    `json:"description"`
	Suggestion   string    `json:"suggestion"`
	Evidence     bool      `json:"evidence"`
}

// dashboardStateToJSON converts TrackingState to JSON-serializable form.
func dashboardStateToJSON(state review.TrackingState) dashboardStateJSON {
	// Collect and sort fingerprints for deterministic ordering
	fingerprints := make([]string, 0, len(state.Findings))
	for fp := range state.Findings {
		fingerprints = append(fingerprints, string(fp))
	}
	sort.Strings(fingerprints)

	// Build findings slice in sorted order
	findings := make([]dashboardFindingJSON, 0, len(state.Findings))
	for _, fpStr := range fingerprints {
		f := state.Findings[domain.FindingFingerprint(fpStr)]

		// Convert ResolvedAt to RFC3339 string pointer
		var resolvedAtStr *string
		if f.ResolvedAt != nil {
			str := f.ResolvedAt.Format(time.RFC3339)
			resolvedAtStr = &str
		}

		findings = append(findings, dashboardFindingJSON{
			Fingerprint:  string(f.Fingerprint),
			Status:       string(f.Status),
			FirstSeen:    f.FirstSeen,
			LastSeen:     f.LastSeen,
			SeenCount:    f.SeenCount,
			StatusReason: f.StatusReason,
			ReviewCommit: f.ReviewCommit,
			ResolvedAt:   resolvedAtStr,
			ResolvedIn:   f.ResolvedIn,
			FindingID:    f.Finding.ID,
			File:         f.Finding.File,
			LineStart:    f.Finding.LineStart,
			LineEnd:      f.Finding.LineEnd,
			Severity:     f.Finding.Severity,
			Category:     f.Finding.Category,
			Description:  f.Finding.Description,
			Suggestion:   f.Finding.Suggestion,
			Evidence:     f.Finding.Evidence,
		})
	}

	return dashboardStateJSON{
		Version:         1,
		Repository:      state.Target.Repository,
		PRNumber:        state.Target.PRNumber,
		Branch:          state.Target.Branch,
		BaseSHA:         state.Target.BaseSHA,
		HeadSHA:         state.Target.HeadSHA,
		ReviewedCommits: state.ReviewedCommits,
		Findings:        findings,
		LastUpdated:     state.LastUpdated,
		ReviewStatus:    string(state.ReviewStatus),
	}
}

// titleCase converts a string to title case (first letter uppercase).
// This is a simple ASCII-only implementation for severity strings.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// renderIndividualFinding renders an expandable detail block for a single finding.
// This provides full finding details when expanded, enabling incremental disclosure.
func renderIndividualFinding(sb *strings.Builder, f domain.TrackedFinding, emoji string) {
	file := f.Finding.File
	line := fmt.Sprintf("%d", f.Finding.LineStart)
	if f.Finding.LineEnd > f.Finding.LineStart {
		line = fmt.Sprintf("%d-%d", f.Finding.LineStart, f.Finding.LineEnd)
	}

	// Summary line: file:line - truncated description
	summaryDesc := TruncateDescription(f.Finding.Description, 50)
	sb.WriteString("<details>\n")
	sb.WriteString(fmt.Sprintf("<summary>%s <code>%s:%s</code> - %s</summary>\n\n",
		emoji,
		escapeMarkdownInlineCode(file),
		line,
		escapeMarkdownTableCell(summaryDesc),
	))

	// Full details inside
	sb.WriteString(fmt.Sprintf("**Category:** %s\n\n", f.Finding.Category))
	sb.WriteString(fmt.Sprintf("**Lines:** %s\n\n", line))
	sb.WriteString(f.Finding.Description)
	sb.WriteString("\n\n")

	if f.Finding.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("**Suggestion:** %s\n\n", f.Finding.Suggestion))
	}

	sb.WriteString("</details>\n")
}

// TruncateDescription truncates a description to the specified length.
// If the description exceeds maxLen, it's truncated with "..." appended.
// Uses rune-based truncation to handle multi-byte UTF-8 characters correctly.
func TruncateDescription(desc string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(desc)
	if len(runes) <= maxLen {
		return desc
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// FormatCost formats a cost value with appropriate precision.
// Uses 4 decimal places for small costs (< $0.10) to show accurate API pricing,
// 3 decimal places for medium costs ($0.10-$0.99), and 2 for larger amounts.
func FormatCost(cost float64) string {
	if cost >= 1.0 {
		return fmt.Sprintf("$%.2f", cost)
	} else if cost >= 0.1 {
		return fmt.Sprintf("$%.3f", cost)
	}
	return fmt.Sprintf("$%.4f", cost)
}

// BuildReviewPointer creates a one-line review body that points to the dashboard.
// This is used as the GitHub review body instead of the full summary.
func BuildReviewPointer(dashboardURL string) string {
	if dashboardURL == "" {
		return "Code review complete. See the tracking comment for details."
	}
	return fmt.Sprintf("See the [Code Review Dashboard](%s) for full details.", dashboardURL)
}
