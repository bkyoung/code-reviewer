// Package github provides use cases for interacting with GitHub.
package github

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/dedup"
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
	client           ReviewClient
	semanticComparer dedup.SemanticComparer
	semanticConfig   SemanticDedupConfig
}

// SemanticDedupConfig configures semantic deduplication behavior.
type SemanticDedupConfig struct {
	// LineThreshold is the maximum line distance for candidate pairing.
	LineThreshold int

	// MaxCandidates limits the number of candidates per review (cost guard).
	MaxCandidates int
}

// DefaultSemanticDedupConfig returns sensible defaults for semantic deduplication.
func DefaultSemanticDedupConfig() SemanticDedupConfig {
	return SemanticDedupConfig{
		LineThreshold: 10,
		MaxCandidates: 50,
	}
}

// ReviewPosterOption configures a ReviewPoster.
type ReviewPosterOption func(*ReviewPoster)

// WithSemanticComparer sets the semantic comparer for LLM-based deduplication.
func WithSemanticComparer(comparer dedup.SemanticComparer, config SemanticDedupConfig) ReviewPosterOption {
	return func(p *ReviewPoster) {
		p.semanticComparer = comparer
		p.semanticConfig = config
	}
}

// NewReviewPoster creates a new ReviewPoster with the given client.
func NewReviewPoster(client ReviewClient, opts ...ReviewPosterOption) *ReviewPoster {
	p := &ReviewPoster{
		client:         client,
		semanticConfig: DefaultSemanticDedupConfig(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
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
	// already posted in previous reviews (exact fingerprint match).
	DuplicatesSkipped int

	// SemanticDuplicatesSkipped is the number of findings skipped because they
	// were determined to be semantic duplicates of existing findings (LLM-based).
	SemanticDuplicatesSkipped int

	// Event is the review event that was used.
	Event github.ReviewEvent

	// HTMLURL is the URL to view the review on GitHub.
	HTMLURL string

	// DismissedCount is the number of previous bot reviews that were dismissed.
	DismissedCount int

	// Status counts from reply analysis (Issue #108)
	// AcknowledgedCount is the number of existing findings with acknowledgment replies.
	AcknowledgedCount int

	// DisputedCount is the number of existing findings with dispute replies.
	DisputedCount int

	// OpenCount is the number of existing findings with no status-changing replies.
	OpenCount int
}

// PostReview posts a code review to GitHub.
// It converts domain findings to GitHub review comments, determines the
// appropriate review event based on severity, and posts the review.
//
// If BotUsername is set:
//   - Existing bot comments are fetched to deduplicate findings (Issue #107)
//   - Reply statuses are analyzed to determine acknowledged/disputed findings (Issue #108)
//   - Acknowledged/disputed findings don't count toward blocking
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
	var semanticDuplicatesSkipped int
	var existingStatuses map[domain.FindingFingerprint]domain.FindingStatus
	var statusCounts StatusCounts

	// Analyze existing comments if BotUsername is set
	if req.BotUsername != "" {
		var comments []github.PullRequestComment
		var err error

		// Fetch comments once for both deduplication and status analysis
		comments, err = p.client.ListPullRequestComments(ctx, req.Owner, req.Repo, req.PullNumber)
		if err != nil {
			log.Printf("warning: failed to fetch comments: %v", err)
		} else {
			// Stage 1: Fingerprint deduplication (exact match)
			findings, duplicatesSkipped = filterDuplicateFindings(req.Findings, comments, req.BotUsername)

			// Stage 2: Semantic deduplication (LLM-based) - Issue #111
			if p.semanticComparer != nil && len(findings) > 0 {
				findings, semanticDuplicatesSkipped = p.filterSemanticDuplicates(ctx, findings, comments, req.BotUsername)
			}

			// Analyze reply statuses (Issue #108)
			existingStatuses, statusCounts = analyzeFindingStatuses(comments, req.BotUsername)
		}
	}

	// Count in-diff vs out-of-diff findings (after deduplication)
	inDiffCount := github.CountInDiffFindings(findings)
	skippedCount := len(findings) - inDiffCount

	// Determine review event considering reply statuses.
	// Acknowledged/disputed findings don't count toward blocking.
	// NOTE: We use req.Findings (original, unfiltered) rather than the deduplicated
	// `findings` because even duplicated high-severity findings should still block
	// the PR if they haven't been acknowledged/disputed. The deduplication only
	// affects what comments are posted, not the blocking decision.
	var event github.ReviewEvent
	if req.OverrideEvent != "" {
		event = req.OverrideEvent
	} else {
		event = determineEffectiveEvent(req.Findings, existingStatuses, req.ReviewActions)
	}

	// Build the summary with status section appended (if applicable)
	summary := req.Review.Summary + formatStatusSection(statusCounts)

	// Call the client to create the new review first
	input := github.CreateReviewInput{
		Owner:      req.Owner,
		Repo:       req.Repo,
		PullNumber: req.PullNumber,
		CommitSHA:  req.CommitSHA,
		Event:      event,
		Summary:    summary,
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
		ReviewID:                  resp.ID,
		CommentsPosted:            inDiffCount,
		CommentsSkipped:           skippedCount,
		DuplicatesSkipped:         duplicatesSkipped,
		SemanticDuplicatesSkipped: semanticDuplicatesSkipped,
		Event:                     event,
		HTMLURL:                   resp.HTMLURL,
		DismissedCount:            dismissedCount,
		AcknowledgedCount:         statusCounts.Acknowledged,
		DisputedCount:             statusCounts.Disputed,
		OpenCount:                 statusCounts.Open,
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

// StatusCounts tracks the count of findings by status.
type StatusCounts struct {
	Open         int
	Acknowledged int
	Disputed     int
}

// filterDuplicateFindings filters out findings that have already been posted.
// Returns the filtered findings and the count of duplicates skipped.
func filterDuplicateFindings(
	findings []github.PositionedFinding,
	comments []github.PullRequestComment,
	botUsername string,
) ([]github.PositionedFinding, int) {
	existingFingerprints := extractBotFingerprints(comments, botUsername)
	if len(existingFingerprints) == 0 {
		return findings, 0
	}

	var filtered []github.PositionedFinding
	var duplicatesSkipped int
	for _, pf := range findings {
		fp := domain.FingerprintFromFinding(pf.Finding)
		if existingFingerprints[fp] {
			duplicatesSkipped++
			continue
		}
		filtered = append(filtered, pf)
	}

	return filtered, duplicatesSkipped
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

// analyzeFindingStatuses analyzes bot comments and their replies to determine
// the status of each existing finding (Issue #108).
// Returns a map of fingerprint → status and counts for each status.
func analyzeFindingStatuses(
	comments []github.PullRequestComment,
	botUsername string,
) (map[domain.FindingFingerprint]domain.FindingStatus, StatusCounts) {
	statuses := make(map[domain.FindingFingerprint]domain.FindingStatus)
	var counts StatusCounts

	// Group comments by parent to get reply chains
	grouped := github.GroupCommentsByParent(comments, botUsername)

	for _, group := range grouped {
		// Extract fingerprint from parent (bot) comment
		fp, ok := github.ExtractFingerprintFromComment(group.Parent.Body)
		if !ok {
			continue // Skip comments without fingerprints (legacy)
		}

		// Collect reply texts
		var replyTexts []string
		for _, reply := range group.Replies {
			replyTexts = append(replyTexts, reply.Body)
		}

		// Detect status from replies
		status := domain.DetectStatusFromReplies(replyTexts)
		statuses[fp] = status

		// Update counts
		switch status {
		case domain.StatusAcknowledged:
			counts.Acknowledged++
		case domain.StatusDisputed:
			counts.Disputed++
		case domain.StatusOpen:
			counts.Open++
		}
	}

	return statuses, counts
}

// formatStatusSection creates a markdown section showing finding status breakdown.
// Returns an empty string if all counts are zero.
func formatStatusSection(counts StatusCounts) string {
	// Only include section if there are any existing findings tracked
	total := counts.Open + counts.Acknowledged + counts.Disputed
	if total == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n---\n\n")
	sb.WriteString("### Existing Finding Status\n\n")

	// Always show all statuses for clarity
	sb.WriteString("| Status | Count | Effect |\n")
	sb.WriteString("|--------|-------|--------|\n")
	sb.WriteString("| 🔓 Open | ")
	sb.WriteString(fmt.Sprintf("%d", counts.Open))
	sb.WriteString(" | Counts toward blocking |\n")
	sb.WriteString("| ✅ Acknowledged | ")
	sb.WriteString(fmt.Sprintf("%d", counts.Acknowledged))
	sb.WriteString(" | Won't block (author accepted) |\n")
	sb.WriteString("| ❌ Disputed | ")
	sb.WriteString(fmt.Sprintf("%d", counts.Disputed))
	sb.WriteString(" | Won't block (author disputes) |\n")

	return sb.String()
}

// determineEffectiveEvent determines the review event considering reply statuses.
// Findings that have been acknowledged or disputed don't count toward blocking.
func determineEffectiveEvent(
	findings []github.PositionedFinding,
	existingStatuses map[domain.FindingFingerprint]domain.FindingStatus,
	actions github.ReviewActions,
) github.ReviewEvent {
	// If no status tracking, fall back to standard behavior
	if existingStatuses == nil {
		return github.DetermineReviewEventWithActions(findings, actions)
	}

	// Filter findings to only include those that are "effective" (not acknowledged/disputed)
	var effectiveFindings []github.PositionedFinding
	for _, pf := range findings {
		fp := domain.FingerprintFromFinding(pf.Finding)
		status, exists := existingStatuses[fp]

		// Include finding if:
		// - It's new (not in existingStatuses)
		// - It's in existingStatuses but status is Open
		if !exists || status == domain.StatusOpen {
			effectiveFindings = append(effectiveFindings, pf)
		}
		// Acknowledged and Disputed findings are excluded from blocking calculation
	}

	return github.DetermineReviewEventWithActions(effectiveFindings, actions)
}

// filterSemanticDuplicates uses LLM-based comparison to identify findings that are
// semantic duplicates of existing comments, even if they have different fingerprints.
// Returns the filtered findings and count of semantic duplicates found.
func (p *ReviewPoster) filterSemanticDuplicates(
	ctx context.Context,
	findings []github.PositionedFinding,
	comments []github.PullRequestComment,
	botUsername string,
) ([]github.PositionedFinding, int) {
	// Convert bot comments to ExistingFinding for candidate detection
	existingFindings := extractExistingFindings(comments, botUsername)
	if len(existingFindings) == 0 {
		return findings, 0
	}

	// Convert positioned findings to domain.Finding for candidate detection
	var domainFindings []domain.Finding
	for _, pf := range findings {
		domainFindings = append(domainFindings, pf.Finding)
	}

	// Find candidate pairs (spatially overlapping findings)
	candidates, overflow := dedup.FindCandidates(
		domainFindings,
		existingFindings,
		p.semanticConfig.LineThreshold,
		p.semanticConfig.MaxCandidates,
	)

	// If no candidates, nothing to compare
	if len(candidates) == 0 {
		return findings, 0
	}

	// Call the semantic comparer
	result, err := p.semanticComparer.Compare(ctx, candidates)
	if err != nil {
		// Fail open: on error, treat all findings as unique
		log.Printf("warning: semantic dedup failed: %v (treating all as unique)", err)
		return findings, 0
	}

	// Build set of fingerprints for findings identified as semantic duplicates
	semanticDupFingerprints := make(map[domain.FindingFingerprint]bool)
	for _, dup := range result.Duplicates {
		fp := domain.FingerprintFromFinding(dup.NewFinding)
		semanticDupFingerprints[fp] = true
		log.Printf("semantic dedup: %s is duplicate of existing (reason: %s)", fp, dup.Reason)
	}

	// Also consider overflow findings as unique (couldn't verify them)
	_ = overflow // Acknowledged but not used - overflow findings remain in the original list

	// Filter out semantic duplicates from positioned findings
	var filtered []github.PositionedFinding
	var duplicatesFound int
	for _, pf := range findings {
		fp := domain.FingerprintFromFinding(pf.Finding)
		if semanticDupFingerprints[fp] {
			duplicatesFound++
			continue
		}
		filtered = append(filtered, pf)
	}

	return filtered, duplicatesFound
}

// extractExistingFindings converts bot comments to ExistingFinding for semantic comparison.
func extractExistingFindings(comments []github.PullRequestComment, botUsername string) []dedup.ExistingFinding {
	var existing []dedup.ExistingFinding

	for _, comment := range comments {
		// Skip comments not from the bot
		if !strings.EqualFold(comment.User.Login, botUsername) {
			continue
		}

		// Skip replies (only process top-level comments)
		if comment.InReplyToID != 0 {
			continue
		}

		// Extract structured details from comment body
		details := github.ExtractCommentDetails(comment.Body)
		if details == nil {
			continue // Not a structured finding comment
		}

		// Use line from API if available, otherwise use parsed line
		lineStart := details.LineStart
		lineEnd := details.LineEnd
		if comment.Line != nil && *comment.Line > 0 {
			lineStart = *comment.Line
			if lineEnd == 0 {
				lineEnd = lineStart
			}
		}

		existing = append(existing, dedup.ExistingFinding{
			Fingerprint: details.Fingerprint,
			File:        comment.Path,
			LineStart:   lineStart,
			LineEnd:     lineEnd,
			Description: details.Description,
			Severity:    details.Severity,
			Category:    details.Category,
		})
	}

	return existing
}
