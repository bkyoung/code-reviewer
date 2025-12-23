package tracking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	llmhttp "github.com/bkyoung/code-reviewer/internal/adapter/llm/http"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

const (
	defaultBaseURL        = "https://api.github.com"
	defaultTimeout        = 30 * time.Second
	defaultMaxRetries     = 3
	defaultInitialBackoff = 2 * time.Second
	providerName          = "github-tracking"
)

// GitHubStore implements TrackingStore using GitHub PR comments.
// State is persisted as a special comment on the PR with embedded JSON metadata.
type GitHubStore struct {
	token      string
	baseURL    string
	httpClient *http.Client
	retryConf  llmhttp.RetryConfig
}

// NewGitHubStore creates a new GitHub-based tracking store.
func NewGitHubStore(token string) *GitHubStore {
	return &GitHubStore{
		token:      token,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		retryConf: llmhttp.RetryConfig{
			MaxRetries:     defaultMaxRetries,
			InitialBackoff: defaultInitialBackoff,
			MaxBackoff:     32 * time.Second,
			Multiplier:     2.0,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing or GitHub Enterprise).
func (s *GitHubStore) SetBaseURL(baseURL string) {
	s.baseURL = strings.TrimRight(baseURL, "/")
}

// Load retrieves the tracking state from the PR's tracking comment.
// Returns an empty state if no tracking comment exists.
func (s *GitHubStore) Load(ctx context.Context, target review.ReviewTarget) (review.TrackingState, error) {
	if err := target.Validate(); err != nil {
		return review.TrackingState{}, fmt.Errorf("invalid target: %w", err)
	}

	if target.PRNumber == 0 {
		return review.TrackingState{}, fmt.Errorf("GitHub tracking requires a PR number")
	}

	// Parse owner/repo
	owner, repo, err := parseRepository(target.Repository)
	if err != nil {
		return review.TrackingState{}, err
	}

	// Find the tracking comment
	comment, err := s.findTrackingComment(ctx, owner, repo, target.PRNumber)
	if err != nil {
		return review.TrackingState{}, fmt.Errorf("failed to find tracking comment: %w", err)
	}

	// No existing comment - return empty state
	if comment == nil {
		return review.NewTrackingState(target), nil
	}

	// Parse the comment body
	state, err := ParseTrackingComment(comment.Body)
	if err != nil {
		return review.TrackingState{}, fmt.Errorf("failed to parse tracking comment: %w", err)
	}

	// Update target with current values (in case they've changed)
	state.Target = target

	return state, nil
}

// Save persists the tracking state to the PR's tracking comment.
// Creates the comment if it doesn't exist, updates if it does.
func (s *GitHubStore) Save(ctx context.Context, state review.TrackingState) error {
	if err := state.Target.Validate(); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}

	if state.Target.PRNumber == 0 {
		return fmt.Errorf("GitHub tracking requires a PR number")
	}

	// Parse owner/repo
	owner, repo, err := parseRepository(state.Target.Repository)
	if err != nil {
		return err
	}

	// Render the comment body
	body, err := RenderTrackingComment(state)
	if err != nil {
		return fmt.Errorf("failed to render tracking comment: %w", err)
	}

	// Find existing comment
	existing, err := s.findTrackingComment(ctx, owner, repo, state.Target.PRNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing tracking comment: %w", err)
	}

	if existing != nil {
		// Update existing comment
		return s.updateComment(ctx, owner, repo, existing.ID, body)
	}

	// Create new comment
	return s.createComment(ctx, owner, repo, state.Target.PRNumber, body)
}

// Clear removes the tracking comment from the PR.
func (s *GitHubStore) Clear(ctx context.Context, target review.ReviewTarget) error {
	if err := target.Validate(); err != nil {
		return fmt.Errorf("invalid target: %w", err)
	}

	if target.PRNumber == 0 {
		// No PR number means nothing to clear
		return nil
	}

	// Parse owner/repo
	owner, repo, err := parseRepository(target.Repository)
	if err != nil {
		return err
	}

	// Find existing comment
	existing, err := s.findTrackingComment(ctx, owner, repo, target.PRNumber)
	if err != nil {
		return fmt.Errorf("failed to find tracking comment: %w", err)
	}

	if existing == nil {
		// No comment to delete
		return nil
	}

	// Delete the comment
	return s.deleteComment(ctx, owner, repo, existing.ID)
}

// issueComment represents a GitHub issue comment.
type issueComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

// findTrackingComment searches for the tracking comment on a PR.
// Returns nil if no tracking comment exists.
func (s *GitHubStore) findTrackingComment(ctx context.Context, owner, repo string, prNumber int) (*issueComment, error) {
	// Validate inputs
	if err := validatePathSegment(owner, "owner"); err != nil {
		return nil, err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return nil, err
	}

	// GitHub treats PRs as issues for comments API
	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), prNumber)

	respBody, err := s.doRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	var comments []issueComment
	if err := json.Unmarshal(respBody, &comments); err != nil {
		return nil, fmt.Errorf("failed to parse comments: %w", err)
	}

	// Find the tracking comment
	for _, c := range comments {
		if IsTrackingComment(c.Body) {
			return &c, nil
		}
	}

	return nil, nil
}

