package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	llmhttp "github.com/bkyoung/code-reviewer/internal/adapter/llm/http"
)

const (
	defaultBaseURL        = "https://api.github.com"
	defaultTimeout        = 30 * time.Second
	defaultMaxRetries     = 3
	defaultInitialBackoff = 2 * time.Second
)

// Client is an HTTP client for the GitHub Pull Request Reviews API.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
	retryConf  llmhttp.RetryConfig
}

// NewClient creates a new GitHub API client with the given token.
// The token should be a GitHub personal access token or GITHUB_TOKEN from Actions.
func NewClient(token string) *Client {
	return &Client{
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

// SetBaseURL sets a custom base URL (for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// SetTimeout sets the HTTP timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// SetMaxRetries sets the maximum number of retry attempts.
func (c *Client) SetMaxRetries(maxRetries int) {
	c.retryConf.MaxRetries = maxRetries
}

// SetInitialBackoff sets the initial backoff duration for retries.
func (c *Client) SetInitialBackoff(backoff time.Duration) {
	c.retryConf.InitialBackoff = backoff
}

// CreateReviewInput contains all data needed to create a PR review.
type CreateReviewInput struct {
	Owner      string
	Repo       string
	PullNumber int
	CommitSHA  string
	Event      ReviewEvent
	Summary    string
	Findings   []PositionedFinding
}

// CreateReview posts a pull request review with inline comments.
// Only findings with a valid DiffPosition (InDiff() == true) are posted as inline comments.
// Returns an error if the request fails after all retries.
func (c *Client) CreateReview(ctx context.Context, input CreateReviewInput) (*CreateReviewResponse, error) {
	// Build the API request
	comments := BuildReviewComments(input.Findings)

	reqBody := CreateReviewRequest{
		CommitID: input.CommitSHA,
		Event:    input.Event,
		Body:     input.Summary,
		Comments: comments,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews",
		c.baseURL, input.Owner, input.Repo, input.PullNumber)

	// Execute with retry
	var resp *http.Response
	err = llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
		req, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if reqErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   reqErr.Error(),
				Retryable: false,
				Provider:  providerName,
			}
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		var callErr error
		resp, callErr = c.httpClient.Do(req)
		if callErr != nil {
			// Could be timeout or network error
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeTimeout,
				Message:   callErr.Error(),
				Retryable: true,
				Provider:  providerName,
			}
		}

		// Check for error status codes
		if resp.StatusCode >= 400 {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				// If we can't read the error body, return a generic error with the status code
				return &llmhttp.Error{
					Type:       llmhttp.ErrTypeUnknown,
					Message:    fmt.Sprintf("HTTP %d (failed to read response: %v)", resp.StatusCode, readErr),
					StatusCode: resp.StatusCode,
					Retryable:  resp.StatusCode >= 500,
					Provider:   providerName,
				}
			}
			return MapHTTPError(resp.StatusCode, bodyBytes)
		}

		return nil
	}, c.retryConf)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response
	var reviewResp CreateReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&reviewResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &reviewResp, nil
}

// ListReviews fetches all reviews for a pull request.
// Returns reviews in chronological order (oldest first).
// Handles GitHub API pagination to ensure all reviews are returned.
func (c *Client) ListReviews(ctx context.Context, owner, repo string, pullNumber int) ([]ReviewSummary, error) {
	var allReviews []ReviewSummary

	// Start with the first page, using max per_page to minimize API calls
	nextURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews?per_page=100",
		c.baseURL, owner, repo, pullNumber)

	for nextURL != "" {
		pageReviews, next, err := c.fetchReviewsPage(ctx, nextURL)
		if err != nil {
			return nil, err
		}
		allReviews = append(allReviews, pageReviews...)

		// Validate pagination URL to prevent SSRF attacks
		// Only follow URLs that match our configured base URL host
		if next != "" && !c.isTrustedURL(next) {
			// Stop pagination if Link header points to untrusted host
			break
		}
		nextURL = next
	}

	return allReviews, nil
}

// isTrustedURL validates that a URL's host matches the configured baseURL.
// This prevents SSRF attacks via malicious Link header manipulation.
func (c *Client) isTrustedURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	base, err := url.Parse(c.baseURL)
	if err != nil {
		return false
	}

	// Require matching host (includes port if present)
	return parsed.Host == base.Host
}

// fetchReviewsPage fetches a single page of reviews and returns the next page URL if present.
func (c *Client) fetchReviewsPage(ctx context.Context, url string) ([]ReviewSummary, string, error) {
	var resp *http.Response
	err := llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
		req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
		if reqErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   reqErr.Error(),
				Retryable: false,
				Provider:  providerName,
			}
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		var callErr error
		resp, callErr = c.httpClient.Do(req)
		if callErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeTimeout,
				Message:   callErr.Error(),
				Retryable: true,
				Provider:  providerName,
			}
		}

		if resp.StatusCode >= 400 {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return &llmhttp.Error{
					Type:       llmhttp.ErrTypeUnknown,
					Message:    fmt.Sprintf("HTTP %d (failed to read response: %v)", resp.StatusCode, readErr),
					StatusCode: resp.StatusCode,
					Retryable:  resp.StatusCode >= 500,
					Provider:   providerName,
				}
			}
			return MapHTTPError(resp.StatusCode, bodyBytes)
		}

		return nil
	}, c.retryConf)

	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var reviews []ReviewSummary
	if err := json.NewDecoder(resp.Body).Decode(&reviews); err != nil {
		return nil, "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse Link header for pagination
	nextURL := parseNextLink(resp.Header.Get("Link"))

	return reviews, nextURL, nil
}

// parseNextLink extracts the "next" URL from the GitHub Link header.
// Format: <url>; rel="next", <url>; rel="last"
func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// Match pattern: <URL>; rel="next"
	re := regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)
	matches := re.FindStringSubmatch(linkHeader)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// DismissReview dismisses a pull request review with the given message.
// Returns an error if the request fails after all retries.
func (c *Client) DismissReview(ctx context.Context, owner, repo string, pullNumber int, reviewID int64, message string) (*DismissReviewResponse, error) {
	reqBody := DismissReviewRequest{Message: message}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews/%d/dismissals",
		c.baseURL, owner, repo, pullNumber, reviewID)

	var resp *http.Response
	err = llmhttp.RetryWithBackoff(ctx, func(ctx context.Context) error {
		req, reqErr := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonData))
		if reqErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeUnknown,
				Message:   reqErr.Error(),
				Retryable: false,
				Provider:  providerName,
			}
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		var callErr error
		resp, callErr = c.httpClient.Do(req)
		if callErr != nil {
			return &llmhttp.Error{
				Type:      llmhttp.ErrTypeTimeout,
				Message:   callErr.Error(),
				Retryable: true,
				Provider:  providerName,
			}
		}

		if resp.StatusCode >= 400 {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return &llmhttp.Error{
					Type:       llmhttp.ErrTypeUnknown,
					Message:    fmt.Sprintf("HTTP %d (failed to read response: %v)", resp.StatusCode, readErr),
					StatusCode: resp.StatusCode,
					Retryable:  resp.StatusCode >= 500,
					Provider:   providerName,
				}
			}
			return MapHTTPError(resp.StatusCode, bodyBytes)
		}

		return nil
	}, c.retryConf)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var dismissResp DismissReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&dismissResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &dismissResp, nil
}
