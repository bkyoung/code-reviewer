package github_test

import (
	"strings"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/diff"
	"github.com/bkyoung/code-reviewer/internal/domain"
)

func TestBuildSummaryAppendix_NoEdgeCases(t *testing.T) {
	findings := []github.PositionedFinding{
		{
			Finding:      domain.Finding{ID: "f1", File: "main.go", LineStart: 10},
			DiffPosition: diff.IntPtr(5), // In diff
		},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
		},
	}

	result := github.BuildSummaryAppendix(findings, d)

	// No appendix needed when all findings are in diff and no binary/renamed files
	if result != "" {
		t.Errorf("expected empty appendix, got %q", result)
	}
}

func TestBuildSummaryAppendix_OutOfDiffFindings(t *testing.T) {
	findings := []github.PositionedFinding{
		{
			Finding:      domain.Finding{ID: "f1", File: "main.go", LineStart: 10, Severity: "high", Description: "Security issue"},
			DiffPosition: nil, // Out of diff
		},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
		},
	}

	result := github.BuildSummaryAppendix(findings, d)

	if !strings.Contains(result, "Findings Outside Diff") {
		t.Errorf("expected 'Findings Outside Diff' section, got %q", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected file name in appendix, got %q", result)
	}
	if !strings.Contains(result, "high") {
		t.Errorf("expected severity in appendix, got %q", result)
	}
}

func TestBuildSummaryAppendix_BinaryFiles(t *testing.T) {
	findings := []github.PositionedFinding{}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "image.png", Status: domain.FileStatusModified, IsBinary: true},
			{Path: "data.bin", Status: domain.FileStatusAdded, IsBinary: true},
		},
	}

	result := github.BuildSummaryAppendix(findings, d)

	if !strings.Contains(result, "Binary Files Changed") {
		t.Errorf("expected 'Binary Files Changed' section, got %q", result)
	}
	if !strings.Contains(result, "image.png") {
		t.Errorf("expected 'image.png' in appendix, got %q", result)
	}
	if !strings.Contains(result, "data.bin") {
		t.Errorf("expected 'data.bin' in appendix, got %q", result)
	}
}

func TestBuildSummaryAppendix_RenamedFiles(t *testing.T) {
	findings := []github.PositionedFinding{}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "new_name.go", OldPath: "old_name.go", Status: domain.FileStatusRenamed},
		},
	}

	result := github.BuildSummaryAppendix(findings, d)

	if !strings.Contains(result, "Files Renamed") {
		t.Errorf("expected 'Files Renamed' section, got %q", result)
	}
	if !strings.Contains(result, "old_name.go") {
		t.Errorf("expected old path in appendix, got %q", result)
	}
	if !strings.Contains(result, "new_name.go") {
		t.Errorf("expected new path in appendix, got %q", result)
	}
}

func TestBuildSummaryAppendix_AllEdgeCases(t *testing.T) {
	findings := []github.PositionedFinding{
		{
			Finding:      domain.Finding{ID: "f1", File: "main.go", LineStart: 100, Severity: "medium", Description: "Deleted line issue"},
			DiffPosition: nil, // Out of diff
		},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
			{Path: "image.png", Status: domain.FileStatusModified, IsBinary: true},
			{Path: "new_name.go", OldPath: "old_name.go", Status: domain.FileStatusRenamed},
		},
	}

	result := github.BuildSummaryAppendix(findings, d)

	// Should contain all three sections
	if !strings.Contains(result, "Findings Outside Diff") {
		t.Errorf("expected 'Findings Outside Diff' section")
	}
	if !strings.Contains(result, "Binary Files Changed") {
		t.Errorf("expected 'Binary Files Changed' section")
	}
	if !strings.Contains(result, "Files Renamed") {
		t.Errorf("expected 'Files Renamed' section")
	}
}

func TestAppendSections_EmptyAppendix(t *testing.T) {
	original := "This is the original summary."
	appendix := ""

	result := github.AppendSections(original, appendix)

	if result != original {
		t.Errorf("expected original summary unchanged, got %q", result)
	}
}

func TestAppendSections_WithAppendix(t *testing.T) {
	original := "This is the original summary."
	appendix := "\n\n---\n\n## Test Section\n\nContent here."

	result := github.AppendSections(original, appendix)

	if !strings.HasPrefix(result, original) {
		t.Errorf("expected result to start with original summary")
	}
	if !strings.Contains(result, "Test Section") {
		t.Errorf("expected appendix to be included")
	}
}

