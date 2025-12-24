package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/adapter/github"
	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubStatusScanner_ScanForStatusUpdates(t *testing.T) {
	fingerprint := domain.FindingFingerprint("abc123def456")

	// Setup mock server that returns comments with replies
	comments := []github.PullRequestComment{
		{
			ID:   1,
			Body: "Finding<!-- CR_FINGERPRINT:" + string(fingerprint) + " -->",
			User: github.User{Login: "code-reviewer[bot]", Type: "Bot"},
		},
		{
			ID:          2,
			Body:        "acknowledged - this is intentional",
			User:        github.User{Login: "developer", Type: "User"},
			InReplyToID: 1,
			CreatedAt:   "2024-01-15T10:00:00Z",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	scanner := github.NewGitHubStatusScanner(client)

	updates, err := scanner.ScanForStatusUpdates(
		context.Background(),
		"owner", "repo", 42, "code-reviewer[bot]",
	)

	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, fingerprint, updates[0].Fingerprint)
	assert.Equal(t, domain.FindingStatusAcknowledged, updates[0].NewStatus)
	assert.Contains(t, updates[0].Reason, "acknowledged")
	assert.Equal(t, "developer", updates[0].UpdatedBy)
}

func TestGitHubStatusScanner_NoComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]github.PullRequestComment{})
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	scanner := github.NewGitHubStatusScanner(client)

	updates, err := scanner.ScanForStatusUpdates(
		context.Background(),
		"owner", "repo", 42, "bot",
	)

	require.NoError(t, err)
	assert.Empty(t, updates)
}

func TestGitHubStatusScanner_MultipleFindings(t *testing.T) {
	fp1 := domain.FindingFingerprint("fp1")
	fp2 := domain.FindingFingerprint("fp2")

	comments := []github.PullRequestComment{
		{ID: 1, Body: "<!-- CR_FINGERPRINT:" + string(fp1) + " -->", User: github.User{Login: "bot"}},
		{ID: 2, Body: "ack", User: github.User{Login: "dev"}, InReplyToID: 1, CreatedAt: "2024-01-15T10:00:00Z"},
		{ID: 3, Body: "<!-- CR_FINGERPRINT:" + string(fp2) + " -->", User: github.User{Login: "bot"}},
		{ID: 4, Body: "false positive", User: github.User{Login: "dev"}, InReplyToID: 3, CreatedAt: "2024-01-15T11:00:00Z"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	}))
	defer server.Close()

	client := github.NewClient("test-token")
	client.SetBaseURL(server.URL)

	scanner := github.NewGitHubStatusScanner(client)

	updates, err := scanner.ScanForStatusUpdates(
		context.Background(),
		"owner", "repo", 42, "bot",
	)

	require.NoError(t, err)
	require.Len(t, updates, 2)

	// Find updates by fingerprint
	var update1, update2 *struct {
		fp     domain.FindingFingerprint
		status domain.FindingStatus
	}
	for i := range updates {
		if updates[i].Fingerprint == fp1 {
			update1 = &struct {
				fp     domain.FindingFingerprint
				status domain.FindingStatus
			}{updates[i].Fingerprint, updates[i].NewStatus}
		}
		if updates[i].Fingerprint == fp2 {
			update2 = &struct {
				fp     domain.FindingFingerprint
				status domain.FindingStatus
			}{updates[i].Fingerprint, updates[i].NewStatus}
		}
	}

	require.NotNil(t, update1)
	assert.Equal(t, domain.FindingStatusAcknowledged, update1.status)

	require.NotNil(t, update2)
	assert.Equal(t, domain.FindingStatusDisputed, update2.status)
}
