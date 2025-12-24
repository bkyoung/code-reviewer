package tracking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
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

	// maxPaginationPages limits how many pages we'll fetch when searching for
	// the tracking comment. This prevents DoS on PRs with thousands of comments.
	maxPaginationPages = 10 // 10 pages * 100 per page = 1000 comments max

	// maxResponseSize limits how much data we'll read from a response body.
	// This prevents memory exhaustion from malicious or misconfigured servers.
	maxResponseSize = 10 * 1024 * 1024 // 10 MB
)

// pathSegmentRegex validates that owner/repo names only contain safe characters.
// GitHub allows alphanumeric, hyphens, underscores, and dots (but not leading dots).
var pathSegmentRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// pathTraversalPattern detects path traversal attempts.
var pathTraversalPattern = regexp.MustCompile(`\.\.`)

// DashboardRenderer defines the interface for rendering unified dashboard comments.
// This is provided by the github adapter package.
type DashboardRenderer interface {
	RenderDashboard(data review.DashboardData) (string, error)
}

// GitHubStore implements TrackingStore using GitHub PR comments.
// State is persisted as a special comment on the PR with embedded JSON metadata.
type GitHubStore struct {
	token             string
	baseURL           string
	httpClient        *http.Client
	retryConf         llmhttp.RetryConfig
	dashboardRenderer DashboardRenderer // Optional: enables unified dashboard mode
}

