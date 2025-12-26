package github_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/diff"
	"github.com/bkyoung/code-reviewer/internal/domain"
	usecasegithub "github.com/bkyoung/code-reviewer/internal/usecase/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockReviewClient is a mock implementation of the ReviewClient interface.
// It uses a mutex to protect shared state for thread safety in concurrent scenarios.
type MockReviewClient struct {
	mu                          sync.Mutex
	CreateReviewFunc            func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error)
	ListReviewsFunc             func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error)
	DismissReviewFunc           func(ctx context.Context, owner, repo string, pullNumber int, reviewID int64, message string) (*github.DismissReviewResponse, error)
	ListPullRequestCommentsFunc func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error)
	LastInput                   *github.CreateReviewInput
	DismissedIDs                []int64
}

func (m *MockReviewClient) CreateReview(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
	m.mu.Lock()
	m.LastInput = &input
	m.mu.Unlock()
	if m.CreateReviewFunc != nil {
		return m.CreateReviewFunc(ctx, input)
	}
	return &github.CreateReviewResponse{ID: 1}, nil
}

func (m *MockReviewClient) ListReviews(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
	if m.ListReviewsFunc != nil {
		return m.ListReviewsFunc(ctx, owner, repo, pullNumber)
	}
	return []github.ReviewSummary{}, nil
}

func (m *MockReviewClient) DismissReview(ctx context.Context, owner, repo string, pullNumber int, reviewID int64, message string) (*github.DismissReviewResponse, error) {
	m.mu.Lock()
	m.DismissedIDs = append(m.DismissedIDs, reviewID)
	m.mu.Unlock()
	if m.DismissReviewFunc != nil {
		return m.DismissReviewFunc(ctx, owner, repo, pullNumber, reviewID, message)
	}
	return &github.DismissReviewResponse{ID: reviewID, State: "DISMISSED"}, nil
}

func (m *MockReviewClient) ListPullRequestComments(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
	if m.ListPullRequestCommentsFunc != nil {
		return m.ListPullRequestCommentsFunc(ctx, owner, repo, pullNumber)
	}
	return []github.PullRequestComment{}, nil
}

// GetDismissedIDs returns a copy of dismissed IDs in a thread-safe manner.
func (m *MockReviewClient) GetDismissedIDs() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]int64, len(m.DismissedIDs))
	copy(result, m.DismissedIDs)
	return result
}

func TestNewReviewPoster(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	require.NotNil(t, poster)
}

func TestReviewPoster_PostReview_Success(t *testing.T) {
	client := &MockReviewClient{
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{
				ID:      123,
				State:   "APPROVED",
				HTMLURL: "https://github.com/owner/repo/pull/1#review-123",
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	// Low and medium findings don't block by default, so review should APPROVE
	findings := []github.PositionedFinding{
		{
			Finding:      makeFinding("file1.go", 10, "low", "Issue 1"),
			DiffPosition: diff.IntPtr(5),
		},
		{
			Finding:      makeFinding("file2.go", 20, "medium", "Issue 2"),
			DiffPosition: diff.IntPtr(15),
		},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha123",
		Review: domain.Review{
			Summary: "Found 2 issues",
		},
		Findings: findings,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(123), result.ReviewID)
	assert.Equal(t, 2, result.CommentsPosted)
	assert.Equal(t, 0, result.CommentsSkipped)
	assert.Equal(t, github.EventApprove, result.Event) // Non-blocking findings → APPROVE
	assert.Equal(t, "https://github.com/owner/repo/pull/1#review-123", result.HTMLURL)
}

func TestReviewPoster_PostReview_DeterminesEventFromSeverity(t *testing.T) {
	tests := []struct {
		name          string
		severity      string
		expectedEvent github.ReviewEvent
	}{
		{
			name:          "high severity triggers REQUEST_CHANGES",
			severity:      "high",
			expectedEvent: github.EventRequestChanges,
		},
		{
			name:          "critical severity triggers REQUEST_CHANGES",
			severity:      "critical",
			expectedEvent: github.EventRequestChanges,
		},
		{
			name:          "medium severity triggers APPROVE (non-blocking)",
			severity:      "medium",
			expectedEvent: github.EventApprove,
		},
		{
			name:          "low severity triggers APPROVE (non-blocking)",
			severity:      "low",
			expectedEvent: github.EventApprove,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockReviewClient{}
			poster := usecasegithub.NewReviewPoster(client)

			findings := []github.PositionedFinding{
				{
					Finding:      makeFinding("file.go", 1, tt.severity, "issue"),
					DiffPosition: diff.IntPtr(1),
				},
			}

			result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
				Owner:      "owner",
				Repo:       "repo",
				PullNumber: 1,
				CommitSHA:  "sha",
				Findings:   findings,
			})

			require.NoError(t, err)
			assert.Equal(t, tt.expectedEvent, result.Event)
			assert.Equal(t, tt.expectedEvent, client.LastInput.Event)
		})
	}
}

