package github_test

import (
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectStatusUpdates_SingleAcknowledge(t *testing.T) {
	fingerprint := domain.FindingFingerprint("abc123")

	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{
				ID:   1,
				Body: "Finding comment<!-- CR_FINGERPRINT:" + string(fingerprint) + " -->",
			},
			Replies: []github.PullRequestComment{
				{
					ID:        2,
					Body:      "acknowledged - this is intentional",
					CreatedAt: "2024-01-15T10:00:00Z",
					User:      github.User{Login: "developer"},
				},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	require.Len(t, updates, 1)
	assert.Equal(t, fingerprint, updates[0].Fingerprint)
	assert.Equal(t, domain.FindingStatusAcknowledged, updates[0].NewStatus)
	assert.Contains(t, updates[0].Reason, "acknowledged")
	assert.Equal(t, "developer", updates[0].UpdatedBy)
}

func TestDetectStatusUpdates_SingleDispute(t *testing.T) {
	fingerprint := domain.FindingFingerprint("def456")

	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{
				ID:   1,
				Body: "<!-- CR_FINGERPRINT:" + string(fingerprint) + " -->Issue",
			},
			Replies: []github.PullRequestComment{
				{
					ID:        2,
					Body:      "false positive - this is not actually a problem",
					CreatedAt: "2024-01-15T11:00:00Z",
					User:      github.User{Login: "alice"},
				},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	require.Len(t, updates, 1)
	assert.Equal(t, fingerprint, updates[0].Fingerprint)
	assert.Equal(t, domain.FindingStatusDisputed, updates[0].NewStatus)
	assert.Contains(t, updates[0].Reason, "false positive")
}

func TestDetectStatusUpdates_MultipleReplies_UsesLatest(t *testing.T) {
	fingerprint := domain.FindingFingerprint("xyz789")

	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{
				ID:   1,
				Body: "<!-- CR_FINGERPRINT:" + string(fingerprint) + " -->",
			},
			Replies: []github.PullRequestComment{
				{
					ID:        2,
					Body:      "acknowledged", // Earlier reply
					CreatedAt: "2024-01-15T10:00:00Z",
					User:      github.User{Login: "dev1"},
				},
				{
					ID:        3,
					Body:      "disputed - actually this is fine", // Later reply supersedes
					CreatedAt: "2024-01-15T11:00:00Z",
					User:      github.User{Login: "dev2"},
				},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	// Should only return the latest status update
	require.Len(t, updates, 1)
	assert.Equal(t, domain.FindingStatusDisputed, updates[0].NewStatus)
	assert.Equal(t, "dev2", updates[0].UpdatedBy)
}

func TestDetectStatusUpdates_NoFingerprint(t *testing.T) {
	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{
				ID:   1,
				Body: "Legacy comment without fingerprint",
			},
			Replies: []github.PullRequestComment{
				{
					ID:   2,
					Body: "acknowledged",
				},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	// No fingerprint = can't match, so no updates
	assert.Empty(t, updates)
}

func TestDetectStatusUpdates_NoKeywordInReplies(t *testing.T) {
	fingerprint := domain.FindingFingerprint("abc123")

	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{
				ID:   1,
				Body: "<!-- CR_FINGERPRINT:" + string(fingerprint) + " -->",
			},
			Replies: []github.PullRequestComment{
				{
					ID:        2,
					Body:      "Thanks for the review!",
					CreatedAt: "2024-01-15T10:00:00Z",
				},
				{
					ID:        3,
					Body:      "I'll look into this tomorrow",
					CreatedAt: "2024-01-15T11:00:00Z",
				},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	// No status keywords found, so no updates
	assert.Empty(t, updates)
}

func TestDetectStatusUpdates_MultipleFindings(t *testing.T) {
	fp1 := domain.FindingFingerprint("finding1")
	fp2 := domain.FindingFingerprint("finding2")
	fp3 := domain.FindingFingerprint("finding3")

	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{ID: 1, Body: "<!-- CR_FINGERPRINT:" + string(fp1) + " -->"},
			Replies: []github.PullRequestComment{
				{ID: 2, Body: "acknowledged", CreatedAt: "2024-01-15T10:00:00Z", User: github.User{Login: "dev"}},
			},
		},
		{
			Parent:  github.PullRequestComment{ID: 3, Body: "<!-- CR_FINGERPRINT:" + string(fp2) + " -->"},
			Replies: []github.PullRequestComment{}, // No replies
		},
		{
			Parent: github.PullRequestComment{ID: 4, Body: "<!-- CR_FINGERPRINT:" + string(fp3) + " -->"},
			Replies: []github.PullRequestComment{
				{ID: 5, Body: "not a bug", CreatedAt: "2024-01-15T11:00:00Z", User: github.User{Login: "alice"}},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	require.Len(t, updates, 2)

	// Check fp1 acknowledged
	var fp1Update, fp3Update *github.StatusUpdate
	for i := range updates {
		if updates[i].Fingerprint == fp1 {
			fp1Update = &updates[i]
		}
		if updates[i].Fingerprint == fp3 {
			fp3Update = &updates[i]
		}
	}

	require.NotNil(t, fp1Update)
	assert.Equal(t, domain.FindingStatusAcknowledged, fp1Update.NewStatus)

	require.NotNil(t, fp3Update)
	assert.Equal(t, domain.FindingStatusDisputed, fp3Update.NewStatus)
}

func TestDetectStatusUpdates_EmptyComments(t *testing.T) {
	updates := github.DetectStatusUpdates([]github.CommentWithReplies{})
	assert.Empty(t, updates)
}

func TestApplyStatusUpdates(t *testing.T) {
	fp1 := domain.FindingFingerprint("fp1")
	fp2 := domain.FindingFingerprint("fp2")
	fp3 := domain.FindingFingerprint("fp3")

	now := time.Now()
	findings := map[domain.FindingFingerprint]domain.TrackedFinding{
		fp1: {
			Fingerprint: fp1,
			Status:      domain.FindingStatusOpen,
			Finding:     domain.Finding{ID: "f1"},
			FirstSeen:   now,
			LastSeen:    now,
			SeenCount:   1,
		},
		fp2: {
			Fingerprint: fp2,
			Status:      domain.FindingStatusAcknowledged, // Already acknowledged
			Finding:     domain.Finding{ID: "f2"},
			FirstSeen:   now,
			LastSeen:    now,
			SeenCount:   1,
		},
		fp3: {
			Fingerprint: fp3,
			Status:      domain.FindingStatusOpen,
			Finding:     domain.Finding{ID: "f3"},
			FirstSeen:   now,
			LastSeen:    now,
			SeenCount:   1,
		},
	}

	updates := []github.StatusUpdate{
		{
			Fingerprint: fp1,
			NewStatus:   domain.FindingStatusAcknowledged,
			Reason:      "acknowledged - by design",
			UpdatedBy:   "dev",
			UpdatedAt:   now,
		},
		{
			Fingerprint: fp2, // Already acknowledged - should be skipped
			NewStatus:   domain.FindingStatusDisputed,
			Reason:      "disputed",
			UpdatedBy:   "other",
			UpdatedAt:   now,
		},
		{
			Fingerprint: domain.FindingFingerprint("unknown"), // Not found
			NewStatus:   domain.FindingStatusAcknowledged,
			Reason:      "ack",
			UpdatedBy:   "dev",
			UpdatedAt:   now,
		},
	}

	applied := github.ApplyStatusUpdates(findings, updates, "commit123")

	// Only fp1 should be updated
	assert.Equal(t, 1, applied)
	assert.Equal(t, domain.FindingStatusAcknowledged, findings[fp1].Status)
	assert.Equal(t, "acknowledged - by design", findings[fp1].StatusReason)

	// fp2 should remain unchanged (was already acknowledged)
	assert.Equal(t, domain.FindingStatusAcknowledged, findings[fp2].Status)

	// fp3 should remain open (no update for it)
	assert.Equal(t, domain.FindingStatusOpen, findings[fp3].Status)
}

func TestApplyStatusUpdates_Empty(t *testing.T) {
	findings := map[domain.FindingFingerprint]domain.TrackedFinding{}
	applied := github.ApplyStatusUpdates(findings, []github.StatusUpdate{}, "commit")
	assert.Equal(t, 0, applied)
}

func TestStatusUpdate_UpdatedAtParsing(t *testing.T) {
	fingerprint := domain.FindingFingerprint("abc123")

	comments := []github.CommentWithReplies{
		{
			Parent: github.PullRequestComment{
				ID:   1,
				Body: "<!-- CR_FINGERPRINT:" + string(fingerprint) + " -->",
			},
			Replies: []github.PullRequestComment{
				{
					ID:        2,
					Body:      "acknowledged",
					CreatedAt: "2024-01-15T10:30:00Z",
					User:      github.User{Login: "dev"},
				},
			},
		},
	}

	updates := github.DetectStatusUpdates(comments)

	require.Len(t, updates, 1)

	expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	assert.Equal(t, expected, updates[0].UpdatedAt)
}
