package review

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// defaultMaxTokens sets the maximum output tokens for LLM responses.
//
// This is set to 8192 as a safe default that works across all providers:
// - Claude Sonnet: max 8k output tokens
// - GPT-4-turbo: max 4k-16k depending on variant
// - Gemini: supports up to 32k
//
// Note for extended thinking models (Gemini 2.5 Pro, OpenAI o1/o3/o4):
// These models use tokens for internal reasoning before generating output.
// If you encounter MAX_TOKENS errors with these models, you may need to:
//   1. Use a provider that supports higher limits (Gemini supports 32k)
//   2. Configure a custom max tokens value per provider in your config
//   3. Reduce the size of diffs being reviewed
//
// The 8k limit provides a good balance: enough for substantial code reviews
// while preventing HTTP 400 errors from providers with lower limits.
const defaultMaxTokens = 8192

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