func TestReviewPoster_PostReview_ApprovesOnNoFindings(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
		Review: domain.Review{
			Summary: "Looks good!",
		},
		Findings: []github.PositionedFinding{},
	})

	require.NoError(t, err)
	assert.Equal(t, github.EventApprove, result.Event)
	assert.Equal(t, 0, result.CommentsPosted)
}

func TestReviewPoster_PostReview_SkipsOutOfDiffFindings(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "a"), DiffPosition: diff.IntPtr(1)},
		{Finding: makeFinding("b.go", 2, "high", "b"), DiffPosition: nil}, // Out of diff
		{Finding: makeFinding("c.go", 3, "low", "c"), DiffPosition: nil},  // Out of diff
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
		Findings:   findings,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.CommentsPosted)
	assert.Equal(t, 2, result.CommentsSkipped)
}

func TestReviewPoster_PostReview_ClientError(t *testing.T) {
	expectedErr := errors.New("API error")
	client := &MockReviewClient{
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return nil, expectedErr
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	_, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestReviewPoster_PostReview_UsesSummaryFromReview(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	_, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
		Review: domain.Review{
			Summary: "This is the review summary",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "This is the review summary", client.LastInput.Summary)
}

func TestReviewPoster_PostReview_OverrideEvent(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	// Even with high severity findings, override to COMMENT
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "critical issue"), DiffPosition: diff.IntPtr(1)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:         "owner",
		Repo:          "repo",
		PullNumber:    1,
		CommitSHA:     "sha",
		Findings:      findings,
		OverrideEvent: github.EventComment, // Force COMMENT instead of REQUEST_CHANGES
	})

	require.NoError(t, err)
	assert.Equal(t, github.EventComment, result.Event)
}

func TestReviewPoster_PostReview_WithCustomReviewActions(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	// With default actions, high severity would trigger REQUEST_CHANGES
	// But with custom actions, we configure high to NOT block (just comment)
	// Since no findings trigger REQUEST_CHANGES, we use OnNonBlocking
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "bug"), DiffPosition: diff.IntPtr(1)},
	}

	customActions := github.ReviewActions{
		OnCritical:    "request_changes",
		OnHigh:        "comment", // Override high to NOT block
		OnMedium:      "comment",
		OnLow:         "comment",
		OnClean:       "approve",
		OnNonBlocking: "comment", // When no findings block, use COMMENT
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:         "owner",
		Repo:          "repo",
		PullNumber:    1,
		CommitSHA:     "sha",
		Findings:      findings,
		ReviewActions: customActions,
	})

	require.NoError(t, err)
	// High doesn't block (OnHigh=comment), so uses OnNonBlocking=comment
	assert.Equal(t, github.EventComment, result.Event)
}

func TestReviewPoster_PostReview_OverrideEventTakesPrecedenceOverReviewActions(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	// Both OverrideEvent and ReviewActions are set
	// OverrideEvent should take precedence
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "bug"), DiffPosition: diff.IntPtr(1)},
	}

	customActions := github.ReviewActions{
		OnHigh: "comment", // Would return COMMENT
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:         "owner",
		Repo:          "repo",
		PullNumber:    1,
		CommitSHA:     "sha",
		Findings:      findings,
		ReviewActions: customActions,
		OverrideEvent: github.EventApprove, // This should win
	})

	require.NoError(t, err)
	// OverrideEvent takes precedence
	assert.Equal(t, github.EventApprove, result.Event)
}

func TestReviewPoster_PostReview_ReviewActionsOnClean(t *testing.T) {
	client := &MockReviewClient{}
	poster := usecasegithub.NewReviewPoster(client)

	// No findings = clean code
	// With custom OnClean = "comment", should return COMMENT instead of APPROVE
	customActions := github.ReviewActions{
		OnClean: "comment", // Override clean to comment
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:         "owner",
		Repo:          "repo",
		PullNumber:    1,
		CommitSHA:     "sha",
		Findings:      []github.PositionedFinding{},
		ReviewActions: customActions,
	})

	require.NoError(t, err)
	assert.Equal(t, github.EventComment, result.Event)
}

