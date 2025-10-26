package merge

import (
	"context"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// Service defines the merging logic.
type Service struct{}

// NewService creates a new merge service.
func NewService() *Service {
	return &Service{}
}

// Merge combines multiple reviews into a single review, de-duplicating findings.
func (s *Service) Merge(ctx context.Context, reviews []domain.Review) domain.Review {
	mergedReview := domain.Review{
		ProviderName: "merged",
		ModelName:    "consensus",
		Summary:      "This is a merged review.",
	}

	seenFindings := make(map[string]bool)
	var findings []domain.Finding

	for _, review := range reviews {
		for _, finding := range review.Findings {
			if !seenFindings[finding.ID] {
				seenFindings[finding.ID] = true
				findings = append(findings, finding)
			}
		}
	}

	mergedReview.Findings = findings
	return mergedReview
}
