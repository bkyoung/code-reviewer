package markdown_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brandon/code-reviewer/internal/adapter/output/markdown"
	"github.com/brandon/code-reviewer/internal/domain"
)

func TestWriterProducesDeterministicMarkdown(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	writer := markdown.NewWriter(func() string {
		return "2025-01-01T00-00-00Z"
	})

	reviewData := domain.Review{
		ProviderName: "stub-openai",
		ModelName:    "gpt-4o",
		Summary:      "Summary text",
		Findings: []domain.Finding{
			{
				ID:          "id",
				File:        "main.go",
				LineStart:   10,
				LineEnd:     12,
				Severity:    "medium",
				Category:    "bug",
				Description: "Bug description",
				Suggestion:  "Fix it",
				Evidence:    true,
			},
		},
	}

	path, err := writer.Write(ctx, domain.MarkdownArtifact{
		OutputDir:    dir,
		Repository:   "repo",
		BaseRef:      "master",
		TargetRef:    "feature",
		Diff:         domain.Diff{},
		Review:       reviewData,
		ProviderName: "stub-openai",
	})
	if err != nil {
		t.Fatalf("writer returned error: %v", err)
	}

	if filepath.Base(path) != "repo_feature_stub-openai_2025-01-01T00-00-00Z.md" {
		t.Fatalf("unexpected filename: %s", filepath.Base(path))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "Summary text") {
		t.Fatalf("markdown missing summary: %s", string(content))
	}
}

func TestWriterIncludesCostInformation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	writer := markdown.NewWriter(func() string {
		return "2025-01-01T00-00-00Z"
	})

	reviewData := domain.Review{
		ProviderName: "openai",
		ModelName:    "gpt-4o",
		Summary:      "Review summary",
		Cost:         0.0523, // $0.0523
		Findings:     []domain.Finding{},
	}

	path, err := writer.Write(ctx, domain.MarkdownArtifact{
		OutputDir:    dir,
		Repository:   "test-repo",
		BaseRef:      "main",
		TargetRef:    "feature",
		Diff:         domain.Diff{},
		Review:       reviewData,
		ProviderName: "openai",
	})
	if err != nil {
		t.Fatalf("writer returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	contentStr := string(content)

	// Verify cost is included with correct formatting
	if !strings.Contains(contentStr, "Cost: $0.0523") {
		t.Errorf("markdown missing cost information: %s", contentStr)
	}

	// Test zero cost case
	reviewData.Cost = 0.0
	path2, err := writer.Write(ctx, domain.MarkdownArtifact{
		OutputDir:    dir,
		Repository:   "test-repo",
		BaseRef:      "main",
		TargetRef:    "feature",
		Diff:         domain.Diff{},
		Review:       reviewData,
		ProviderName: "openai",
	})
	if err != nil {
		t.Fatalf("writer returned error: %v", err)
	}

	content2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Zero cost should show as $0.00
	if !strings.Contains(string(content2), "Cost: $0.00") {
		t.Errorf("markdown missing zero cost: %s", string(content2))
	}
}
