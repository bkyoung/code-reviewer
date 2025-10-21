package anthropic

import (
	"context"
	"fmt"

	"github.com/brandon/code-reviewer/internal/domain"
	"github.com/brandon/code-reviewer/internal/usecase/review"
)

const providerName = "anthropic"

// Client abstracts the Anthropic HTTP client behaviour we need.
type Client interface {
	CreateReview(ctx context.Context, req Request) (Response, error)
}

// Request represents the outbound payload for the Anthropic provider.
type Request struct {
	Model     string
	Prompt    string
	Seed      uint64
	MaxTokens int
}

// Response captures the structured result returned by the LLM.
type Response struct {
	Model    string
	Summary  string
	Findings []domain.Finding
}

// Provider implements the usecase Provider port.
type Provider struct {
	model  string
	client Client
}

// NewProvider constructs a Provider for the supplied model.
func NewProvider(model string, client Client) *Provider {
	return &Provider{
		model:  model,
		client: client,
	}
}

// Review sends the prompt to Anthropic and translates the response.
func (p *Provider) Review(ctx context.Context, req review.ProviderRequest) (domain.Review, error) {
	if p.client == nil {
		return domain.Review{}, fmt.Errorf("anthropic client missing")
	}

	response, err := p.client.CreateReview(ctx, Request{
		Model:     p.model,
		Prompt:    req.Prompt,
		Seed:      req.Seed,
		MaxTokens: req.MaxSize,
	})
	if err != nil {
		return domain.Review{}, err
	}

	return domain.Review{
		ProviderName: providerName,
		ModelName:    response.Model,
		Summary:      response.Summary,
		Findings:     response.Findings,
	}, nil
}
