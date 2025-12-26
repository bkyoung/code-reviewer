package review

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// TokenEstimator estimates the token count for a given text.
// This is used by size guards to determine when truncation is needed.
type TokenEstimator interface {
	EstimateTokens(text string) int
}

// TruncationResult captures the outcome of size guard processing.
// This information is included in the review output to warn users
// about potentially incomplete reviews due to size limits.
type TruncationResult struct {
	// WasWarned indicates if the prompt exceeded the warning threshold.
	WasWarned bool

	// WasTruncated indicates if files were removed to fit within limits.
	WasTruncated bool

	// OriginalTokens is the estimated token count before truncation.
	OriginalTokens int

	// FinalTokens is the estimated token count after truncation.
	FinalTokens int

	// RemovedFiles lists files that were removed during truncation.
	RemovedFiles []string

	// TruncationNote is a human-readable message about what was truncated.
	TruncationNote string
}

// SizeGuardLimits specifies the token thresholds for size guards.
type SizeGuardLimits struct {
	WarnTokens int
	MaxTokens  int
}

// EnhancedPromptBuilder builds prompts with rich context and provider-specific templates.
type EnhancedPromptBuilder struct {
	providerTemplates map[string]string // Provider-specific templates
	defaultTemplate   string            // Fallback template
}

// NewEnhancedPromptBuilder creates a new enhanced prompt builder with default templates.
func NewEnhancedPromptBuilder() *EnhancedPromptBuilder {
	return &EnhancedPromptBuilder{
		providerTemplates: make(map[string]string),
		defaultTemplate:   defaultPromptTemplate(),
	}
}

// SetProviderTemplate sets a custom template for a specific provider.
func (b *EnhancedPromptBuilder) SetProviderTemplate(providerName, templateText string) {
	b.providerTemplates[providerName] = templateText
}

// Build constructs a provider request with enhanced context.
func (b *EnhancedPromptBuilder) Build(
	context ProjectContext,
	diff domain.Diff,
	req BranchRequest,
	providerName string,
) (ProviderRequest, error) {
	// Select template for provider
	templateText := b.defaultTemplate
	if providerTemplate, ok := b.providerTemplates[providerName]; ok {
		templateText = providerTemplate
	}

	// Render template
	prompt, err := b.renderTemplate(templateText, context, diff, req)
	if err != nil {
		return ProviderRequest{}, fmt.Errorf("failed to render template: %w", err)
	}

	return ProviderRequest{
		Prompt:  prompt,
		MaxSize: defaultMaxTokens,
	}, nil
}

// BuildWithSizeGuards constructs a provider request with size guard enforcement.
// If the prompt exceeds limits, files are truncated by priority (docs first, source last).
func (b *EnhancedPromptBuilder) BuildWithSizeGuards(
	context ProjectContext,
	diff domain.Diff,
	req BranchRequest,
	providerName string,
	estimator TokenEstimator,
	limits SizeGuardLimits,
) (ProviderRequest, TruncationResult, error) {
	// Select template for provider
	templateText := b.defaultTemplate
	if providerTemplate, ok := b.providerTemplates[providerName]; ok {
		templateText = providerTemplate
	}

	// Build initial prompt to estimate size
	prompt, err := b.renderTemplate(templateText, context, diff, req)
	if err != nil {
		return ProviderRequest{}, TruncationResult{}, fmt.Errorf("failed to render template: %w", err)
	}

	originalTokens := estimator.EstimateTokens(prompt)
	result := TruncationResult{
		OriginalTokens: originalTokens,
		FinalTokens:    originalTokens,
	}

	// Check if warning threshold exceeded
	if originalTokens >= limits.WarnTokens {
		result.WasWarned = true
	}

	// Check if truncation is needed
	if originalTokens <= limits.MaxTokens {
		// No truncation needed
		return ProviderRequest{
			Prompt:  prompt,
			MaxSize: defaultMaxTokens,
		}, result, nil
	}

	// Truncation needed - remove files by priority until under limit
	truncatedDiff, removedFiles := b.truncateDiff(
		diff,
		context,
		req,
		templateText,
		estimator,
		limits.MaxTokens,
	)

	// Re-render with truncated diff
	prompt, err = b.renderTemplate(templateText, context, truncatedDiff, req)
	if err != nil {
		return ProviderRequest{}, TruncationResult{}, fmt.Errorf("failed to render truncated template: %w", err)
	}

	finalTokens := estimator.EstimateTokens(prompt)

	result.WasTruncated = len(removedFiles) > 0
	result.FinalTokens = finalTokens
	result.RemovedFiles = removedFiles

	if result.WasTruncated {
		result.TruncationNote = fmt.Sprintf(
			"PR size (%d tokens) exceeded limit (%d tokens). Removed %d file(s) for review: %s. "+
				"The review may be incomplete. Consider splitting this PR into smaller changes.",
			originalTokens,
			limits.MaxTokens,
			len(removedFiles),
			strings.Join(removedFiles, ", "),
		)
	}

	return ProviderRequest{
		Prompt:  prompt,
		MaxSize: defaultMaxTokens,
	}, result, nil
}