// Helper to create a finding for tests
func makeFinding(file string, line int, severity, description string) domain.Finding {
	return domain.Finding{
		ID:          "test-id",
		File:        file,
		LineStart:   line,
		LineEnd:     line,
		Severity:    severity,
		Category:    "test",
		Description: description,
	}
}

func TestReviewPoster_PostReview_DismissesBotReviews(t *testing.T) {
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "github-actions[bot]"}, State: "APPROVED"},
				{ID: 101, User: github.User{Login: "github-actions[bot]"}, State: "CHANGES_REQUESTED"},
				{ID: 102, User: github.User{Login: "human-user"}, State: "COMMENTED"},
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 200, HTMLURL: "https://example.com/review"}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "github-actions[bot]",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(200), result.ReviewID)
	assert.Equal(t, 2, result.DismissedCount)
	assert.Equal(t, []int64{100, 101}, client.GetDismissedIDs())
}

func TestReviewPoster_PostReview_CaseInsensitiveBotUsername(t *testing.T) {
	// GitHub usernames are case-insensitive, so "GitHub-Actions[bot]" should match "github-actions[bot]"
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				// Different case than what's configured
				{ID: 100, User: github.User{Login: "GitHub-Actions[bot]"}, State: "APPROVED"},
				{ID: 101, User: github.User{Login: "GITHUB-ACTIONS[BOT]"}, State: "CHANGES_REQUESTED"},
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 200, HTMLURL: "https://example.com/review"}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "github-actions[bot]", // lowercase
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.DismissedCount, "should dismiss both reviews despite case difference")
	assert.Equal(t, []int64{100, 101}, client.GetDismissedIDs())
}

func TestReviewPoster_PostReview_NoDismissWhenBotUsernameEmpty(t *testing.T) {
	listCalled := false
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			listCalled = true
			return []github.ReviewSummary{}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "", // Empty - no dismiss
	})

	require.NoError(t, err)
	assert.False(t, listCalled)
	assert.Equal(t, 0, result.DismissedCount)
}

func TestReviewPoster_PostReview_SkipsAlreadyDismissedReviews(t *testing.T) {
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "bot[bot]"}, State: "DISMISSED"},
				{ID: 101, User: github.User{Login: "bot[bot]"}, State: "APPROVED"},
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	require.NoError(t, err)
	// Only the APPROVED one should be dismissed, not the already DISMISSED one
	assert.Equal(t, 1, result.DismissedCount)
	assert.Equal(t, []int64{101}, client.GetDismissedIDs())
}

func TestReviewPoster_PostReview_SkipsPendingReviews(t *testing.T) {
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "bot[bot]"}, State: "PENDING"},
				{ID: 101, User: github.User{Login: "bot[bot]"}, State: "COMMENTED"},
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	require.NoError(t, err)
	// Only COMMENTED should be dismissed, not PENDING
	assert.Equal(t, 1, result.DismissedCount)
	assert.Equal(t, []int64{101}, client.GetDismissedIDs())
}

func TestReviewPoster_PostReview_ListFailureContinues(t *testing.T) {
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return nil, errors.New("list failed")
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 200}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	// Should succeed despite list failure
	require.NoError(t, err)
	assert.Equal(t, int64(200), result.ReviewID)
	assert.Equal(t, 0, result.DismissedCount)
}

func TestReviewPoster_PostReview_DismissFailureContinues(t *testing.T) {
	dismissCalls := 0
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "bot[bot]"}, State: "APPROVED"},
				{ID: 101, User: github.User{Login: "bot[bot]"}, State: "CHANGES_REQUESTED"},
			}, nil
		},
		DismissReviewFunc: func(ctx context.Context, owner, repo string, pullNumber int, reviewID int64, message string) (*github.DismissReviewResponse, error) {
			dismissCalls++
			if reviewID == 100 {
				return nil, errors.New("dismiss failed")
			}
			return &github.DismissReviewResponse{ID: reviewID, State: "DISMISSED"}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 200}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	// Should succeed despite partial dismiss failure
	require.NoError(t, err)
	assert.Equal(t, int64(200), result.ReviewID)
	assert.Equal(t, 2, dismissCalls) // Both were attempted
	assert.Equal(t, 1, result.DismissedCount)
}

func TestReviewPoster_PostReview_NoBotReviewsToDissmiss(t *testing.T) {
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "human-user"}, State: "APPROVED"},
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	require.NoError(t, err)
	assert.Equal(t, 0, result.DismissedCount)
	assert.Empty(t, client.GetDismissedIDs())
}

