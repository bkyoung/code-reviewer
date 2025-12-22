// Package github provides use cases for interacting with GitHub.
package github

import (
	"context"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/domain"
)

// ReviewClient defines the interface for posting reviews to GitHub.
// This interface allows for mocking in tests.
type ReviewClient interface {
	CreateReview(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error)
}

// ReviewPoster orchestrates posting code review findings to GitHub as PR reviews.
// It determines the appropriate review event based on finding severities,
// filters out findings that are not in the diff, and delegates the actual
// API call to the ReviewClient.
type ReviewPoster struct {
	client ReviewClient
}

// NewReviewPoster creates a new ReviewPoster with the given client.
func NewReviewPoster(client ReviewClient) *ReviewPoster {
	return &ReviewPoster{
		client: client,
	}
}

// PostReviewRequest contains all data needed to post a review.
type PostReviewRequest struct {
	// Owner is the GitHub repository owner (user or organization).
	Owner string

	// Repo is the GitHub repository name.
	Repo string

	// PullNumber is the PR number.
	PullNumber int

	// CommitSHA is the head commit SHA of the PR.
	CommitSHA string

	// Review contains the summary and other metadata.
	Review domain.Review

	// Findings are the positioned findings to post as inline comments.
	Findings []github.PositionedFinding

	// OverrideEvent optionally overrides the automatically determined event.
	// If set, this event will be used instead of determining from severities.
	OverrideEvent github.ReviewEvent

	// ReviewActions configures the review action for each finding severity.
	// If empty, sensible defaults are used (critical/high → request_changes,
	// medium/low → comment, clean → approve).
	ReviewActions github.ReviewActions
}

// PostReviewResult contains the result of posting a review.
type PostReviewResult struct {
	// ReviewID is the GitHub review ID.
	ReviewID int64

	// CommentsPosted is the number of inline comments posted.
	CommentsPosted int

	// CommentsSkipped is the number of findings skipped (not in diff).
	CommentsSkipped int

	// Event is the review event that was used.
	Event github.ReviewEvent

	// HTMLURL is the URL to view the review on GitHub.
	HTMLURL string
}

// PostReview posts a code review to GitHub.
// It converts domain findings to GitHub review comments, determines the
// appropriate review event based on severity, and posts the review.
//
// Findings without a DiffPosition (not in diff) are silently skipped and
// counted in CommentsSkipped.
func (p *ReviewPoster) PostReview(ctx context.Context, req PostReviewRequest) (*PostReviewResult, error) {
	// Count in-diff vs out-of-diff findings
	inDiffCount := github.CountInDiffFindings(req.Findings)
	skippedCount := len(req.Findings) - inDiffCount

	// Determine review event
	var event github.ReviewEvent
	if req.OverrideEvent != "" {
		event = req.OverrideEvent
	} else {
		event = github.DetermineReviewEventWithActions(req.Findings, req.ReviewActions)
	}

	// Call the client
	input := github.CreateReviewInput{
		Owner:      req.Owner,
		Repo:       req.Repo,
		PullNumber: req.PullNumber,
		CommitSHA:  req.CommitSHA,
		Event:      event,
		Summary:    req.Review.Summary,
		Findings:   req.Findings,
	}

	resp, err := p.client.CreateReview(ctx, input)
	if err != nil {
		return nil, err
	}

	return &PostReviewResult{
		ReviewID:        resp.ID,
		CommentsPosted:  inDiffCount,
		CommentsSkipped: skippedCount,
		Event:           event,
		HTMLURL:         resp.HTMLURL,
	}, nil
}
