package review

import (
	"fmt"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

const defaultMaxTokens = 4096

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