// NewGitHubStore creates a new GitHub-based tracking store.
func NewGitHubStore(token string) *GitHubStore {
	return &GitHubStore{
		token:   token,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			// Disable redirects to prevent SSRF attacks where a same-host
			// pagination URL could redirect to an internal endpoint.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
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

// SetDashboardRenderer configures the store to use unified dashboard comments.
// When set, SaveDashboard will use this renderer for rich presentation.
func (s *GitHubStore) SetDashboardRenderer(renderer DashboardRenderer) {
	s.dashboardRenderer = renderer
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

	// Update target fields to reflect current PR state.
	// The stored state captures historical information (Repository, PRNumber),
	// while the incoming target has current commit information.
	// We always update all three dynamic fields to maintain consistency:
	// - HeadSHA: always changes with new commits
	// - BaseSHA: may change if base branch is updated or rebased
	// - Branch: may change if PR is retargeted
	// Using empty string as "no change" would leave stale data, so we only
	// skip update if incoming value is empty AND stored value exists.
	if target.HeadSHA != "" {
		state.Target.HeadSHA = target.HeadSHA
	}
	if target.BaseSHA != "" {
		state.Target.BaseSHA = target.BaseSHA
	}
	if target.Branch != "" {
		state.Target.Branch = target.Branch
	}

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

// SaveDashboard persists a unified dashboard comment with full review content.
// If no DashboardRenderer is configured, it falls back to Save with basic rendering.
// Returns the URL of the dashboard comment for linking from the review body.
func (s *GitHubStore) SaveDashboard(ctx context.Context, data review.DashboardData) (string, error) {
	if err := data.Target.Validate(); err != nil {
		return "", fmt.Errorf("invalid target: %w", err)
	}

	if data.Target.PRNumber == 0 {
		return "", fmt.Errorf("GitHub tracking requires a PR number")
	}

	// Parse owner/repo
	owner, repo, err := parseRepository(data.Target.Repository)
	if err != nil {
		return "", err
	}

	// Render the comment body
	var body string
	if s.dashboardRenderer != nil {
		// Use the dashboard renderer for rich presentation
		body, err = s.dashboardRenderer.RenderDashboard(data)
		if err != nil {
			return "", fmt.Errorf("failed to render dashboard: %w", err)
		}
	} else {
		// Fallback to basic tracking comment rendering
		state := review.TrackingState{
			Target:          data.Target,
			ReviewedCommits: data.ReviewedCommits,
			Findings:        data.Findings,
			LastUpdated:     data.LastUpdated,
			ReviewStatus:    data.ReviewStatus,
		}
		body, err = RenderTrackingComment(state)
		if err != nil {
			return "", fmt.Errorf("failed to render tracking comment: %w", err)
		}
	}

	// Find existing comment (check for both dashboard and legacy tracking markers)
	existing, err := s.findTrackingOrDashboardComment(ctx, owner, repo, data.Target.PRNumber)
	if err != nil {
		return "", fmt.Errorf("failed to find existing comment: %w", err)
	}

	var commentURL string
	if existing != nil {
		// Update existing comment
		if err := s.updateComment(ctx, owner, repo, existing.ID, body); err != nil {
			return "", err
		}
		commentURL = existing.HTMLURL
	} else {
		// Create new comment
		response, err := s.createCommentWithResponse(ctx, owner, repo, data.Target.PRNumber, body)
		if err != nil {
			return "", err
		}
		commentURL = response.HTMLURL
	}

	return commentURL, nil
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
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"` // URL to view the comment on GitHub
}

// findTrackingComment searches for the tracking comment on a PR.
// Returns nil if no tracking comment exists.
// Implements pagination to handle PRs with more than 100 comments,
// with a page limit to prevent DoS on very active PRs.
func (s *GitHubStore) findTrackingComment(ctx context.Context, owner, repo string, prNumber int) (*issueComment, error) {
	// Validate inputs
	if err := validatePathSegment(owner, "owner"); err != nil {
		return nil, err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return nil, err
	}
	if prNumber <= 0 {
		return nil, fmt.Errorf("invalid PR number: %d", prNumber)
	}

	// GitHub treats PRs as issues for comments API
	// Use sort=updated&direction=desc so the bot's tracking comment (which gets
	// updated on each run) stays near the top, avoiding pagination limit issues
	// on PRs with many comments.
	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100&sort=updated&direction=desc",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), prNumber)

	// Paginate through comments until we find the tracking comment
	// Limit pages to prevent DoS on PRs with thousands of comments
	for page := 0; apiURL != "" && page < maxPaginationPages; page++ {
		respBody, nextURL, err := s.doRequestWithPagination(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		var comments []issueComment
		if err := json.Unmarshal(respBody, &comments); err != nil {
			return nil, fmt.Errorf("failed to parse comments: %w", err)
		}

		// Find the tracking comment (first match is most recent due to desc sort)
		for i := range comments {
			if IsTrackingComment(comments[i].Body) {
				return &comments[i], nil
			}
		}

		// Validate next URL before following (SSRF protection)
		if nextURL != "" && !s.isValidPaginationURL(nextURL) {
			return nil, fmt.Errorf("invalid pagination URL: host mismatch")
		}
		apiURL = nextURL
	}

	// If we still have pages to check but hit the limit, return an error
	// to signal that the search was incomplete
	if apiURL != "" {
		return nil, fmt.Errorf("pagination limit reached (%d pages), tracking comment may exist beyond searched range", maxPaginationPages)
	}

	return nil, nil
}

// isValidPaginationURL checks that a pagination URL is safe to follow.
// It must match the configured baseURL's host to prevent SSRF attacks.
func (s *GitHubStore) isValidPaginationURL(nextURL string) bool {
	next, err := url.Parse(nextURL)
	if err != nil {
		return false
	}

	base, err := url.Parse(s.baseURL)
	if err != nil {
		return false
	}

	// Require same scheme and host
	return next.Scheme == base.Scheme && next.Host == base.Host
}

// findTrackingOrDashboardComment searches for either a dashboard or tracking comment on a PR.
// This enables backward compatibility: dashboards can update legacy tracking comments.
// Returns nil if no matching comment exists.
func (s *GitHubStore) findTrackingOrDashboardComment(ctx context.Context, owner, repo string, prNumber int) (*issueComment, error) {
	// Validate inputs
	if err := validatePathSegment(owner, "owner"); err != nil {
		return nil, err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return nil, err
	}
	if prNumber <= 0 {
		return nil, fmt.Errorf("invalid PR number: %d", prNumber)
	}

	// GitHub treats PRs as issues for comments API
	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100&sort=updated&direction=desc",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), prNumber)

	// Paginate through comments until we find a matching comment
	for page := 0; apiURL != "" && page < maxPaginationPages; page++ {
		respBody, nextURL, err := s.doRequestWithPagination(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		var comments []issueComment
		if err := json.Unmarshal(respBody, &comments); err != nil {
			return nil, fmt.Errorf("failed to parse comments: %w", err)
		}

		// Check for both dashboard and legacy tracking markers
		for i := range comments {
			if isDashboardOrTrackingComment(comments[i].Body) {
				return &comments[i], nil
			}
		}

		// Validate next URL before following (SSRF protection)
		if nextURL != "" && !s.isValidPaginationURL(nextURL) {
			return nil, fmt.Errorf("invalid pagination URL: host mismatch")
		}
		apiURL = nextURL
	}

	// If we still have pages to check but hit the limit, return an error
	if apiURL != "" {
		return nil, fmt.Errorf("pagination limit reached (%d pages), comment may exist beyond searched range", maxPaginationPages)
	}

	return nil, nil
}

// isDashboardOrTrackingComment checks if a comment body contains either marker.
func isDashboardOrTrackingComment(body string) bool {
	// Check for both new dashboard marker and legacy tracking marker
	return strings.Contains(body, "<!-- CODE_REVIEWER_DASHBOARD_V1 -->") ||
		strings.Contains(body, trackingCommentMarker)
}

// createCommentWithResponse creates a new issue comment and returns the full response.
func (s *GitHubStore) createCommentWithResponse(ctx context.Context, owner, repo string, prNumber int, body string) (*issueComment, error) {
	if err := validatePathSegment(owner, "owner"); err != nil {
		return nil, err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return nil, err
	}
	if prNumber <= 0 {
		return nil, fmt.Errorf("invalid PR number: %d", prNumber)
	}

	apiURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments",
		s.baseURL, url.PathEscape(owner), url.PathEscape(repo), prNumber)

	reqBody := struct {
		Body string `json:"body"`
	}{Body: body}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	respBody, err := s.doRequest(ctx, "POST", apiURL, jsonData)
	if err != nil {
		return nil, err
	}

	var response issueComment
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// createComment creates a new issue comment.
func (s *GitHubStore) createComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	if err := validatePathSegment(owner, "owner"); err != nil {
		return err
	}
	if err := validatePathSegment(repo, "repo"); err != nil {
		return err
	}
	if prNumber <= 0 {
		return fmt.Errorf("invalid PR number: %d", prNumber)
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
	if commentID <= 0 {
		return fmt.Errorf("invalid comment ID: %d", commentID)
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
	if commentID <= 0 {
		return fmt.Errorf("invalid comment ID: %d", commentID)
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

// requestResult holds the result of an HTTP request attempt.
type requestResult struct {
	body       []byte
	statusCode int
	linkHeader string
}

// doRequestWithPagination executes an HTTP request and returns the next page URL if present.
func (s *GitHubStore) doRequestWithPagination(ctx context.Context, method, apiURL string, body []byte) (respBody []byte, nextURL string, err error) {
	var result *requestResult

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

		resp, callErr := s.httpClient.Do(req)
		if callErr != nil {
			errType, retryable := classifyTransportError(callErr)
			return &llmhttp.Error{
				Type:      errType,
				Message:   callErr.Error(),
				Retryable: retryable,
				Provider:  providerName,
			}
		}
		defer resp.Body.Close()

		// Limit response size to prevent memory exhaustion
		limitedBody := io.LimitReader(resp.Body, maxResponseSize)

		if resp.StatusCode >= 400 {
			bodyBytes, readErr := io.ReadAll(limitedBody)
			errMsg := string(bodyBytes)
			if readErr != nil {
				errMsg = fmt.Sprintf("(failed to read error response: %v)", readErr)
			}
			return mapHTTPError(resp.StatusCode, errMsg, resp.Header)
		}

		respBody, readErr := io.ReadAll(limitedBody)
		if readErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   fmt.Sprintf("failed to read response body: %v", readErr),
				Retryable: false,
				Provider:  providerName,
			}
		}

		result = &requestResult{
			body:       respBody,
			statusCode: resp.StatusCode,
			linkHeader: resp.Header.Get("Link"),
		}
		return nil
	}, s.retryConf)

	if err != nil {
		return nil, "", err
	}

	if result == nil {
		return nil, "", fmt.Errorf("no response after retries")
	}

	// Parse Link header for next page
	nextURL = parseNextPageURL(result.linkHeader)

	return result.body, nextURL, nil
}

// parseNextPageURL extracts the "next" URL from a GitHub Link header.
// Link header format: <url>; rel="next", <url>; rel="last"
func parseNextPageURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// Split by comma to get individual links
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		// Each link is: <url>; rel="type"
		parts := strings.Split(strings.TrimSpace(link), ";")
		if len(parts) < 2 {
			continue
		}

		// Check if this is the "next" link
		relPart := strings.TrimSpace(parts[1])
		if relPart == `rel="next"` {
			// Extract URL from <url>
			urlPart := strings.TrimSpace(parts[0])
			if strings.HasPrefix(urlPart, "<") && strings.HasSuffix(urlPart, ">") {
				return urlPart[1 : len(urlPart)-1]
			}
		}
	}

	return ""
}

// doRequest executes an HTTP request with retry logic and error handling.
// It returns the response body for successful requests, or an error.
// For requests without a body (like DELETE returning 204), respBody may be nil.
func (s *GitHubStore) doRequest(ctx context.Context, method, apiURL string, body []byte) ([]byte, error) {
	var result *requestResult

	err := llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
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

		resp, callErr := s.httpClient.Do(req)
		if callErr != nil {
			errType, retryable := classifyTransportError(callErr)
			return &llmhttp.Error{
				Type:      errType,
				Message:   callErr.Error(),
				Retryable: retryable,
				Provider:  providerName,
			}
		}
		// Always close response body within this attempt
		defer resp.Body.Close()

		// Limit response size to prevent memory exhaustion
		limitedBody := io.LimitReader(resp.Body, maxResponseSize)

		if resp.StatusCode >= 400 {
			bodyBytes, readErr := io.ReadAll(limitedBody)
			errMsg := string(bodyBytes)
			if readErr != nil {
				errMsg = fmt.Sprintf("(failed to read error response: %v)", readErr)
			}
			return mapHTTPError(resp.StatusCode, errMsg, resp.Header)
		}

		// Read successful response body
		var respBody []byte
		if resp.StatusCode == http.StatusNoContent {
			// Drain body to enable connection reuse even for 204 responses
			_, _ = io.Copy(io.Discard, limitedBody)
		} else {
			var readErr error
			respBody, readErr = io.ReadAll(limitedBody)
			if readErr != nil {
				return &llmhttp.Error{
					Type:      llmhttp.ErrTypeUnknown,
					Message:   fmt.Sprintf("failed to read response body: %v", readErr),
					Retryable: false,
					Provider:  providerName,
				}
			}
		}

		result = &requestResult{
			body:       respBody,
			statusCode: resp.StatusCode,
		}
		return nil
	}, s.retryConf)

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("no response after retries")
	}

	return result.body, nil
}