// createComment creates a new issue comment.
func (s *GitHubStore) createComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	if err := validatePathSegment(owner, "owner"); err != nil {
		return err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), prNumber)

	reqBody := struct {
		Body string `json:"body"`
	}{Body: body}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = s.doRequest(ctx, "POST", apiURL, jsonData)
	return err
}

// updateComment updates an existing issue comment.
func (s *GitHubStore) updateComment(ctx context.Context, owner, repo string, commentID int64, body string) error {
	if err := validatePathSegment(owner, "owner"); err != nil {
		return err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), commentID)

	reqBody := struct {
		Body string `json:"body"`
	}{Body: body}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = s.doRequest(ctx, "PATCH", apiURL, jsonData)
	return err
}

// deleteComment deletes an issue comment.
func (s *GitHubStore) deleteComment(ctx context.Context, owner, repo string, commentID int64) error {
	if err := validatePathSegment(owner, "owner"); err != nil {
		return err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), commentID)

	_, err := s.doRequest(ctx, "DELETE", apiURL, nil)
	return err
}

// setHeaders sets the common headers for GitHub API requests.
func (s *GitHubStore) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// doRequest executes an HTTP request with retry logic and error handling.
// It returns the response body for successful requests, or an error.
// For requests without a body (like DELETE returning 204), respBody may be nil.
func (s *GitHubStore) doRequest(ctx context.Context, method, apiURL string, body []byte) (respBody []byte, err error) {
	var resp *http.Response

	err = llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, reqErr := http.NewRequestWithContext(ctx, method, apiURL, bodyReader)
		if reqErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   reqErr.Error(),
				Retryable: false,
				Provider:  providerName,
			}
		}

		s.setHeaders(req)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		var callErr error
		resp, callErr = s.httpClient.Do(req)
		if callErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeTimeout,
				Message:   callErr.Error(),
				Retryable: true,
				Provider:  providerName,
			}
		}

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return mapHTTPError(resp.StatusCode, bodyBytes)
		}

		return nil
	}, s.retryConf)

	if err != nil {
		return nil, err
	}

	// Read response body if present
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
		// Only read body for non-204 responses
		if resp.StatusCode != http.StatusNoContent {
			respBody, err = io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}
		}
	}

	return respBody, nil
}

// parseRepository splits "owner/repo" into owner and repo.
func parseRepository(repository string) (owner, repo string, err error) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository format: %q (expected owner/repo)", repository)
	}
	return parts[0], parts[1], nil
}

// validatePathSegment validates that a path segment doesn't contain injection characters.
func validatePathSegment(value, name string) error {
	if strings.Contains(value, "..") {
		return fmt.Errorf("invalid %s: must not contain '..'", name)
	}
	if strings.Contains(value, "/") {
		return fmt.Errorf("invalid %s: must not contain '/'", name)
	}
	if value == "" {
		return fmt.Errorf("invalid %s: must not be empty", name)
	}
	return nil
}

// mapHTTPError converts an HTTP error response to an llmhttp.Error.
func mapHTTPError(statusCode int, body []byte) error {
	retryable := statusCode >= 500 || statusCode == 429

	return &llmhttp.Error{
		Type:       llmhttp.ErrTypeUnknown,
		Message:    fmt.Sprintf("HTTP %d: %s", statusCode, string(body)),
		StatusCode: statusCode,
		Retryable:  retryable,
		Provider:   providerName,
	}
}

// Ensure GitHubStore implements TrackingStore.
var _ review.TrackingStore = (*GitHubStore)(nil)
