package github

import (
	"log"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// StatusUpdate represents a detected status change for a finding.
// This is produced by analyzing reply comments on the bot's inline comments.
type StatusUpdate struct {
	// Fingerprint identifies which finding this update applies to.
	Fingerprint domain.FindingFingerprint

	// NewStatus is the status to transition to.
	NewStatus domain.FindingStatus

	// Reason is the full text of the reply that triggered the update.
	// This is truncated to MaxStatusReasonLength if necessary.
	Reason string

	// UpdatedBy is the GitHub username who posted the reply.
	UpdatedBy string

	// UpdatedAt is when the status-changing reply was posted.
	UpdatedAt time.Time

	// CommentID is the GitHub comment ID of the reply (for audit/debugging).
	CommentID int64
}

// DetectStatusUpdates analyzes grouped comments to detect status changes.
// For each parent comment (bot's inline finding comment), it checks replies
// for status update keywords. Only the latest relevant reply is used if multiple
// replies contain keywords.
//
// Returns a slice of StatusUpdate for each finding where a status change was detected.
// Findings without fingerprints or without keyword-containing replies are skipped.
func DetectStatusUpdates(groupedComments []CommentWithReplies) []StatusUpdate {
	var updates []StatusUpdate

	for _, group := range groupedComments {
		// Extract fingerprint from parent comment
		fingerprint, found := ExtractFingerprintFromComment(group.Parent.Body)
		if !found {
			// Legacy comment without fingerprint - can't match to finding
			continue
		}

		// Find the latest reply with a status keyword
		update := findLatestStatusUpdate(group.Replies, fingerprint)
		if update != nil {
			updates = append(updates, *update)
		}
	}

	return updates
}

// findLatestStatusUpdate scans replies (in chronological order) and returns
// the status update from the latest reply that contains a status keyword.
// Returns nil if no replies contain status keywords.
func findLatestStatusUpdate(replies []PullRequestComment, fingerprint domain.FindingFingerprint) *StatusUpdate {
	var latest *StatusUpdate

	// Replies are assumed to be sorted chronologically (oldest first)
	// We iterate through all to find the latest keyword-containing reply
	for i := range replies {
		reply := &replies[i]

		status, reason, found := domain.ParseStatusKeyword(reply.Body)
		if !found {
			continue
		}

		// Parse the timestamp
		updatedAt, err := time.Parse(time.RFC3339, reply.CreatedAt)
		if err != nil {
			// Log but continue - use zero time rather than skipping
			log.Printf("warning: failed to parse reply timestamp %q: %v", reply.CreatedAt, err)
			updatedAt = time.Time{}
		}

		// This reply has a keyword - it supersedes any previous one
		latest = &StatusUpdate{
			Fingerprint: fingerprint,
			NewStatus:   status,
			Reason:      reason,
			UpdatedBy:   reply.User.Login,
			UpdatedAt:   updatedAt,
			CommentID:   reply.ID,
		}
	}

	return latest
}

// ApplyStatusUpdates applies detected status updates to a tracking state.
// For each update, it looks up the finding by fingerprint and calls UpdateStatus.
// Returns the number of updates successfully applied.
//
// Updates are only applied if:
// - The fingerprint exists in the findings map
// - The finding's current status is open (already acknowledged/disputed findings are not changed)
//
// This function modifies the findings map in place.
func ApplyStatusUpdates(
	findings map[domain.FindingFingerprint]domain.TrackedFinding,
	updates []StatusUpdate,
	currentCommit string,
) (applied int) {
	for _, update := range updates {
		finding, exists := findings[update.Fingerprint]
		if !exists {
			// Fingerprint not found - might be from a previous review
			continue
		}

		// Only update if status is currently open
		// Once acknowledged/disputed, user must fix the code to change status
		if finding.Status != domain.FindingStatusOpen {
			continue
		}

		// Apply the status update
		err := finding.UpdateStatus(update.NewStatus, update.Reason, currentCommit, update.UpdatedAt)
		if err != nil {
			log.Printf("warning: failed to apply status update for %s: %v", update.Fingerprint, err)
			continue
		}

		// Write back to map
		findings[update.Fingerprint] = finding
		applied++
	}

	return applied
}