func TestFilterOutOfDiff(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1"}, DiffPosition: diff.IntPtr(5)},  // In diff
		{Finding: domain.Finding{ID: "f2"}, DiffPosition: nil},             // Out of diff
		{Finding: domain.Finding{ID: "f3"}, DiffPosition: diff.IntPtr(10)}, // In diff
		{Finding: domain.Finding{ID: "f4"}, DiffPosition: nil},             // Out of diff
	}

	result := github.FilterOutOfDiff(findings)

	if len(result) != 2 {
		t.Fatalf("expected 2 out-of-diff findings, got %d", len(result))
	}
	if result[0].Finding.ID != "f2" || result[1].Finding.ID != "f4" {
		t.Errorf("unexpected findings: %v", result)
	}
}

func TestFilterBinaryFiles(t *testing.T) {
	files := []domain.FileDiff{
		{Path: "text.go", IsBinary: false},
		{Path: "image.png", IsBinary: true},
		{Path: "another.go", IsBinary: false},
		{Path: "data.bin", IsBinary: true},
	}

	result := github.FilterBinaryFiles(files)

	if len(result) != 2 {
		t.Fatalf("expected 2 binary files, got %d", len(result))
	}
	if result[0].Path != "image.png" || result[1].Path != "data.bin" {
		t.Errorf("unexpected files: %v", result)
	}
}

func TestFilterRenamedFiles(t *testing.T) {
	files := []domain.FileDiff{
		{Path: "unchanged.go", Status: domain.FileStatusModified},
		{Path: "new.go", OldPath: "old.go", Status: domain.FileStatusRenamed},
		{Path: "added.go", Status: domain.FileStatusAdded},
		{Path: "new2.go", OldPath: "old2.go", Status: domain.FileStatusRenamed},
	}

	result := github.FilterRenamedFiles(files)

	if len(result) != 2 {
		t.Fatalf("expected 2 renamed files, got %d", len(result))
	}
	if result[0].Path != "new.go" || result[1].Path != "new2.go" {
		t.Errorf("unexpected files: %v", result)
	}
}

// =============================================================================
// BuildProgrammaticSummary Tests
// =============================================================================

func TestBuildProgrammaticSummary_CleanCode(t *testing.T) {
	findings := []github.PositionedFinding{}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "main.go", Status: domain.FileStatusModified},
			{Path: "util.go", Status: domain.FileStatusModified},
			{Path: "handler.go", Status: domain.FileStatusAdded},
		},
	}
	actions := github.ReviewActions{}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	if !strings.Contains(result, "No issues found") {
		t.Errorf("expected 'No issues found' message, got %q", result)
	}
	if !strings.Contains(result, "3 files") {
		t.Errorf("expected '3 files' count, got %q", result)
	}
}

func TestBuildProgrammaticSummary_BadgeLine(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", File: "a.go", Severity: "critical", Category: "security"}, DiffPosition: diff.IntPtr(1)},
		{Finding: domain.Finding{ID: "f2", File: "b.go", Severity: "critical", Category: "security"}, DiffPosition: diff.IntPtr(2)},
		{Finding: domain.Finding{ID: "f3", File: "c.go", Severity: "high", Category: "bug"}, DiffPosition: diff.IntPtr(3)},
		{Finding: domain.Finding{ID: "f4", File: "d.go", Severity: "medium", Category: "style"}, DiffPosition: diff.IntPtr(4)},
		{Finding: domain.Finding{ID: "f5", File: "e.go", Severity: "medium", Category: "style"}, DiffPosition: diff.IntPtr(5)},
		{Finding: domain.Finding{ID: "f6", File: "f.go", Severity: "medium", Category: "performance"}, DiffPosition: diff.IntPtr(6)},
		{Finding: domain.Finding{ID: "f7", File: "g.go", Severity: "low", Category: "style"}, DiffPosition: diff.IntPtr(7)},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}, {Path: "d.go"},
			{Path: "e.go"}, {Path: "f.go"}, {Path: "g.go"}, {Path: "h.go"},
		},
	}
	actions := github.ReviewActions{}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Check badge line format
	if !strings.Contains(result, "8 files") {
		t.Errorf("expected '8 files' in badge line, got %q", result)
	}
	if !strings.Contains(result, "2 critical") {
		t.Errorf("expected '2 critical' in badge line, got %q", result)
	}
	if !strings.Contains(result, "1 high") {
		t.Errorf("expected '1 high' in badge line, got %q", result)
	}
	if !strings.Contains(result, "3 medium") {
		t.Errorf("expected '3 medium' in badge line, got %q", result)
	}
	if !strings.Contains(result, "1 low") {
		t.Errorf("expected '1 low' in badge line, got %q", result)
	}
}

