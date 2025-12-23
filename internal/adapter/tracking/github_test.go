package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

func TestGitHubStore_Load_NoExistingComment(t *testing.T) {
	// Mock server returns empty comments list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/repos/owner/repo/issues/123/comments") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]issueComment{})
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	state, err := store.Load(context.Background(), target)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should return empty state
	if len(state.Findings) != 0 {
		t.Errorf("expected empty findings, got %d", len(state.Findings))
	}
	if state.Target.Repository != target.Repository {
		t.Errorf("target repository mismatch")
	}
}

func TestGitHubStore_Load_ExistingComment(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	// Create a tracking comment body
	finding := domain.NewFinding(domain.FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test issue",
		Suggestion:  "",
		Evidence:    false,
	})
	trackedFinding, _ := domain.NewTrackedFindingFromFinding(finding, now)

	existingState := review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   123,
			HeadSHA:    "abc123",
		},
		ReviewedCommits: []string{"abc123"},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{
			trackedFinding.Fingerprint: trackedFinding,
		},
		LastUpdated: now,
	}

	commentBody, _ := RenderTrackingComment(existingState)

	// Mock server returns comment with tracking data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]issueComment{
			{ID: 1, Body: "Regular comment"},
			{ID: 2, Body: commentBody},
		})
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "def456", // Different head SHA
	}

	state, err := store.Load(context.Background(), target)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have the finding
	if len(state.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(state.Findings))
	}

	// Target should be updated to current
	if state.Target.HeadSHA != "def456" {
		t.Errorf("HeadSHA should be updated to current, got %s", state.Target.HeadSHA)
	}
}

func TestGitHubStore_Save_CreateNew(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		if r.Method == "GET" {
			// Return empty comments list (no existing tracking comment)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]issueComment{})
			return
		}

		if r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(issueComment{ID: 1})
			return
		}
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	state := review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   123,
			HeadSHA:    "abc123",
		},
		ReviewedCommits: []string{"abc123"},
		Findings:        map[domain.FindingFingerprint]domain.TrackedFinding{},
		LastUpdated:     time.Now(),
	}

	err := store.Save(context.Background(), state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Last call should be POST to create
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if !strings.Contains(receivedPath, "/issues/123/comments") {
		t.Errorf("unexpected path: %s", receivedPath)
	}

	// Body should contain tracking marker
	body, ok := receivedBody["body"].(string)
	if !ok {
		t.Error("body should be a string")
	}
	if !strings.Contains(body, trackingCommentMarker) {
		t.Error("body should contain tracking marker")
	}
}

func TestGitHubStore_Save_UpdateExisting(t *testing.T) {
	existingCommentID := int64(42)
	var lastMethod string
	var lastPath string

	// Create an existing tracking comment
	existingBody, _ := RenderTrackingComment(review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   123,
			HeadSHA:    "old-sha",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]issueComment{
				{ID: existingCommentID, Body: existingBody},
			})
			return
		}

		if r.Method == "PATCH" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(issueComment{ID: existingCommentID})
			return
		}
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	state := review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   123,
			HeadSHA:    "new-sha",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{},
	}

	err := store.Save(context.Background(), state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Should use PATCH to update
	if lastMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", lastMethod)
	}
	if !strings.Contains(lastPath, "/issues/comments/42") {
		t.Errorf("unexpected path: %s", lastPath)
	}
}

func TestGitHubStore_Clear_ExistingComment(t *testing.T) {
	existingCommentID := int64(42)
	var deleteCalled bool
	var deletePath string

	existingBody, _ := RenderTrackingComment(review.TrackingState{
		Target: review.ReviewTarget{
			Repository: "owner/repo",
			PRNumber:   123,
			HeadSHA:    "abc123",
		},
		Findings: map[domain.FindingFingerprint]domain.TrackedFinding{},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]issueComment{
				{ID: existingCommentID, Body: existingBody},
			})
			return
		}

		if r.Method == "DELETE" {
			deleteCalled = true
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	err := store.Clear(context.Background(), target)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if !deleteCalled {
		t.Error("DELETE should have been called")
	}
	if !strings.Contains(deletePath, "/issues/comments/42") {
		t.Errorf("unexpected delete path: %s", deletePath)
	}
}

