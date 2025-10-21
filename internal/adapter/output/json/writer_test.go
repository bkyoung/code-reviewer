package json_test

import (
	"context"
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/brandon/code-reviewer/internal/adapter/output/json"
	"github.com/brandon/code-reviewer/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestWriter_Write(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	now := func() string { return "20251020T120000Z" }
	writer := json.NewWriter(now)

	review := domain.Review{
		ProviderName: "test-provider",
		ModelName:    "test-model",
		Summary:      "Test summary",
		Findings: []domain.Finding{
			{ID: "123", File: "main.go", LineStart: 1, LineEnd: 5, Description: "Test finding"},
		},
	}

	artifact := domain.JSONArtifact{
		OutputDir:    tempDir,
		Repository:   "test-repo",
		BaseRef:      "main",
		TargetRef:    "feature",
		Review:       review,
		ProviderName: "test-provider",
	}

	// When
	path, err := writer.Write(context.Background(), artifact)

	// Then
	assert.NoError(t, err)

	expectedPath := filepath.Join(tempDir, "test-repo_feature", "20251020T120000Z", "review-test-provider.json")
	assert.Equal(t, expectedPath, path)

	_, err = os.Stat(path)
	assert.NoError(t, err, "Expected file to be created")

	// Verify content
	content, err := os.ReadFile(path)
	assert.NoError(t, err)

	var writtenReview domain.Review
	err = stdjson.Unmarshal(content, &writtenReview)
	assert.NoError(t, err)
	assert.Equal(t, review, writtenReview)
}
