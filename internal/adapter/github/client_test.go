package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := github.NewClient("test-token")

	require.NotNil(t, client)
}

func TestSetBaseURL_TrimsTrailingSlashes(t *testing.T) {
	// Test that ALL trailing slashes are normalized to prevent double-slash URLs
	testCases := []struct {
		name   string
		suffix string
	}{
		{"single slash", "/"},
		{"double slash", "//"},
		{"triple slash", "///"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify no double slashes in path
				assert.NotContains(t, r.URL.Path, "//", "URL should not contain double slashes")
				assert.Equal(t, "/repos/owner/repo/pulls/1/reviews", r.URL.Path)

				resp := github.CreateReviewResponse{ID: 1, State: "COMMENTED", HTMLURL: "https://example.com"}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			client := github.NewClient("test-token")
			// Set base URL WITH trailing slashes - should all be normalized
			client.SetBaseURL(server.URL + tc.suffix)

			_, err := client.CreateReview(context.Background(), github.CreateReviewInput{
				Owner:      "owner",
				Repo:       "repo",
				PullNumber: 1,
				CommitSHA:  "abc123",
				Event:      github.EventComment,
			})
			require.NoError(t, err)
		})
	}
}

func TestClient_CreateReview_Success(t *testing.T) {
	requestReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/repos/owner/repo/pulls/123/reviews", r.URL.Path)

		// Verify headers
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))

		// Parse and verify request body
		var req github.CreateReviewRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "sha123", req.CommitID)
		assert.Equal(t, github.EventComment, req.Event)
		assert.Equal(t, "Review summary", req.Body)
		assert.Len(t, req.Comments, 2)

		// Send response
		resp := github.CreateReviewResponse{
			ID:      456,
			State:   "COMMENTED",
			HTMLURL: "https://github.com/owner/repo/pull/123#pullrequestreview-456",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	findings := []github.PositionedFinding{
		{
			Finding:      makeFinding("file1.go", 10, "low", "Issue 1"),
			DiffPosition: diff.IntPtr(5),
		},
		{
			Finding:      makeFinding("file2.go", 20, "low", "Issue 2"),
			DiffPosition: diff.IntPtr(15),
		},
	}

	resp, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 123,
		CommitSHA:  "sha123",
		Event:      github.EventComment,
		Summary:    "Review summary",
		Findings:   findings,
	})

	require.NoError(t, err)
	require.True(t, requestReceived)
	assert.Equal(t, int64(456), resp.ID)
	assert.Equal(t, "COMMENTED", resp.State)
}

func TestClient_CreateReview_FiltersDiffPosition(t *testing.T) {
	var receivedCommentCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req github.CreateReviewRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedCommentCount = len(req.Comments)

		resp := github.CreateReviewResponse{ID: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	// Mix of in-diff and out-of-diff findings
	findings := []github.PositionedFinding{
		{Finding: makeFinding("a.go", 1, "high", "a"), DiffPosition: diff.IntPtr(1)},
		{Finding: makeFinding("b.go", 2, "low", "b"), DiffPosition: nil}, // Out of diff
		{Finding: makeFinding("c.go", 3, "low", "c"), DiffPosition: diff.IntPtr(3)},
	}

	_, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
		Event:      github.EventComment,
		Summary:    "Test",
		Findings:   findings,
	})

	require.NoError(t, err)
	assert.Equal(t, 2, receivedCommentCount) // Only 2 in-diff findings
}

func TestClient_CreateReview_AuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(github.GitHubErrorResponse{
			Message: "Bad credentials",
		})
	}))
	defer server.Close()

	client := github.NewClient("bad-token")
	client.SetBaseURL(server.URL)

	_, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}

func TestClient_CreateReview_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(github.GitHubErrorResponse{
			Message: "Not Found",
		})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "nonexistent",
		Repo:       "repo",
		PullNumber: 999,
		CommitSHA:  "sha",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestClient_CreateReview_ValidationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(github.GitHubErrorResponse{
			Message: "Validation Failed",
			Errors: []struct {
				Resource string `json:"resource"`
				Field    string `json:"field"`
				Code     string `json:"code"`
				Message  string `json:"message"`
			}{
				{Field: "position", Code: "invalid"},
			},
		})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestClient_CreateReview_RateLimitWithRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(github.GitHubErrorResponse{
				Message: "API rate limit exceeded",
			})
			return
		}
		// Succeed on third try
		resp := github.CreateReviewResponse{ID: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)
	client.SetMaxRetries(5)
	client.SetInitialBackoff(10 * time.Millisecond) // Fast for testing

	resp, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, 3, callCount)
}

