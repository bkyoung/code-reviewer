package review

import (
	"strings"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

func TestRenderPromptTemplate(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		context        ProjectContext
		diff           domain.Diff
		expectedText   []string // Strings that should appear in output
		unexpectedText []string // Strings that should NOT appear
		expectError    bool
	}{
		{
			name:     "simple template with architecture",
			template: "Architecture:\n{{.Architecture}}\n\nDiff:\n{{.Diff}}",
			context: ProjectContext{
				Architecture: "Clean architecture with layers",
			},
			diff: domain.Diff{
				Files: []domain.FileDiff{
					{Path: "main.go", Status: "modified", Patch: "diff content"},
				},
			},
			expectedText: []string{"Architecture:", "Clean architecture with layers", "Diff:", "diff content"},
		},
		{
			name:     "template with custom instructions",
			template: "{{if .CustomInstructions}}Instructions: {{.CustomInstructions}}{{end}}",
			context: ProjectContext{
				CustomInstructions: "Focus on security",
			},
			expectedText: []string{"Instructions:", "Focus on security"},
		},
		{
			name:     "template without custom instructions",
			template: "{{if .CustomInstructions}}Instructions: {{.CustomInstructions}}{{end}}Review the code.",
			context: ProjectContext{
				CustomInstructions: "",
			},
			expectedText:   []string{"Review the code."},
			unexpectedText: []string{"Instructions:"},
		},
		{
			name:     "template with change types",
			template: "Change types: {{range $i, $type := .ChangeTypes}}{{if $i}}, {{end}}{{$type}}{{end}}",
			context: ProjectContext{
				ChangeTypes: []string{"auth", "database"},
			},
			expectedText: []string{"Change types:", "auth", "database"},
		},
		{
			name: "template with multiple sections",
			template: `{{if .Architecture}}## Architecture
{{.Architecture}}
{{end}}
{{if .CustomInstructions}}## Instructions
{{.CustomInstructions}}
{{end}}
## Changes
{{.Diff}}`,
			context: ProjectContext{
				Architecture:       "Layered architecture",
				CustomInstructions: "Check for race conditions",
			},
			diff: domain.Diff{
				Files: []domain.FileDiff{
					{Path: "worker.go", Patch: "+func process()"},
				},
			},
			expectedText: []string{
				"## Architecture",
				"Layered architecture",
				"## Instructions",
				"Check for race conditions",
				"## Changes",
				"+func process()",
			},
		},
		{
			name:     "template with join helper",
			template: `Files: {{join .ChangedPaths ", "}}`,
			context: ProjectContext{
				ChangedPaths: []string{"main.go", "util.go", "test.go"},
			},
			expectedText: []string{"Files:", "main.go, util.go, test.go"},
		},
		{
			name:        "invalid template syntax",
			template:    "{{.InvalidField",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &EnhancedPromptBuilder{}

			// Use empty request for template rendering tests
			req := BranchRequest{BaseRef: "main", TargetRef: "feature"}
			result, err := builder.renderTemplate(tt.template, tt.context, tt.diff, req)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectError {
				return // Don't check content if we expected an error
			}

			// Check expected text appears
			for _, expected := range tt.expectedText {
				if !strings.Contains(result, expected) {
					t.Errorf("expected text %q not found in output:\n%s", expected, result)
				}
			}

			// Check unexpected text does NOT appear
			for _, unexpected := range tt.unexpectedText {
				if strings.Contains(result, unexpected) {
					t.Errorf("unexpected text %q found in output:\n%s", unexpected, result)
				}
			}
		})
	}
}

func TestFormatDiff(t *testing.T) {
	tests := []struct {
		name     string
		diff     domain.Diff
		expected []string // Strings that should appear in formatted output
	}{
		{
			name: "single file diff",
			diff: domain.Diff{
				Files: []domain.FileDiff{
					{
						Path:   "main.go",
						Status: "modified",
						Patch:  "@@ -1,3 +1,4 @@\n func main() {\n+  fmt.Println(\"hello\")\n }",
					},
				},
			},
			expected: []string{"main.go", "modified", "func main()", "fmt.Println"},
		},
		{
			name: "multiple files",
			diff: domain.Diff{
				Files: []domain.FileDiff{
					{Path: "main.go", Status: "modified", Patch: "patch1"},
					{Path: "util.go", Status: "added", Patch: "patch2"},
					{Path: "old.go", Status: "deleted", Patch: ""},
				},
			},
			expected: []string{"main.go", "modified", "util.go", "added", "old.go", "deleted"},
		},
		{
			name: "empty diff",
			diff: domain.Diff{
				Files: []domain.FileDiff{},
			},
			expected: []string{}, // Should return empty or minimal output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &EnhancedPromptBuilder{}
			result := builder.formatDiff(tt.diff)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("expected text %q not found in formatted diff:\n%s", expected, result)
				}
			}
		})
	}
}