func TestBuildProgrammaticSummary_FilesRequiringAttention_DefaultActions(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", File: "auth/handler.go", Severity: "critical"}, DiffPosition: diff.IntPtr(1)},
		{Finding: domain.Finding{ID: "f2", File: "auth/handler.go", Severity: "high"}, DiffPosition: diff.IntPtr(2)},
		{Finding: domain.Finding{ID: "f3", File: "db/query.go", Severity: "high"}, DiffPosition: diff.IntPtr(3)},
		{Finding: domain.Finding{ID: "f4", File: "util/helper.go", Severity: "medium"}, DiffPosition: diff.IntPtr(4)},
		{Finding: domain.Finding{ID: "f5", File: "util/helper.go", Severity: "low"}, DiffPosition: diff.IntPtr(5)},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{{Path: "auth/handler.go"}, {Path: "db/query.go"}, {Path: "util/helper.go"}},
	}
	// Empty actions = defaults (critical/high trigger request_changes)
	actions := github.ReviewActions{}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Should include Files Requiring Attention section
	if !strings.Contains(result, "Files Requiring Attention") {
		t.Errorf("expected 'Files Requiring Attention' section, got %q", result)
	}
	// Should include auth/handler.go (critical + high)
	if !strings.Contains(result, "auth/handler.go") {
		t.Errorf("expected 'auth/handler.go' in attention section, got %q", result)
	}
	// Should include db/query.go (high)
	if !strings.Contains(result, "db/query.go") {
		t.Errorf("expected 'db/query.go' in attention section, got %q", result)
	}
	// Should NOT include util/helper.go (only medium/low)
	if strings.Contains(result, "util/helper.go") {
		t.Errorf("expected 'util/helper.go' to NOT be in attention section (only medium/low), got %q", result)
	}
}

func TestBuildProgrammaticSummary_FilesRequiringAttention_CustomActions(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", File: "a.go", Severity: "critical"}, DiffPosition: diff.IntPtr(1)},
		{Finding: domain.Finding{ID: "f2", File: "b.go", Severity: "high"}, DiffPosition: diff.IntPtr(2)},
		{Finding: domain.Finding{ID: "f3", File: "c.go", Severity: "medium"}, DiffPosition: diff.IntPtr(3)},
		{Finding: domain.Finding{ID: "f4", File: "d.go", Severity: "low"}, DiffPosition: diff.IntPtr(4)},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}, {Path: "d.go"}},
	}
	// Custom: medium also triggers request_changes
	actions := github.ReviewActions{
		OnCritical: "request_changes",
		OnHigh:     "request_changes",
		OnMedium:   "request_changes",
		OnLow:      "comment",
	}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Should include a.go, b.go, c.go (critical, high, medium)
	if !strings.Contains(result, "a.go") {
		t.Errorf("expected 'a.go' in attention section, got %q", result)
	}
	if !strings.Contains(result, "b.go") {
		t.Errorf("expected 'b.go' in attention section, got %q", result)
	}
	if !strings.Contains(result, "c.go") {
		t.Errorf("expected 'c.go' in attention section, got %q", result)
	}
	// Should NOT include d.go (low = comment)
	// Note: d.go might appear elsewhere, check attention section specifically
	attentionSection := extractSection(result, "Files Requiring Attention")
	if strings.Contains(attentionSection, "d.go") {
		t.Errorf("expected 'd.go' to NOT be in attention section, got %q", attentionSection)
	}
}

