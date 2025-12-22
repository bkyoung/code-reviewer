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
