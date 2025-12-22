package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