func TestBuildPromptWithContext(t *testing.T) {
	// Integration test: build a complete prompt with all context
	builder := NewEnhancedPromptBuilder()

	context := ProjectContext{
		Architecture:       "Clean architecture system",
		README:             "# My Project\nA great project",
		DesignDocs:         []string{"=== AUTH_DESIGN.md ===\nJWT authentication"},
		CustomInstructions: "Focus on security and performance",
		RelevantDocs:       []string{"=== SECURITY.md ===\nSecurity guidelines"},
		ChangedPaths:       []string{"auth/handler.go", "auth/middleware.go"},
		ChangeTypes:        []string{"auth", "security"},
	}

	diff := domain.Diff{
		Files: []domain.FileDiff{
			{
				Path:   "auth/handler.go",
				Status: "modified",
				Patch:  "@@ -10,5 +10,6 @@\n func Login(req Request) {\n+  validateToken(req.Token)\n }",
			},
		},
	}

	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature/auth-improvements",
	}

	// Use default template
	result, err := builder.Build(context, diff, req, "openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify prompt contains key elements
	expectedElements := []string{
		"Clean architecture system",
		"Focus on security and performance",
		"auth/handler.go",
		"validateToken",
		"main",
		"feature/auth-improvements",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(result.Prompt, expected) {
			t.Errorf("expected element %q not found in prompt", expected)
		}
	}

	// Verify max size is set
	if result.MaxSize == 0 {
		t.Error("expected MaxSize to be set")
	}
}

func TestProviderSpecificTemplates(t *testing.T) {
	builder := NewEnhancedPromptBuilder()

	// Add provider-specific templates
	builder.SetProviderTemplate("anthropic", `<role>Expert reviewer</role>
<instructions>{{.CustomInstructions}}</instructions>
<changes>{{.Diff}}</changes>`)

	builder.SetProviderTemplate("openai", `You are an expert reviewer.
Instructions: {{.CustomInstructions}}
Changes: {{.Diff}}`)

	context := ProjectContext{
		CustomInstructions: "Check for bugs",
	}
	diff := domain.Diff{
		Files: []domain.FileDiff{{Path: "test.go", Patch: "patch"}},
	}
	req := BranchRequest{BaseRef: "main", TargetRef: "feature"}

	// Test Anthropic template
	anthropicResult, err := builder.Build(context, diff, req, "anthropic")
	if err != nil {
		t.Fatalf("anthropic build failed: %v", err)
	}
	if !strings.Contains(anthropicResult.Prompt, "<role>") {
		t.Error("Anthropic template not used (missing <role> tag)")
	}

	// Test OpenAI template
	openaiResult, err := builder.Build(context, diff, req, "openai")
	if err != nil {
		t.Fatalf("openai build failed: %v", err)
	}
	if !strings.Contains(openaiResult.Prompt, "You are an expert reviewer") {
		t.Error("OpenAI template not used")
	}
	if strings.Contains(openaiResult.Prompt, "<role>") {
		t.Error("OpenAI should not have Anthropic-style tags")
	}
}