// parseRepository splits "owner/repo" into owner and repo.
// Rejects repositories with more than one slash (e.g., "owner/repo/extra").
func parseRepository(repository string) (owner, repo string, err error) {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository format: %q (expected exactly owner/repo)", repository)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository format: %q (owner and repo must not be empty)", repository)
	}
	return parts[0], parts[1], nil
}

// validatePathSegment validates that a path segment contains only safe characters.
// Uses whitelist validation to prevent path traversal and injection attacks.
func validatePathSegment(value, name string) error {
	if value == "" {
		return fmt.Errorf("invalid %s: must not be empty", name)
	}
	if pathTraversalPattern.MatchString(value) {
		return fmt.Errorf("invalid %s: must not contain '..'", name)
	}
	if !pathSegmentRegex.MatchString(value) {
		return fmt.Errorf("invalid %s: must contain only alphanumeric characters, hyphens, underscores, and dots (not leading)", name)
	}
	return nil
}

// mapHTTPError converts an HTTP error response to an llmhttp.Error.
// It also inspects headers to detect GitHub rate limiting on 403 responses.
func mapHTTPError(statusCode int, errMsg string, headers http.Header) error {
	// Only retry on server errors and rate limits
	// 4xx client errors (except 429) should not be retried
	retryable := statusCode >= 500 || statusCode == 429

	errType := llmhttp.ErrTypeUnknown

	// Check for rate limiting: 429 is explicit, but GitHub also uses 403
	// with X-RateLimit-Remaining: 0 or specific error messages
	isRateLimited := statusCode == 429
	if statusCode == 403 {
		// Check rate limit headers
		if headers.Get("X-RateLimit-Remaining") == "0" {
			isRateLimited = true
		}
		// Check for rate limit message in body
		if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "API rate limit exceeded") {
			isRateLimited = true
		}
	}

	if isRateLimited {
		errType = llmhttp.ErrTypeRateLimit
		retryable = true // Rate limits should be retried with backoff
	} else if statusCode == 401 || statusCode == 403 {
		errType = llmhttp.ErrTypeAuthentication
	}

	return &llmhttp.Error{
		Type:       errType,
		Message:    fmt.Sprintf("HTTP %d: %s", statusCode, errMsg),
		StatusCode: statusCode,
		Retryable:  retryable,
		Provider:   providerName,
	}
}

// classifyTransportError determines error type and retryability for transport errors.
func classifyTransportError(err error) (errType llmhttp.ErrorType, retryable bool) {
	// Check for context cancellation/deadline
	if errors.Is(err, context.DeadlineExceeded) {
		return llmhttp.ErrTypeTimeout, true
	}
	if errors.Is(err, context.Canceled) {
		return llmhttp.ErrTypeUnknown, false // Don't retry cancelled requests
	}

	// Check for network timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return llmhttp.ErrTypeTimeout, true
		}
		// Other network errors (DNS, connection refused, etc.) are retryable
		return llmhttp.ErrTypeUnknown, true
	}

	// Unknown transport errors - don't retry by default
	return llmhttp.ErrTypeUnknown, false
}

// Ensure GitHubStore implements TrackingStore.
var _ review.TrackingStore = (*GitHubStore)(nil)