// truncateDiff removes files by priority until the prompt fits within maxTokens.
// Removal priority (docs removed first, source code last):
// - Priority 4: Documentation (.md, .rst, .txt, docs/)
// - Priority 3: Build/CI files (Dockerfile, Makefile, .github/, ci)
// - Priority 2: Configuration files (.yaml, .yml, .json, .toml, etc.)
// - Priority 1: Test files (files containing "test" or "spec")
// - Priority 0: Source code (highest priority to keep)
func (b *EnhancedPromptBuilder) truncateDiff(
	diff domain.Diff,
	context ProjectContext,
	req BranchRequest,
	templateText string,
	estimator TokenEstimator,
	maxTokens int,
) (domain.Diff, []string) {
	// Sort files by removal priority (highest priority to remove first)
	type prioritizedFile struct {
		file     domain.FileDiff
		priority int
		index    int
	}

	files := make([]prioritizedFile, len(diff.Files))
	for i, f := range diff.Files {
		files[i] = prioritizedFile{
			file:     f,
			priority: fileTypePriority(f.Path),
			index:    i,
		}
	}

	// Sort by priority descending (higher priority = remove first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].priority > files[j].priority
	})

	var removedFiles []string
	currentFiles := make([]domain.FileDiff, len(diff.Files))
	copy(currentFiles, diff.Files)

	for len(files) > 0 {
		// Try building prompt with current files
		testDiff := domain.Diff{
			FromCommitHash: diff.FromCommitHash,
			ToCommitHash:   diff.ToCommitHash,
			Files:          currentFiles,
		}

		prompt, err := b.renderTemplate(templateText, context, testDiff, req)
		if err != nil {
			// If we can't render, return what we have
			break
		}

		tokens := estimator.EstimateTokens(prompt)
		if tokens <= maxTokens {
			// We're under limit, done
			return testDiff, removedFiles
		}

		// Remove the file with highest removal priority (first in sorted list)
		fileToRemove := files[0]
		files = files[1:]

		// Remove from currentFiles
		newFiles := make([]domain.FileDiff, 0, len(currentFiles)-1)
		for _, f := range currentFiles {
			if f.Path != fileToRemove.file.Path {
				newFiles = append(newFiles, f)
			}
		}
		currentFiles = newFiles
		removedFiles = append(removedFiles, fileToRemove.file.Path)

		// Avoid removing all files
		if len(currentFiles) == 0 {
			break
		}
	}

	return domain.Diff{
		FromCommitHash: diff.FromCommitHash,
		ToCommitHash:   diff.ToCommitHash,
		Files:          currentFiles,
	}, removedFiles
}

// TemplateData holds all data available to templates.
type TemplateData struct {
	// Context fields
	Architecture       string
	README             string
	DesignDocs         string // Concatenated design docs
	CustomInstructions string
	CustomContext      string // User-provided files
	RelevantDocs       string // Concatenated relevant docs
	ChangeTypes        []string
	ChangedPaths       []string

	// Request fields
	BaseRef   string
	TargetRef string

	// Diff content
	Diff string
}