func TestIntegration_ContextGatheringWithPromptBuilder(t *testing.T) {
	// Integration test: gather context and build prompts in realistic scenario

	// Setup context gatherer with test data directory
	gatherer := NewContextGatherer("testdata")

	// Load architecture and design docs
	architecture, err := gatherer.loadFile("docs/ARCHITECTURE.md")
	if err != nil {
		t.Logf("Warning: failed to load architecture: %v", err)
		architecture = "" // Continue without it
	}

	designDocs, err := gatherer.loadDesignDocs()
	if err != nil {
		t.Logf("Warning: failed to load design docs: %v", err)
		designDocs = nil // Continue without them
	}
	t.Logf("Loaded %d design docs", len(designDocs))
	for i, doc := range designDocs {
		t.Logf("Design doc %d preview: %s", i, doc[:min(100, len(doc))])
	}

	// Simulate a diff with auth-related changes
	diff := domain.Diff{
		Files: []domain.FileDiff{
			{
				Path:   "internal/auth/handler.go",
				Status: "modified",
				Patch:  "@@ -15,5 +15,7 @@\n func Login(req Request) {\n+  token := generateToken()\n+  validateToken(token)\n }",
			},
		},
	}

	// Detect change types
	changeTypes := gatherer.detectChangeTypes(diff)
	t.Logf("Detected change types: %v", changeTypes)

	// Find relevant docs
	relevantDocs, err := gatherer.findRelevantDocs([]string{"internal/auth/handler.go"}, changeTypes)
	if err != nil {
		t.Logf("Warning: failed to find relevant docs: %v", err)
		relevantDocs = nil // Continue without them
	}
	t.Logf("Found %d relevant docs", len(relevantDocs))

	// Build project context
	context := ProjectContext{
		Architecture:       architecture,
		DesignDocs:         designDocs,
		RelevantDocs:       relevantDocs,
		CustomInstructions: "Focus on security vulnerabilities",
		ChangeTypes:        changeTypes,
		ChangedPaths:       []string{"internal/auth/handler.go"},
	}

	// Create prompt builder
	builder := NewEnhancedPromptBuilder()

	// Build prompt
	req := BranchRequest{
		BaseRef:   "main",
		TargetRef: "feature/auth-improvements",
	}

	result, err := builder.Build(context, diff, req, "openai")
	if err != nil {
		t.Fatalf("failed to build prompt: %v", err)
	}

	t.Logf("Generated prompt length: %d bytes", len(result.Prompt))
	t.Logf("Prompt preview (first 500 chars):\n%s", result.Prompt[:min(500, len(result.Prompt))])

	// Verify prompt contains key expected context
	expectedElements := []string{
		"Focus on security vulnerabilities", // Custom instructions should always be there
		"generateToken",                     // From diff
		"main",                              // Base ref
		"feature/auth-improvements",         // Target ref
	}

	for _, expected := range expectedElements {
		if !strings.Contains(result.Prompt, expected) {
			t.Errorf("prompt missing expected element %q", expected)
		}
	}

	// Verify design docs are included if they were loaded
	if len(designDocs) > 0 {
		if !strings.Contains(result.Prompt, "JWT") && !strings.Contains(result.Prompt, "authentication") {
			t.Error("prompt should include content from design docs when available")
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestFileTypePriority(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		// Priority 0: Source code files (including test files with source extensions)
		{"main.go", 0},
		{"internal/auth/handler.go", 0},
		{"src/app.py", 0},
		{"index.js", 0},
		{"component.tsx", 0},
		{"lib.rs", 0},
		{"Main.java", 0},
		{"utils.c", 0},
		{"helper.cpp", 0},
		{"test_utils.py", 0},  // .py is source code, even if it's a test
		{"spec/helper.rb", 0}, // .rb is source code, even if it's a spec

		// Priority 1: Test directories without source extensions
		// (Currently no common cases - most test files have source extensions)

		// Priority 2: Configuration files
		{"config.yaml", 2},
		{"settings.yml", 2},
		{"package.json", 2},
		{"config.toml", 2},
		{".env", 2},
		{".github/workflows/ci.yml", 2}, // .yml is config (checked before CI path check)

		// Priority 3: Build/CI files (without config extensions)
		{"Dockerfile", 3},
		{"Makefile", 3},

		// Priority 4: Documentation files
		{"README.md", 4},
		{"docs/ARCHITECTURE.md", 4},
		{"CHANGELOG.rst", 4},
		{"notes.txt", 4},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fileTypePriority(tt.path)
			if result != tt.expected {
				t.Errorf("fileTypePriority(%q) = %d, want %d", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFormatDiff_FileOrdering(t *testing.T) {
	// Test that files are sorted with source code first
	builder := &EnhancedPromptBuilder{}

	diff := domain.Diff{
		Files: []domain.FileDiff{
			{Path: "README.md", Status: "modified", Patch: "readme patch"},
			{Path: "docs/DESIGN.md", Status: "added", Patch: "design patch"},
			{Path: "main.go", Status: "modified", Patch: "go patch"},
			{Path: "config.yaml", Status: "modified", Patch: "yaml patch"},
			{Path: "security-tests/test.go", Status: "added", Patch: "test patch"},
		},
	}

	result := builder.formatDiff(diff)

	// Find positions of each file in the output
	goPos := strings.Index(result, "main.go")
	testPos := strings.Index(result, "security-tests/test.go")
	yamlPos := strings.Index(result, "config.yaml")
	readmePos := strings.Index(result, "README.md")
	docsPos := strings.Index(result, "docs/DESIGN.md")

	// Source code (.go) should come first
	if goPos > yamlPos || goPos > readmePos || goPos > docsPos {
		t.Error("Source code files (.go) should appear before config and docs")
	}

	// Test files should come after main source but before config
	if testPos > yamlPos {
		t.Error("Test files should appear before config files")
	}

	// Config should come before documentation
	if yamlPos > readmePos || yamlPos > docsPos {
		t.Error("Config files should appear before documentation")
	}

	// Both markdown files should be at the end
	if readmePos < yamlPos || docsPos < yamlPos {
		t.Error("Documentation files should appear last")
	}
}

func TestPromptTemplate_CodeFirst(t *testing.T) {
	// Verify that the default template puts code before documentation
	template := defaultPromptTemplate()

	// Find positions of key sections
	codeChangesPos := strings.Index(template, "Code Changes to Review")
	architecturePos := strings.Index(template, "Project Architecture")
	readmePos := strings.Index(template, "Project Overview")
	designDocsPos := strings.Index(template, "Design Documentation")

	// Code changes section should come before documentation sections
	if codeChangesPos > architecturePos {
		t.Error("Code Changes section should appear before Architecture section")
	}
	if codeChangesPos > readmePos {
		t.Error("Code Changes section should appear before Project Overview section")
	}
	if codeChangesPos > designDocsPos {
		t.Error("Code Changes section should appear before Design Documentation section")
	}

	// Template should emphasize code review
	if !strings.Contains(template, "PRIMARY FOCUS") {
		t.Error("Template should emphasize that code is the primary focus")
	}
	if !strings.Contains(template, "source code") {
		t.Error("Template should mention reviewing source code files")
	}
}