func TestReviewPoster_PostReview_NoDismissalOnCreateFailure(t *testing.T) {
	// Verify that if CreateReview fails, no reviews are dismissed.
	// This ensures the PR always maintains review signal.
	listCalled := false
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			listCalled = true
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "bot[bot]"}, State: "APPROVED"},
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return nil, errors.New("create review failed")
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	_, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create review failed")
	// ListReviews should NOT have been called since dismissal happens after CreateReview
	assert.False(t, listCalled, "ListReviews should not be called when CreateReview fails")
	// No reviews should have been dismissed
	assert.Empty(t, client.GetDismissedIDs())
}

func TestReviewPoster_PostReview_SkipsNewlyCreatedReview(t *testing.T) {
	// Verify that the newly created review is not dismissed.
	// This prevents the bot from dismissing its own fresh review.
	const newReviewID = int64(200)
	client := &MockReviewClient{
		ListReviewsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.ReviewSummary, error) {
			return []github.ReviewSummary{
				{ID: 100, User: github.User{Login: "bot[bot]"}, State: "APPROVED"},          // Old review - should be dismissed
				{ID: newReviewID, User: github.User{Login: "bot[bot]"}, State: "COMMENTED"}, // New review - should NOT be dismissed
				{ID: 101, User: github.User{Login: "bot[bot]"}, State: "CHANGES_REQUESTED"}, // Old review - should be dismissed
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: newReviewID, State: "COMMENTED"}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		BotUsername: "bot[bot]",
	})

	require.NoError(t, err)
	assert.Equal(t, newReviewID, result.ReviewID)
	// Only old reviews should be dismissed, not the newly created one
	assert.Equal(t, 2, result.DismissedCount)
	assert.Contains(t, client.GetDismissedIDs(), int64(100))
	assert.Contains(t, client.GetDismissedIDs(), int64(101))
	assert.NotContains(t, client.GetDismissedIDs(), newReviewID, "newly created review should not be dismissed")
}

// ==== Deduplication Tests (Issue #107) ====

func TestReviewPoster_PostReview_DeduplicatesFindings(t *testing.T) {
	// Simulate a previous bot comment with an embedded fingerprint.
	// The fingerprint matches one of the findings we're about to post.
	finding1 := makeFinding("file1.go", 10, "high", "Issue already posted")
	finding2 := makeFinding("file2.go", 20, "medium", "New issue")

	// Create a comment body with embedded fingerprint for finding1
	fp1 := domain.FingerprintFromFinding(finding1)
	existingCommentBody := "**Severity:** high\n\n<!-- CR_FINGERPRINT:" + string(fp1) + " -->"

	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return []github.PullRequestComment{
				{
					ID:   1,
					Body: existingCommentBody,
					User: github.User{Login: "github-actions[bot]"},
				},
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 123, HTMLURL: "https://example.com/review"}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: finding1, DiffPosition: diff.IntPtr(5)},  // Already posted - should be skipped
		{Finding: finding2, DiffPosition: diff.IntPtr(15)}, // New - should be posted
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "github-actions[bot]",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.CommentsPosted, "only new finding should be posted")
	assert.Equal(t, 1, result.DuplicatesSkipped, "one duplicate should be skipped")
	// Verify the client only received 1 finding (the non-duplicate)
	require.NotNil(t, client.LastInput)
	assert.Len(t, client.LastInput.Findings, 1)
}

func TestReviewPoster_PostReview_NoDuplicatesWhenNoExistingComments(t *testing.T) {
	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return []github.PullRequestComment{}, nil // No existing comments
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: makeFinding("file1.go", 10, "high", "Issue 1"), DiffPosition: diff.IntPtr(5)},
		{Finding: makeFinding("file2.go", 20, "medium", "Issue 2"), DiffPosition: diff.IntPtr(15)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "github-actions[bot]",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.CommentsPosted)
	assert.Equal(t, 0, result.DuplicatesSkipped)
}

func TestReviewPoster_PostReview_IgnoresNonBotComments(t *testing.T) {
	finding := makeFinding("file1.go", 10, "high", "Issue from human")
	fp := domain.FingerprintFromFinding(finding)
	humanCommentBody := "**Severity:** high\n\n<!-- CR_FINGERPRINT:" + string(fp) + " -->"

	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return []github.PullRequestComment{
				{
					ID:   1,
					Body: humanCommentBody,
					User: github.User{Login: "human-user"}, // Not the bot
				},
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: finding, DiffPosition: diff.IntPtr(5)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "github-actions[bot]",
	})

	require.NoError(t, err)
	// The finding should NOT be deduplicated because the comment is from a human
	assert.Equal(t, 1, result.CommentsPosted)
	assert.Equal(t, 0, result.DuplicatesSkipped)
}

