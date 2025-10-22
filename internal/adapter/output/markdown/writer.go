package markdown

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

type clock func() string

// Writer renders provider reviews into Markdown files.
type Writer struct {
	now clock
}

// NewWriter constructs a Markdown writer with a timestamp supplier.
func NewWriter(now clock) *Writer {
	return &Writer{now: now}
}

// Write persists a Markdown artifact to disk.
func (w *Writer) Write(ctx context.Context, artifact domain.MarkdownArtifact) (string, error) {
	if err := os.MkdirAll(artifact.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s_%s.md",
		sanitise(artifact.Repository),
		sanitise(artifact.TargetRef),
		sanitise(artifact.ProviderName),
		w.now(),
	)
	path := filepath.Join(artifact.OutputDir, filename)

	content := buildContent(artifact)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write markdown: %w", err)
	}

	return path, nil
}

func buildContent(artifact domain.MarkdownArtifact) string {
	var builder strings.Builder
	caser := cases.Title(language.English)
	builder.WriteString("# Code Review Report\n\n")
	builder.WriteString(fmt.Sprintf("- Provider: %s (%s)\n", artifact.Review.ProviderName, artifact.Review.ModelName))
	builder.WriteString(fmt.Sprintf("- Base: %s\n", artifact.BaseRef))
	builder.WriteString(fmt.Sprintf("- Target: %s\n", artifact.TargetRef))
	builder.WriteString(fmt.Sprintf("- Cost: $%.4f\n\n", artifact.Review.Cost))
	builder.WriteString("## Summary\n\n")
	builder.WriteString(artifact.Review.Summary)
	builder.WriteString("\n\n")

	if len(artifact.Review.Findings) == 0 {
		builder.WriteString("No findings reported.\n")
		return builder.String()
	}

	builder.WriteString("## Findings\n\n")
	for _, finding := range artifact.Review.Findings {
		builder.WriteString(fmt.Sprintf("### %s (%s)\n", finding.Description, caser.String(finding.Severity)))
		builder.WriteString(fmt.Sprintf("- File: %s:%d-%d\n", finding.File, finding.LineStart, finding.LineEnd))
		builder.WriteString(fmt.Sprintf("- Category: %s\n", finding.Category))
		builder.WriteString(fmt.Sprintf("- Suggestion: %s\n", finding.Suggestion))
		if finding.Evidence {
			builder.WriteString("- Evidence: Provided\n")
		} else {
			builder.WriteString("- Evidence: Not provided\n")
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

func sanitise(value string) string {
	if value == "" {
		return "unknown"
	}
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}