// renderTemplate renders a prompt template with context and diff.
func (b *EnhancedPromptBuilder) renderTemplate(
	templateText string,
	context ProjectContext,
	diff domain.Diff,
	req BranchRequest,
) (string, error) {
	// Prepare template data
	data := TemplateData{
		Architecture:       context.Architecture,
		README:             context.README,
		DesignDocs:         strings.Join(context.DesignDocs, "\n\n"),
		CustomInstructions: context.CustomInstructions,
		CustomContext:      strings.Join(context.CustomContextFiles, "\n\n"),
		RelevantDocs:       strings.Join(context.RelevantDocs, "\n\n"),
		ChangeTypes:        context.ChangeTypes,
		ChangedPaths:       context.ChangedPaths,
		BaseRef:            req.BaseRef,
		TargetRef:          req.TargetRef,
		Diff:               b.formatDiff(diff),
	}

	// Create template with custom functions
	tmpl, err := template.New("prompt").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// formatDiff converts a domain.Diff into a human-readable string.
// Files are sorted with source code first and documentation last to ensure
// the LLM prioritizes code review over documentation review.
func (b *EnhancedPromptBuilder) formatDiff(diff domain.Diff) string {
	if len(diff.Files) == 0 {
		return "(no changes)"
	}

	// Sort files: source code first, then config, then documentation
	sortedFiles := make([]domain.FileDiff, len(diff.Files))
	copy(sortedFiles, diff.Files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return fileTypePriority(sortedFiles[i].Path) < fileTypePriority(sortedFiles[j].Path)
	})

	var buf bytes.Buffer

	for _, file := range sortedFiles {
		buf.WriteString(fmt.Sprintf("File: %s (%s)\n", file.Path, file.Status))
		if file.Patch != "" {
			buf.WriteString(file.Patch)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// fileTypePriority returns a priority value for sorting files.
// Lower values appear first in the diff (higher priority for review).
func fileTypePriority(path string) int {
	lowerPath := strings.ToLower(path)

	// Priority 0: Source code files (highest priority)
	sourceExtensions := []string{".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".rs", ".java", ".c", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".kt", ".scala"}
	for _, ext := range sourceExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return 0
		}
	}

	// Priority 1: Test files (still important code)
	if strings.Contains(lowerPath, "test") || strings.Contains(lowerPath, "spec") {
		return 1
	}

	// Priority 2: Configuration files
	configExtensions := []string{".yaml", ".yml", ".json", ".toml", ".ini", ".env", ".conf"}
	for _, ext := range configExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return 2
		}
	}

	// Priority 3: Build/CI files
	if strings.Contains(lowerPath, "dockerfile") || strings.Contains(lowerPath, "makefile") ||
		strings.Contains(lowerPath, ".github/") || strings.Contains(lowerPath, "ci") {
		return 3
	}

	// Priority 4: Documentation (lowest priority)
	if strings.HasSuffix(lowerPath, ".md") || strings.HasSuffix(lowerPath, ".rst") ||
		strings.HasSuffix(lowerPath, ".txt") || strings.Contains(lowerPath, "docs/") {
		return 4
	}

	// Default priority for unknown file types
	return 3
}

// defaultPromptTemplate returns the default template used when no provider-specific template is set.
// IMPORTANT: Code diff appears FIRST to ensure LLM prioritizes code review over documentation.
// LLMs exhibit primacy bias - they weight early content more heavily.
func defaultPromptTemplate() string {
	return `You are an expert software engineer performing a code review.
Your PRIMARY task is to review the CODE CHANGES below for bugs, security issues, and improvements.

## Code Changes to Review (PRIMARY FOCUS)

Base Ref: {{.BaseRef}}
Target Ref: {{.TargetRef}}
{{if .ChangeTypes}}Change Types: {{join .ChangeTypes ", "}}{{end}}
{{if .ChangedPaths}}Files Modified: {{len .ChangedPaths}}{{end}}

IMPORTANT: Review ALL code files below, especially source code (.go, .py, .js, .ts, etc.).
Look for: bugs, security vulnerabilities, logic errors, performance issues, and code quality problems.

{{.Diff}}

{{if .CustomInstructions}}
## Review Instructions
{{.CustomInstructions}}
{{end}}

{{if .CustomContext}}
## Additional Context
{{.CustomContext}}
{{end}}

## Background Documentation (for reference only)

{{if .Architecture}}
### Project Architecture
{{.Architecture}}
{{end}}

{{if .README}}
### Project Overview
{{.README}}
{{end}}

{{if .DesignDocs}}
### Design Documentation
{{.DesignDocs}}
{{end}}

{{if .RelevantDocs}}
### Relevant Documentation
{{.RelevantDocs}}
{{end}}

## Required Output Format

You MUST respond with a JSON object matching this EXACT schema (use camelCase for field names):

` + "```" + `json
{
  "summary": "A brief text summary of the review (1-3 sentences)",
  "findings": [
    {
      "file": "path/to/file.go",
      "lineStart": 42,
      "lineEnd": 42,
      "severity": "high|medium|low",
      "category": "security|bug|performance|maintainability|test_coverage|error_handling|architecture",
      "description": "Clear description of the issue",
      "suggestion": "Actionable fix or improvement",
      "evidence": true
    }
  ]
}
` + "```" + `

Rules:
- "summary" MUST be a string, not an object
- Use camelCase: "lineStart" and "lineEnd", NOT "line_start" or "line_end"
- "severity" must be one of: "high", "medium", "low"
- "evidence" should be true if you can point to specific code
- If no issues found, return: {"summary": "No issues found.", "findings": []}
- Focus on actual code issues, not documentation improvements`
}
