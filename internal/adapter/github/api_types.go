package github

// GitHub Pull Request Reviews API types.
// See: https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request

// ReviewEvent represents the action to take when submitting a review.
type ReviewEvent string

const (
	// EventComment submits the review without approval.
	EventComment ReviewEvent = "COMMENT"

	// EventApprove approves the pull request.
	EventApprove ReviewEvent = "APPROVE"

	// EventRequestChanges requests changes to the pull request.
	EventRequestChanges ReviewEvent = "REQUEST_CHANGES"
)

// CreateReviewRequest is the request body for POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews.
type CreateReviewRequest struct {
	// CommitID is the SHA of the commit to review (must be the head commit of the PR).
	CommitID string `json:"commit_id"`

	// Event is the review action: APPROVE, REQUEST_CHANGES, or COMMENT.
	Event ReviewEvent `json:"event"`

	// Body is the review summary comment.
	Body string `json:"body"`

	// Comments are the inline review comments at specific diff positions.
	Comments []ReviewComment `json:"comments,omitempty"`
}

// ReviewComment represents an inline comment at a specific diff position.
type ReviewComment struct {
	// Path is the relative path of the file to comment on.
	Path string `json:"path"`

	// Position is the line index in the diff to comment on (1-indexed from first @@).
	// Use side/line for multi-line diff format (not supported yet).
	Position int `json:"position"`

	// Body is the comment text (supports GitHub-flavored Markdown).
	Body string `json:"body"`
}

// CreateReviewResponse is the response from POST /repos/{owner}/{repo}/pulls/{pull_number}/reviews.
type CreateReviewResponse struct {
	ID          int64  `json:"id"`
	NodeID      string `json:"node_id"`
	User        User   `json:"user"`
	Body        string `json:"body"`
	State       string `json:"state"` // PENDING, APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	HTMLURL     string `json:"html_url"`
	SubmittedAt string `json:"submitted_at"`
}

// User represents a GitHub user in the response.
type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Type  string `json:"type"` // "User" or "Bot"
}

// GitHubErrorResponse represents an error response from the GitHub API.
type GitHubErrorResponse struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url"`
	Errors           []struct {
		Resource string `json:"resource"`
		Field    string `json:"field"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	} `json:"errors,omitempty"`
}