func TestClient_CreateReview_ServerError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(github.GitHubErrorResponse{
				Message: "Service temporarily unavailable",
			})
			return
		}
		// Succeed on retry
		resp := github.CreateReviewResponse{ID: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)
	client.SetMaxRetries(3)
	client.SetInitialBackoff(10 * time.Millisecond)

	resp, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, 2, callCount)
}

func TestClient_CreateReview_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Slow response
		resp := github.CreateReviewResponse{ID: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.CreateReview(ctx, github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestClient_CreateReview_EmptyFindings(t *testing.T) {
	var receivedRequest github.CreateReviewRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedRequest)
		resp := github.CreateReviewResponse{ID: 1, State: "APPROVED"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	resp, err := client.CreateReview(context.Background(), github.CreateReviewInput{
		Owner:      "owner",
		Repo:       "repo",
		PullNumber: 1,
		CommitSHA:  "sha",
		Event:      github.EventApprove,
		Summary:    "LGTM!",
		Findings:   []github.PositionedFinding{},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.ID)
	assert.Empty(t, receivedRequest.Comments)
	assert.Equal(t, "LGTM!", receivedRequest.Body)
}

func TestClient_ListReviews_Success(t *testing.T) {
	requestReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify request method and path
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/repos/owner/repo/pulls/123/reviews", r.URL.Path)

		// Verify pagination parameter (max per_page)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))

		// Verify headers
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))

		// Send response
		reviews := []github.ReviewSummary{
			{
				ID:          100,
				User:        github.User{Login: "github-actions[bot]", Type: "Bot"},
				State:       "APPROVED",
				SubmittedAt: "2024-01-01T00:00:00Z",
			},
			{
				ID:          101,
				User:        github.User{Login: "human-reviewer", Type: "User"},
				State:       "CHANGES_REQUESTED",
				SubmittedAt: "2024-01-02T00:00:00Z",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.NoError(t, err)
	require.True(t, requestReceived)
	require.Len(t, reviews, 2)
	assert.Equal(t, int64(100), reviews[0].ID)
	assert.Equal(t, "github-actions[bot]", reviews[0].User.Login)
	assert.Equal(t, "APPROVED", reviews[0].State)
	assert.Equal(t, int64(101), reviews[1].ID)
	assert.Equal(t, "human-reviewer", reviews[1].User.Login)
}

func TestClient_ListReviews_Pagination(t *testing.T) {
	pageCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		switch pageCount {
		case 1:
			// First page - return Link header with next page (use full URL with scheme)
			w.Header().Set("Link", `<`+serverURL+`/repos/owner/repo/pulls/123/reviews?per_page=100&page=2>; rel="next", <`+serverURL+`/repos/owner/repo/pulls/123/reviews?per_page=100&page=3>; rel="last"`)
			reviews := []github.ReviewSummary{
				{ID: 1, User: github.User{Login: "bot"}, State: "APPROVED"},
				{ID: 2, User: github.User{Login: "bot"}, State: "COMMENTED"},
			}
			json.NewEncoder(w).Encode(reviews)
		case 2:
			// Second page - has another page
			w.Header().Set("Link", `<`+serverURL+`/repos/owner/repo/pulls/123/reviews?per_page=100&page=3>; rel="next", <`+serverURL+`/repos/owner/repo/pulls/123/reviews?per_page=100&page=3>; rel="last"`)
			reviews := []github.ReviewSummary{
				{ID: 3, User: github.User{Login: "human"}, State: "CHANGES_REQUESTED"},
			}
			json.NewEncoder(w).Encode(reviews)
		case 3:
			// Last page - no next link
			reviews := []github.ReviewSummary{
				{ID: 4, User: github.User{Login: "bot"}, State: "DISMISSED"},
			}
			json.NewEncoder(w).Encode(reviews)
		default:
			t.Fatal("unexpected page request")
		}
	}))
	defer server.Close()
	serverURL = server.URL // Set after server starts so we have the actual URL

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.NoError(t, err)
	assert.Equal(t, 3, pageCount, "should have fetched all 3 pages")
	require.Len(t, reviews, 4, "should have all 4 reviews from 3 pages")
	assert.Equal(t, int64(1), reviews[0].ID)
	assert.Equal(t, int64(2), reviews[1].ID)
	assert.Equal(t, int64(3), reviews[2].ID)
	assert.Equal(t, int64(4), reviews[3].ID)
}