func TestReviewPoster_PostReview_IgnoresCommentsWithoutFingerprint(t *testing.T) {
	// Legacy comments without fingerprints should be ignored
	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return []github.PullRequestComment{
				{
					ID:   1,
					Body: "This is an old comment without a fingerprint",
					User: github.User{Login: "github-actions[bot]"},
				},
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: makeFinding("file1.go", 10, "high", "New issue"), DiffPosition: diff.IntPtr(5)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "github-actions[bot]",
	})

	require.NoError(t, err)
	// The finding should be posted because no valid fingerprint was found
	assert.Equal(t, 1, result.CommentsPosted)
	assert.Equal(t, 0, result.DuplicatesSkipped)
}

func TestReviewPoster_PostReview_DeduplicationDisabledWithoutBotUsername(t *testing.T) {
	// When BotUsername is empty, deduplication is disabled
	listCalled := false
	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			listCalled = true
			return []github.PullRequestComment{}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: makeFinding("file1.go", 10, "high", "Issue"), DiffPosition: diff.IntPtr(5)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "", // No bot username
	})

	require.NoError(t, err)
	assert.False(t, listCalled, "ListPullRequestComments should not be called when BotUsername is empty")
	assert.Equal(t, 1, result.CommentsPosted)
	assert.Equal(t, 0, result.DuplicatesSkipped)
}

func TestReviewPoster_PostReview_CommentFetchErrorContinues(t *testing.T) {
	// If fetching comments fails, continue without deduplication
	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return nil, errors.New("API error")
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 123}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: makeFinding("file1.go", 10, "high", "Issue"), DiffPosition: diff.IntPtr(5)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "bot[bot]",
	})

	// Should succeed despite fetch error
	require.NoError(t, err)
	assert.Equal(t, 1, result.CommentsPosted)
	assert.Equal(t, 0, result.DuplicatesSkipped)
}

func TestReviewPoster_PostReview_AllFindingsDeduplicated(t *testing.T) {
	// When all findings are duplicates, an empty review should still be posted
	finding := makeFinding("file1.go", 10, "high", "Already posted")
	fp := domain.FingerprintFromFinding(finding)
	existingCommentBody := "<!-- CR_FINGERPRINT:" + string(fp) + " -->"

	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return []github.PullRequestComment{
				{
					ID:   1,
					Body: existingCommentBody,
					User: github.User{Login: "bot[bot]"},
				},
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 123}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: finding, DiffPosition: diff.IntPtr(5)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "bot[bot]",
	})

	require.NoError(t, err)
	assert.Equal(t, 0, result.CommentsPosted)
	assert.Equal(t, 1, result.DuplicatesSkipped)
	// Verify the review was still created (with no comments)
	require.NotNil(t, client.LastInput)
	assert.Len(t, client.LastInput.Findings, 0)
}

func TestReviewPoster_PostReview_CaseInsensitiveBotUsernameForDedup(t *testing.T) {
	// Bot username matching should be case-insensitive
	finding := makeFinding("file1.go", 10, "high", "Issue")
	fp := domain.FingerprintFromFinding(finding)
	existingCommentBody := "<!-- CR_FINGERPRINT:" + string(fp) + " -->"

	client := &MockReviewClient{
		ListPullRequestCommentsFunc: func(ctx context.Context, owner, repo string, pullNumber int) ([]github.PullRequestComment, error) {
			return []github.PullRequestComment{
				{
					ID:   1,
					Body: existingCommentBody,
					User: github.User{Login: "GitHub-Actions[BOT]"}, // Different case
				},
			}, nil
		},
		CreateReviewFunc: func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
			return &github.CreateReviewResponse{ID: 123}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

	findings := []github.PositionedFinding{
		{Finding: finding, DiffPosition: diff.IntPtr(5)},
	}

	result, err := poster.PostReview(context.Background(), usecasegithub.PostReviewRequest{
		Owner:       "owner",
		Repo:        "repo",
		PullNumber:  1,
		CommitSHA:   "sha",
		Findings:    findings,
		BotUsername: "github-actions[bot]", // lowercase
	})

	require.NoError(t, err)
	assert.Equal(t, 0, result.CommentsPosted, "finding should be deduplicated despite case difference")
	assert.Equal(t, 1, result.DuplicatesSkipped)
}
