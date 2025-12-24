package github

import (
	"context"

	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

// GitHubStatusScanner implements review.StatusScanner using the GitHub API.
// It fetches PR review comments and analyzes replies for status update keywords.
type GitHubStatusScanner struct {
	client *Client
}

// NewGitHubStatusScanner creates a new StatusScanner backed by the GitHub API.
func NewGitHubStatusScanner(client *Client) *GitHubStatusScanner {
	return &GitHubStatusScanner{client: client}
}

// ScanForStatusUpdates fetches PR comments and detects status update keywords in replies.
// It returns a list of status updates for findings where users have replied with
// keywords like "acknowledged", "disputed", "won't fix", etc.
func (s *GitHubStatusScanner) ScanForStatusUpdates(
	ctx context.Context,
	owner, repo string,
	prNumber int,
	botUsername string,
) ([]review.StatusUpdateResult, error) {
	// Fetch all review comments on the PR
	comments, err := s.client.ListPullRequestComments(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, err
	}

	// Group comments by parent (bot comments with their replies)
	grouped := GroupCommentsByParent(comments, botUsername)

	// Detect status updates from replies
	updates := DetectStatusUpdates(grouped)

	// Convert to review.StatusUpdateResult
	result := make([]review.StatusUpdateResult, len(updates))
	for i, u := range updates {
		result[i] = review.StatusUpdateResult{
			Fingerprint: u.Fingerprint,
			NewStatus:   u.NewStatus,
			Reason:      u.Reason,
			UpdatedBy:   u.UpdatedBy,
			UpdatedAt:   u.UpdatedAt,
		}
	}

	return result, nil
}

// Ensure GitHubStatusScanner implements review.StatusScanner
var _ review.StatusScanner = (*GitHubStatusScanner)(nil)
