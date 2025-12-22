package github_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/diff"
	"github.com/bkyoung/code-reviewer/internal/domain"
	usecasegithub "github.com/bkyoung/code-reviewer/internal/usecase/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockReviewClient is a mock implementation of the ReviewClient interface.
type MockReviewClient struct {
	CreateReviewFunc func(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error)
	LastInput        *github.CreateReviewInput
}

func (m *MockReviewClient) CreateReview(ctx context.Context, input github.CreateReviewInput) (*github.CreateReviewResponse, error) {
	m.LastInput = &input
	if m.CreateReviewFunc != nil {
		return m.CreateReviewFunc(ctx, input)
	}
	return &github.CreateReviewResponse{ID: 1}, nil
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
				State:   "COMMENTED",
				HTMLURL: "https://github.com/owner/repo/pull/1#review-123",
			}, nil
		},
	}
	poster := usecasegithub.NewReviewPoster(client)

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
	assert.Equal(t, github.EventComment, result.Event)
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
			name:          "medium severity triggers COMMENT",
			severity:      "medium",
			expectedEvent: github.EventComment,
		},
		{
			name:          "low severity triggers COMMENT",
			severity:      "low",
			expectedEvent: github.EventComment,
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