func TestBuildProgrammaticSummary_NoAttentionSection_WhenNoBlockingSeverities(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", File: "a.go", Severity: "medium"}, DiffPosition: diff.IntPtr(1)},
		{Finding: domain.Finding{ID: "f2", File: "b.go", Severity: "low"}, DiffPosition: diff.IntPtr(2)},
	}
	d := domain.Diff{
		Files: []domain.FileDiff{{Path: "a.go"}, {Path: "b.go"}},
	}
	actions := github.ReviewActions{} // Default: only critical/high block

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Should NOT include Files Requiring Attention section
	if strings.Contains(result, "Files Requiring Attention") {
		t.Errorf("expected NO 'Files Requiring Attention' section when only medium/low findings, got %q", result)
	}
}

func TestBuildProgrammaticSummary_CategoryTable(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", Severity: "high", Category: "security"}, DiffPosition: diff.IntPtr(1)},
		{Finding: domain.Finding{ID: "f2", Severity: "high", Category: "security"}, DiffPosition: diff.IntPtr(2)},
		{Finding: domain.Finding{ID: "f3", Severity: "medium", Category: "bug"}, DiffPosition: diff.IntPtr(3)},
		{Finding: domain.Finding{ID: "f4", Severity: "low", Category: "style"}, DiffPosition: diff.IntPtr(4)},
		{Finding: domain.Finding{ID: "f5", Severity: "low", Category: "style"}, DiffPosition: diff.IntPtr(5)},
		{Finding: domain.Finding{ID: "f6", Severity: "low", Category: "style"}, DiffPosition: diff.IntPtr(6)},
	}
	d := domain.Diff{Files: []domain.FileDiff{{Path: "a.go"}}}
	actions := github.ReviewActions{}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Should include category table
	if !strings.Contains(result, "Findings by Category") {
		t.Errorf("expected 'Findings by Category' section, got %q", result)
	}
	if !strings.Contains(result, "security") {
		t.Errorf("expected 'security' category, got %q", result)
	}
	if !strings.Contains(result, "bug") {
		t.Errorf("expected 'bug' category, got %q", result)
	}
	if !strings.Contains(result, "style") {
		t.Errorf("expected 'style' category, got %q", result)
	}
}

func TestBuildProgrammaticSummary_EmptyCategory(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", Severity: "high", Category: ""}, DiffPosition: diff.IntPtr(1)},
		{Finding: domain.Finding{ID: "f2", Severity: "medium", Category: ""}, DiffPosition: diff.IntPtr(2)},
	}
	d := domain.Diff{Files: []domain.FileDiff{{Path: "a.go"}}}
	actions := github.ReviewActions{}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Empty categories should be grouped as "general" or similar
	if !strings.Contains(result, "Findings by Category") {
		t.Errorf("expected 'Findings by Category' section even with empty categories, got %q", result)
	}
}

func TestBuildProgrammaticSummary_OutOfDiffFindingsNotCounted(t *testing.T) {
	findings := []github.PositionedFinding{
		{Finding: domain.Finding{ID: "f1", File: "a.go", Severity: "critical"}, DiffPosition: diff.IntPtr(1)}, // In diff
		{Finding: domain.Finding{ID: "f2", File: "b.go", Severity: "critical"}, DiffPosition: nil},            // Out of diff
		{Finding: domain.Finding{ID: "f3", File: "c.go", Severity: "high"}, DiffPosition: nil},                // Out of diff
	}
	d := domain.Diff{Files: []domain.FileDiff{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}}}
	actions := github.ReviewActions{}

	result := github.BuildProgrammaticSummary(findings, d, actions)

	// Badge should only count in-diff findings
	if !strings.Contains(result, "1 critical") {
		t.Errorf("expected '1 critical' (only in-diff), got %q", result)
	}
	// Out-of-diff findings should not appear in Files Requiring Attention
	attentionSection := extractSection(result, "Files Requiring Attention")
	if strings.Contains(attentionSection, "b.go") {
		t.Errorf("expected 'b.go' (out of diff) to NOT be in attention section, got %q", attentionSection)
	}
}

// extractSection extracts a section from markdown by header name.
// Returns empty string if section not found.
func extractSection(markdown, headerName string) string {
	lines := strings.Split(markdown, "\n")
	var inSection bool
	var section strings.Builder

	for _, line := range lines {
		if strings.Contains(line, headerName) {
			inSection = true
			continue
		}
		if inSection {
			// Stop at next header
			if strings.HasPrefix(line, "###") || strings.HasPrefix(line, "## ") {
				break
			}
			section.WriteString(line)
			section.WriteString("\n")
		}
	}

	return section.String()
}
