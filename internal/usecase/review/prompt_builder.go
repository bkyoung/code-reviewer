package review

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

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
func (b *EnhancedPromptBuilder) formatDiff(diff domain.Diff) string {
	if len(diff.Files) == 0 {
		return "(no changes)"
	}

	var buf bytes.Buffer

	for _, file := range diff.Files {
		buf.WriteString(fmt.Sprintf("File: %s (%s)\n", file.Path, file.Status))
		if file.Patch != "" {
			buf.WriteString(file.Patch)
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// defaultPromptTemplate returns the default template used when no provider-specific template is set.
func defaultPromptTemplate() string {
	return `You are an expert software engineer performing a code review.
Provide actionable findings in JSON format matching the expected schema.

{{if .Architecture}}
## Project Architecture
{{.Architecture}}
{{end}}

{{if .README}}
## Project Overview
{{.README}}
{{end}}

{{if .DesignDocs}}
## Design Documentation
{{.DesignDocs}}
{{end}}

{{if .RelevantDocs}}
## Relevant Documentation
{{.RelevantDocs}}
{{end}}

{{if .CustomInstructions}}
## Review Instructions
{{.CustomInstructions}}
{{end}}

{{if .CustomContext}}
## Additional Context
{{.CustomContext}}
{{end}}

## Changes to Review
Base Ref: {{.BaseRef}}
Target Ref: {{.TargetRef}}
{{if .ChangeTypes}}Change Types: {{join .ChangeTypes ", "}}{{end}}
{{if .ChangedPaths}}Files Modified: {{len .ChangedPaths}}{{end}}

{{.Diff}}

Analyze these changes and provide structured feedback in JSON format with severity, category, file, line numbers, description, and actionable suggestions.`
}
