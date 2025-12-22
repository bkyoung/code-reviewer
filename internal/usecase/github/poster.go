// Package github provides use cases for interacting with GitHub.
package github

import (
	"context"
	"log"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/domain"
)

// ReviewClient defines the interface for interacting with GitHub reviews.
// This interface allows for mocking in tests.
type ReviewClient interface {
	CreateReview(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error)
	ListReviews(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error)
	DismissReview(ctx context.Context, owner, repo string, pullNumber int, reviewID int64, message string) (*github.DismissReviewResponse, error)
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

	// BotUsername is the username to match for auto-dismissing stale reviews.
	// If empty, no reviews are dismissed. Example: "github-actions[bot]"
	BotUsername string
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

	// DismissedCount is the number of previous bot reviews that were dismissed.
	DismissedCount int
}

// PostReview posts a code review to GitHub.
// It converts domain findings to GitHub review comments, determines the
// appropriate review event based on severity, and posts the review.
//
// If BotUsername is set, previous reviews from that bot are dismissed AFTER
// posting the new review succeeds. This ensures the PR always has at least one
// review signal - if posting fails, previous reviews are preserved.
// Dismiss failures are logged but do not affect the result.
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

	// Call the client to create the new review first
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

	// Dismiss previous bot reviews AFTER successful post
	// This ensures PR always has review signal - if post failed, old reviews remain
	var dismissedCount int
	if req.BotUsername != "" {
		dismissedCount = p.dismissStaleReviews(ctx, req.Owner, req.Repo, req.PullNumber, req.BotUsername)
	}

	return &PostReviewResult{
		ReviewID:        resp.ID,
		CommentsPosted:  inDiffCount,
		CommentsSkipped: skippedCount,
		Event:           event,
		HTMLURL:         resp.HTMLURL,
		DismissedCount:  dismissedCount,
	}, nil
}

// dismissStaleReviews finds and dismisses all previous reviews from the bot.
// Returns the number of reviews dismissed. Errors are logged but do not
// block the review posting workflow.
func (p *ReviewPoster) dismissStaleReviews(ctx context.Context, owner, repo string, pullNumber int, botUsername string) int {
	reviews, err := p.client.ListReviews(ctx, owner, repo, pullNumber)
	if err != nil {
		log.Printf("warning: failed to list reviews for dismissal: %v", err)
		return 0
	}

	var dismissedCount int
	for _, review := range reviews {
		if shouldDismissReview(review, botUsername) {
			_, err := p.client.DismissReview(ctx, owner, repo, pullNumber, review.ID, "Superseded by new review")
			if err != nil {
				log.Printf("warning: failed to dismiss review %d: %v", review.ID, err)
				continue
			}
			dismissedCount++
		}
	}

	return dismissedCount
}

// shouldDismissReview returns true if the review should be dismissed.
// A review should be dismissed if it's from the bot and not already dismissed.
func shouldDismissReview(review github.ReviewSummary, botUsername string) bool {
	// Case-insensitive comparison for usernames (GitHub usernames are case-insensitive)
	if !strings.EqualFold(review.User.Login, botUsername) {
		return false
	}

	// Skip already dismissed reviews
	if review.State == "DISMISSED" {
		return false
	}

	// Skip pending reviews (not yet submitted)
	if review.State == "PENDING" {
		return false
	}

	// Dismiss all other states (APPROVED, CHANGES_REQUESTED, COMMENTED)
	return true
}
