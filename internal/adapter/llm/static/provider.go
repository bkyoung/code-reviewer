package static

import (
	"context"

	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

const providerName = "static"

// Provider implements the usecase Provider port.
type Provider struct {
	model string
}

// NewProvider constructs a static Provider.
func NewProvider(model string) *Provider {
	return &Provider{
		model: model,
	}
}

// Review returns a static, pre-determined review.
func (p *Provider) Review(ctx context.Context, req review.ProviderRequest) (domain.Review, error) {
	finding := domain.NewFinding(domain.FindingInput{
		File:        "internal/adapter/llm/static/provider.go",
		LineStart:   1,
		LineEnd:     5,
		Severity:    "low",
		Category:    "style",
		Description: "This is a static finding.",
		Suggestion:  "No suggestion.",
		Evidence:    true,
	})

	return domain.Review{
		ProviderName: providerName,
		ModelName:    p.model,
		Summary:      "This is a static review from a mock provider.",
		Findings:     []domain.Finding{finding},
	}, nil
}
