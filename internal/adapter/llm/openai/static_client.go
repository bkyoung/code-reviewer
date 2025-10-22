package openai

import (
	"context"
	"fmt"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// StaticClient provides an offline-friendly OpenAI client implementation.
type StaticClient struct{}

// NewStaticClient constructs a stubbed OpenAI client.
func NewStaticClient() *StaticClient {
	return &StaticClient{}
}

// CreateReview returns a deterministic placeholder review.
func (s *StaticClient) CreateReview(ctx context.Context, req Request) (Response, error) {
	summary := fmt.Sprintf("Static review for model %s with seed %d over prompt: %.40s", req.Model, req.Seed, req.Prompt)
	return Response{
		Model:    req.Model,
		Summary:  summary,
		Findings: []domain.Finding{},
	}, nil
}