func TestClient_ListReviews_SSRFProtection_DifferentHost(t *testing.T) {
	// Test that the client returns an error when Link header points to untrusted host
	// This prevents SSRF attacks via malicious Link header manipulation
	pageCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		if pageCount == 1 {
			// First page - return Link header pointing to a DIFFERENT host (attacker-controlled)
			// Use http:// to match test server scheme, so we test host validation specifically
			w.Header().Set("Link", `<http://evil-attacker.com/steal-token?page=2>; rel="next"`)
			reviews := []github.ReviewSummary{
				{ID: 1, User: github.User{Login: "bot"}, State: "APPROVED"},
			}
			json.NewEncoder(w).Encode(reviews)
		} else {
			// This should never be reached
			t.Fatal("client followed untrusted Link header - SSRF vulnerability!")
		}
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	// Should return error instead of silently truncating
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe pagination URL")
	assert.Contains(t, err.Error(), "untrusted host")
	assert.Equal(t, 1, pageCount, "should only fetch first page")
}

func TestClient_ListReviews_SSRFProtection_SchemeDowngrade(t *testing.T) {
	// Test that we reject https->http downgrades but allow http->https upgrades
	// This is tested via the validateAndResolvePaginationURL function directly
	// since httptest servers use http and we can't easily test https->http

	client := github.NewClient("test-token")

	// Simulate an https base URL (production scenario)
	client.SetBaseURL("https://api.github.com")

	// Test: https base -> http link should be rejected (downgrade attack)
	_, err := client.ValidateAndResolvePaginationURL("http://api.github.com/repos/owner/repo/pulls/1/reviews?page=2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme downgrade not allowed")

	// Test: http base -> https link should be allowed (upgrade is safe)
	client.SetBaseURL("http://localhost:8080")
	resolved, err := client.ValidateAndResolvePaginationURL("https://localhost:8080/repos/owner/repo/pulls/1/reviews?page=2")
	require.NoError(t, err)
	assert.Equal(t, "https://localhost:8080/repos/owner/repo/pulls/1/reviews?page=2", resolved)
}