func TestGitHubStore_Clear_NoExistingComment(t *testing.T) {
	var deleteCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]issueComment{})
			return
		}

		if r.Method == "DELETE" {
			deleteCalled = true
			return
		}
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	err := store.Clear(context.Background(), target)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if deleteCalled {
		t.Error("DELETE should not be called when no comment exists")
	}
}

func TestGitHubStore_Load_InvalidRepository(t *testing.T) {
	store := NewGitHubStore("test-token")

	target := review.ReviewTarget{
		Repository: "invalid-no-slash",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	_, err := store.Load(context.Background(), target)
	if err == nil {
		t.Error("expected error for invalid repository format")
	}
}

func TestGitHubStore_Load_NoPRNumber(t *testing.T) {
	store := NewGitHubStore("test-token")

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   0, // No PR number
		HeadSHA:    "abc123",
	}

	_, err := store.Load(context.Background(), target)
	if err == nil {
		t.Error("expected error when PR number is 0")
	}
}

func TestGitHubStore_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "Resource not accessible"}`))
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)
	store.retryConf.MaxRetries = 0 // Disable retries for test speed

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	_, err := store.Load(context.Background(), target)
	if err == nil {
		t.Error("expected error for HTTP 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestParseRepository(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"my-org/my-repo", "my-org", "my-repo", false},
		{"owner/repo/extra", "", "", true}, // Now rejects extra slashes
		{"noslash", "", "", true},
		{"", "", "", true},
		{"/repo", "", "", true},  // Empty owner
		{"owner/", "", "", true}, // Empty repo
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, err := parseRepository(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %s, want %s", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %s, want %s", repo, tt.wantRepo)
			}
		})
	}
}

func TestValidatePathSegment(t *testing.T) {
	tests := []struct {
		value   string
		name    string
		wantErr bool
	}{
		{"valid", "test", false},
		{"valid-with-dash", "test", false},
		{"valid_underscore", "test", false},
		{"valid.dot", "test", false},
		{"valid123", "test", false},
		{"", "test", true},
		{"has/slash", "test", true},
		{"has..dots", "test", true},
		{"..", "test", true},
		{".leading-dot", "test", true},
		{"-leading-dash", "test", true},
		{"has spaces", "test", true},
		{"has@symbol", "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validatePathSegment(tt.value, tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePathSegment(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestParseNextPageURL(t *testing.T) {
	tests := []struct {
		name       string
		linkHeader string
		want       string
	}{
		{
			name:       "empty header",
			linkHeader: "",
			want:       "",
		},
		{
			name:       "next link only",
			linkHeader: `<https://api.github.com/repos/owner/repo/issues/1/comments?page=2>; rel="next"`,
			want:       "https://api.github.com/repos/owner/repo/issues/1/comments?page=2",
		},
		{
			name:       "next and last links",
			linkHeader: `<https://api.github.com/repos/owner/repo/issues/1/comments?page=2>; rel="next", <https://api.github.com/repos/owner/repo/issues/1/comments?page=5>; rel="last"`,
			want:       "https://api.github.com/repos/owner/repo/issues/1/comments?page=2",
		},
		{
			name:       "first prev next last",
			linkHeader: `<https://api.github.com/repos/owner/repo/issues/1/comments?page=1>; rel="first", <https://api.github.com/repos/owner/repo/issues/1/comments?page=2>; rel="prev", <https://api.github.com/repos/owner/repo/issues/1/comments?page=4>; rel="next", <https://api.github.com/repos/owner/repo/issues/1/comments?page=5>; rel="last"`,
			want:       "https://api.github.com/repos/owner/repo/issues/1/comments?page=4",
		},
		{
			name:       "last page - no next",
			linkHeader: `<https://api.github.com/repos/owner/repo/issues/1/comments?page=1>; rel="first", <https://api.github.com/repos/owner/repo/issues/1/comments?page=4>; rel="prev"`,
			want:       "",
		},
		{
			name:       "malformed - no rel",
			linkHeader: `<https://api.github.com/repos/owner/repo/issues/1/comments?page=2>`,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNextPageURL(tt.linkHeader)
			if got != tt.want {
				t.Errorf("parseNextPageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitHubStore_Load_Pagination(t *testing.T) {
	// Track which page we're on
	pageCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++

		w.Header().Set("Content-Type", "application/json")

		if pageCount == 1 {
			// First page: no tracking comment, has next link
			nextURL := "http://" + r.Host + "/repos/owner/repo/issues/123/comments?page=2"
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
			json.NewEncoder(w).Encode([]issueComment{
				{ID: 1, Body: "Regular comment 1"},
				{ID: 2, Body: "Regular comment 2"},
			})
		} else if pageCount == 2 {
			// Second page: has tracking comment
			trackingBody, _ := RenderTrackingComment(review.TrackingState{
				Target: review.ReviewTarget{
					Repository: "owner/repo",
					PRNumber:   123,
					HeadSHA:    "abc123",
				},
				Findings: map[domain.FindingFingerprint]domain.TrackedFinding{},
			})
			json.NewEncoder(w).Encode([]issueComment{
				{ID: 3, Body: "Regular comment 3"},
				{ID: 4, Body: trackingBody},
			})
		}
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "def456",
	}

	state, err := store.Load(context.Background(), target)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have paginated to find the comment
	if pageCount != 2 {
		t.Errorf("expected 2 pages to be fetched, got %d", pageCount)
	}

	// Should have loaded the state
	if state.Target.Repository != "owner/repo" {
		t.Errorf("Repository = %s, want owner/repo", state.Target.Repository)
	}
}

func TestGitHubStore_Load_PaginationSSRFProtection(t *testing.T) {
	// Server that returns a Link header pointing to a different host
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Malicious next URL pointing to different host
		w.Header().Set("Link", `<https://evil.com/steal-data>; rel="next"`)
		json.NewEncoder(w).Encode([]issueComment{
			{ID: 1, Body: "No tracking comment here"},
		})
	}))
	defer server.Close()

	store := NewGitHubStore("test-token")
	store.SetBaseURL(server.URL)

	target := review.ReviewTarget{
		Repository: "owner/repo",
		PRNumber:   123,
		HeadSHA:    "abc123",
	}

	_, err := store.Load(context.Background(), target)
	if err == nil {
		t.Error("expected error for SSRF attempt, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "host mismatch") {
		t.Errorf("expected 'host mismatch' error, got: %v", err)
	}
}

func TestIsValidPaginationURL(t *testing.T) {
	store := NewGitHubStore("test-token")
	store.SetBaseURL("https://api.github.com")

	tests := []struct {
		name    string
		nextURL string
		want    bool
	}{
		{
			name:    "valid same host",
			nextURL: "https://api.github.com/repos/owner/repo/issues/1/comments?page=2",
			want:    true,
		},
		{
			name:    "different host",
			nextURL: "https://evil.com/steal",
			want:    false,
		},
		{
			name:    "different scheme",
			nextURL: "http://api.github.com/repos/owner/repo/issues/1/comments?page=2",
			want:    false,
		},
		{
			name:    "invalid URL",
			nextURL: "://invalid",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.isValidPaginationURL(tt.nextURL)
			if got != tt.want {
				t.Errorf("isValidPaginationURL(%q) = %v, want %v", tt.nextURL, got, tt.want)
			}
		})
	}
}
