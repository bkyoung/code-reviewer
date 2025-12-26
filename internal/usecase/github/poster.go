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
	ListPullRequestComments(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error)
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

	// DuplicatesSkipped is the number of findings skipped because they were
	// already posted in previous reviews (deduplication).
	DuplicatesSkipped int

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
// If BotUsername is set:
//   - Existing bot comments are fetched to deduplicate findings (Issue #107)
//   - Previous reviews from that bot are dismissed AFTER posting succeeds
//
// This ensures the PR always has at least one review signal - if posting fails,
// previous reviews are preserved. Dismiss failures are logged but do not affect
// the result.
//
// Findings without a DiffPosition (not in diff) are silently skipped and
// counted in CommentsSkipped. Findings already posted (matching fingerprint)
// are counted in DuplicatesSkipped.
func (p *ReviewPoster) PostReview(ctx context.Context, req PostReviewRequest) (*PostReviewResult, error) {
	findings := req.Findings
	var duplicatesSkipped int

	// Deduplicate findings if BotUsername is set
	if req.BotUsername != "" {
		var err error
		findings, duplicatesSkipped, err = p.deduplicateFindings(ctx, req)
		if err != nil {
			// Log and continue without deduplication
			log.Printf("warning: failed to fetch comments for deduplication: %v", err)
			findings = req.Findings
			duplicatesSkipped = 0
		}
	}

	// Count in-diff vs out-of-diff findings (after deduplication)
	inDiffCount := github.CountInDiffFindings(findings)
	skippedCount := len(findings) - inDiffCount

	// Determine review event (using deduplicated findings)
	var event github.ReviewEvent
	if req.OverrideEvent != "" {
		event = req.OverrideEvent
	} else {
		event = github.DetermineReviewEventWithActions(findings, req.ReviewActions)
	}

	// Call the client to create the new review first
	input := github.CreateReviewInput{
		Owner:      req.Owner,
		Repo:       req.Repo,
		PullNumber: req.PullNumber,
		CommitSHA:  req.CommitSHA,
		Event:      event,
		Summary:    req.Review.Summary,
		Findings:   findings,
	}

	resp, err := p.client.CreateReview(ctx, input)
	if err != nil {
		return nil, err
	}

	// Dismiss previous bot reviews AFTER successful post
	// This ensures PR always has review signal - if post failed, old reviews remain
	// Pass the new review ID to avoid dismissing the review we just created
	var dismissedCount int
	if req.BotUsername != "" {
		dismissedCount = p.dismissStaleReviews(ctx, req.Owner, req.Repo, req.PullNumber, req.BotUsername, resp.ID)
	}

	return &PostReviewResult{
		ReviewID:          resp.ID,
		CommentsPosted:    inDiffCount,
		CommentsSkipped:   skippedCount,
		DuplicatesSkipped: duplicatesSkipped,
		Event:             event,
		HTMLURL:           resp.HTMLURL,
		DismissedCount:    dismissedCount,
	}, nil
}

// dismissStaleReviews finds and dismisses all previous reviews from the bot.
// The excludeReviewID parameter specifies a review ID to skip (typically the
// newly created review). Returns the number of reviews dismissed. Errors are
// logged but do not block the review posting workflow.
func (p *ReviewPoster) dismissStaleReviews(ctx context.Context, owner, repo string, pullNumber int, botUsername string, excludeReviewID int64) int {
	reviews, err := p.client.ListReviews(ctx, owner, repo, pullNumber)
	if err != nil {
		log.Printf("warning: failed to list reviews for dismissal: %v", err)
		return 0
	}

	var dismissedCount int
	for _, review := range reviews {
		// Skip the newly created review to avoid dismissing our own fresh review
		if review.ID == excludeReviewID {
			continue
		}
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
	if review.State == string(github.StateDismissed) {
		return false
	}

	// Skip pending reviews (not yet submitted)
	if review.State == string(github.StatePending) {
		return false
	}

	// Dismiss all other states (APPROVED, CHANGES_REQUESTED, COMMENTED)
	return true
}

// deduplicateFindings fetches existing bot comments from the PR and filters out
// any findings that have already been posted (based on fingerprint matching).
// Returns the filtered findings, the count of duplicates skipped, and any error.
func (p *ReviewPoster) deduplicateFindings(
	ctx context.Context,
	req PostReviewRequest,
) ([]github.PositionedFinding, int, error) {
	// Fetch existing PR comments
	comments, err := p.client.ListPullRequestComments(ctx, req.Owner, req.Repo, req.PullNumber)
	if err != nil {
		return nil, 0, err
	}

	// Extract fingerprints from bot's comments
	existingFingerprints := extractBotFingerprints(comments, req.BotUsername)
	if len(existingFingerprints) == 0 {
		return req.Findings, 0, nil
	}

	// Filter out findings that already have comments
	var filtered []github.PositionedFinding
	var duplicatesSkipped int
	for _, pf := range req.Findings {
		fp := domain.FingerprintFromFinding(pf.Finding)
		if existingFingerprints[fp] {
			duplicatesSkipped++
			continue
		}
		filtered = append(filtered, pf)
	}

	return filtered, duplicatesSkipped, nil
}

// extractBotFingerprints extracts fingerprints from comments authored by the bot.
// Returns a map of fingerprints for O(1) lookup.
func extractBotFingerprints(comments []github.PullRequestComment, botUsername string) map[domain.FindingFingerprint]bool {
	fingerprints := make(map[domain.FindingFingerprint]bool)

	for _, comment := range comments {
		// Skip comments not from the bot (case-insensitive)
		if !strings.EqualFold(comment.User.Login, botUsername) {
			continue
		}

		// Try to extract fingerprint from comment body
		if fp, ok := github.ExtractFingerprintFromComment(comment.Body); ok {
			fingerprints[fp] = true
		}
	}

	return fingerprints
}