func TestClient_ListReviews_RelativeURLResolution(t *testing.T) {
	// Test that the client correctly resolves relative URLs in Link header
	// This supports various GitHub/enterprise configurations
	pageCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		if pageCount == 1 {
			// Return relative URL (no scheme/host) - should be resolved against baseURL
			w.Header().Set("Link", `</repos/owner/repo/pulls/123/reviews?page=2>; rel="next"`)
			reviews := []github.ReviewSummary{
				{ID: 1, User: github.User{Login: "bot"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
			}
			json.NewEncoder(w).Encode(reviews)
		} else {
			// Second page - no more pages
			reviews := []github.ReviewSummary{
				{ID: 2, User: github.User{Login: "bot"}, State: "COMMENTED", SubmittedAt: "2024-01-02T00:00:00Z"},
			}
			json.NewEncoder(w).Encode(reviews)
		}
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.NoError(t, err)
	assert.Len(t, reviews, 2, "should have fetched both pages")
	assert.Equal(t, 2, pageCount, "should have made two requests")
}

func TestClient_ListReviews_SSRFProtection_WrongPathPrefix(t *testing.T) {
	// Test that the client rejects pagination URLs with unexpected path prefix
	// This prevents redirecting to other endpoints on the same host
	pageCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		if pageCount == 1 {
			// Return Link header pointing to a different API endpoint on same host
			w.Header().Set("Link", `<`+serverURL+`/admin/secrets?page=2>; rel="next"`)
			reviews := []github.ReviewSummary{
				{ID: 1, User: github.User{Login: "bot"}, State: "APPROVED"},
			}
			json.NewEncoder(w).Encode(reviews)
		} else {
			t.Fatal("client followed Link to unexpected path!")
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected API path")
}

func TestClient_ListReviews_GitHubEnterprisePathPrefix(t *testing.T) {
	// Test that pagination works with GitHub Enterprise API path prefix (/api/v3)
	pageCount := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")

		// Verify requests use the /api/v3 prefix
		assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v3/repos/"), "path should have /api/v3 prefix")

		if pageCount == 1 {
			// Return Link with GHES-style path prefix
			w.Header().Set("Link", `<`+serverURL+`/api/v3/repos/owner/repo/pulls/123/reviews?page=2>; rel="next"`)
			reviews := []github.ReviewSummary{
				{ID: 1, User: github.User{Login: "bot"}, State: "APPROVED", SubmittedAt: "2024-01-01T00:00:00Z"},
			}
			json.NewEncoder(w).Encode(reviews)
		} else {
			reviews := []github.ReviewSummary{
				{ID: 2, User: github.User{Login: "bot"}, State: "COMMENTED", SubmittedAt: "2024-01-02T00:00:00Z"},
			}
			json.NewEncoder(w).Encode(reviews)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := github.NewClient("test-token")
	// Set base URL with GHES-style path prefix
	client.SetBaseURL(server.URL + "/api/v3")

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.NoError(t, err)
	assert.Len(t, reviews, 2, "should have fetched both pages")
	assert.Equal(t, 2, pageCount, "should have made two requests")
}

func TestClient_ListReviews_PathEscaping(t *testing.T) {
	// Test that owner/repo with special characters are properly escaped
	// to prevent path injection attacks
	var receivedRawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use RawPath to see the escaped version (before server decoding)
		// If RawPath is empty, the path had no escaping needed
		receivedRawPath = r.URL.RawPath
		if receivedRawPath == "" {
			receivedRawPath = r.URL.Path
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]github.ReviewSummary{})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	// Owner with path traversal attempt - slashes should be escaped
	_, err := client.ListReviews(context.Background(), "owner/../admin", "repo", 123)
	require.NoError(t, err)
	// The RawPath should contain %2F (escaped slash), preventing path traversal
	assert.Contains(t, receivedRawPath, "%2F")
}

func TestClient_ListReviews_ChronologicalOrder(t *testing.T) {
	// Test that reviews are sorted chronologically (oldest first)
	// regardless of the order returned by the API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return reviews out of order - newest first
		reviews := []github.ReviewSummary{
			{ID: 3, SubmittedAt: "2024-01-03T12:00:00Z", State: "APPROVED"},
			{ID: 1, SubmittedAt: "2024-01-01T12:00:00Z", State: "COMMENTED"},
			{ID: 2, SubmittedAt: "2024-01-02T12:00:00Z", State: "CHANGES_REQUESTED"},
		}
		json.NewEncoder(w).Encode(reviews)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.NoError(t, err)
	require.Len(t, reviews, 3)
	// Should be sorted oldest first
	assert.Equal(t, int64(1), reviews[0].ID)
	assert.Equal(t, int64(2), reviews[1].ID)
	assert.Equal(t, int64(3), reviews[2].ID)
}

func TestClient_ListReviews_SortFallbackToID(t *testing.T) {
	// Test that sorting falls back to ID when timestamps are missing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return reviews with missing/invalid timestamps
		reviews := []github.ReviewSummary{
			{ID: 3, SubmittedAt: "", State: "APPROVED"},
			{ID: 1, SubmittedAt: "", State: "COMMENTED"},
			{ID: 2, SubmittedAt: "", State: "CHANGES_REQUESTED"},
		}
		json.NewEncoder(w).Encode(reviews)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 123)

	require.NoError(t, err)
	require.Len(t, reviews, 3)
	// Should be sorted by ID as fallback
	assert.Equal(t, int64(1), reviews[0].ID)
	assert.Equal(t, int64(2), reviews[1].ID)
	assert.Equal(t, int64(3), reviews[2].ID)
}

func TestClient_ListReviews_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]github.ReviewSummary{})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	reviews, err := client.ListReviews(context.Background(), "owner", "repo", 1)

	require.NoError(t, err)
	assert.Empty(t, reviews)
}

func TestClient_ListReviews_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(github.GitHubErrorResponse{
			Message: "Not Found",
		})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.ListReviews(context.Background(), "nonexistent", "repo", 999)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestClient_DismissReview_Success(t *testing.T) {
	var receivedRequest github.DismissReviewRequest
	requestReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true

		// Verify request method and path
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/repos/owner/repo/pulls/123/reviews/456/dismissals", r.URL.Path)

		// Verify headers
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))

		// Parse request body
		json.NewDecoder(r.Body).Decode(&receivedRequest)

		// Send response
		resp := github.DismissReviewResponse{
			ID:    456,
			State: "DISMISSED",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	resp, err := client.DismissReview(context.Background(), "owner", "repo", 123, 456, "Superseded by new review")

	require.NoError(t, err)
	require.True(t, requestReceived)
	assert.Equal(t, int64(456), resp.ID)
	assert.Equal(t, "DISMISSED", resp.State)
	assert.Equal(t, "Superseded by new review", receivedRequest.Message)
}

func TestClient_DismissReview_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(github.GitHubErrorResponse{
			Message: "Not Found",
		})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.DismissReview(context.Background(), "owner", "repo", 123, 999, "message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestClient_DismissReview_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(github.GitHubErrorResponse{
			Message: "Resource not accessible by integration",
		})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	_, err := client.DismissReview(context.Background(), "owner", "repo", 123, 456, "message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}
