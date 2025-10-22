package review

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// defaultMaxTokens sets the maximum output tokens for LLM responses.
// This needs to be high enough to accommodate:
// 1. Extended thinking models (Gemini 2.5 Pro, OpenAI o1/o4) which use tokens for internal reasoning
// 2. The actual JSON response with findings
// Gemini 2.5 Pro supports up to 32k output tokens, Claude Sonnet up to 8k.
const defaultMaxTokens = 16384

// DefaultPromptBuilder renders a structured prompt for the provider.
func DefaultPromptBuilder(diff domain.Diff, req BranchRequest) (ProviderRequest, error) {
	var builder strings.Builder
	builder.WriteString("You are an expert software engineer performing a code review.\n")
	builder.WriteString("Provide actionable findings in JSON matching the expected schema.\n\n")
	builder.WriteString(fmt.Sprintf("Base Ref: %s\n", req.BaseRef))
	builder.WriteString(fmt.Sprintf("Target Ref: %s\n\n", req.TargetRef))
	builder.WriteString("Unified Diff:\n")
	for _, file := range diff.Files {
		builder.WriteString(fmt.Sprintf("File: %s (%s)\n", file.Path, file.Status))
		builder.WriteString(file.Patch)
		builder.WriteString("\n")
	}

	return ProviderRequest{
		Prompt:  builder.String(),
		MaxSize: defaultMaxTokens,
	}, nil
}
