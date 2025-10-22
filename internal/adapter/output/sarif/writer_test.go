package sarif_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/brandon/code-reviewer/internal/adapter/output/sarif"
	"github.com/brandon/code-reviewer/internal/domain"
	"github.com/brandon/code-reviewer/internal/usecase/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_Write(t *testing.T) {
	now := func() string { return "2025-10-20T12-00-00" }

	t.Run("writes SARIF file successfully", func(t *testing.T) {
		tmpDir := t.TempDir()

		writer := sarif.NewWriter(now)
		artifact := review.SARIFArtifact{
			OutputDir:    tmpDir,
			Repository:   "test-repo",
			BaseRef:      "main",
			TargetRef:    "feature",
			Review:       createTestReview(),
			ProviderName: "openai",
		}

		path, err := writer.Write(context.Background(), artifact)
		require.NoError(t, err)

		expectedPath := filepath.Join(tmpDir, "test-repo_feature", "2025-10-20T12-00-00", "review-openai.sarif")
		assert.Equal(t, expectedPath, path)

		// Verify file exists
		_, err = os.Stat(path)
		require.NoError(t, err)

		// Verify it's valid JSON
		content, err := os.ReadFile(path)
		require.NoError(t, err)

		var sarifDoc map[string]interface{}
		err = json.Unmarshal(content, &sarifDoc)
		require.NoError(t, err)

		// Verify SARIF structure
		assert.Equal(t, "2.1.0", sarifDoc["version"])
		assert.NotNil(t, sarifDoc["runs"])
	})

	t.Run("creates output directory if it doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "nested", "path")

		writer := sarif.NewWriter(now)
		artifact := review.SARIFArtifact{
			OutputDir:    outputDir,
			Repository:   "test-repo",
			BaseRef:      "main",
			TargetRef:    "feature",
			Review:       createTestReview(),
			ProviderName: "openai",
		}

		path, err := writer.Write(context.Background(), artifact)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(path)
		require.NoError(t, err)
	})

	t.Run("converts findings to SARIF results", func(t *testing.T) {
		tmpDir := t.TempDir()

		finding := domain.NewFinding(domain.FindingInput{
			File:        "main.go",
			LineStart:   10,
			LineEnd:     15,
			Severity:    "high",
			Category:    "security",
			Description: "SQL injection vulnerability",
			Suggestion:  "Use parameterized queries",
			Evidence:    true,
		})

		testReview := domain.Review{
			ProviderName: "openai",
			ModelName:    "gpt-4",
			Summary:      "Test review",
			Findings:     []domain.Finding{finding},
		}

		writer := sarif.NewWriter(now)
		artifact := review.SARIFArtifact{
			OutputDir:    tmpDir,
			Repository:   "test-repo",
			BaseRef:      "main",
			TargetRef:    "feature",
			Review:       testReview,
			ProviderName: "openai",
		}

		path, err := writer.Write(context.Background(), artifact)
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)

		var sarifDoc map[string]interface{}
		err = json.Unmarshal(content, &sarifDoc)
		require.NoError(t, err)

		// Verify results exist
		runs := sarifDoc["runs"].([]interface{})
		require.Len(t, runs, 1)

		run := runs[0].(map[string]interface{})
		results := run["results"].([]interface{})
		require.Len(t, results, 1)

		result := results[0].(map[string]interface{})
		assert.Equal(t, "SQL injection vulnerability", result["message"].(map[string]interface{})["text"])
	})
}

func createTestReview() domain.Review {
	finding := domain.NewFinding(domain.FindingInput{
		File:        "internal/test.go",
		LineStart:   1,
		LineEnd:     5,
		Severity:    "low",
		Category:    "style",
		Description: "Test finding",
		Suggestion:  "Fix it",
		Evidence:    true,
	})

	return domain.Review{
		ProviderName: "openai",
		ModelName:    "gpt-4",
		Summary:      "This is a test review.",
		Findings:     []domain.Finding{finding},
	}
}

func TestWriter_Write_IncludesCostInProperties(t *testing.T) {
	tmpDir := t.TempDir()
	now := func() string { return "2025-10-20T12-00-00" }

	testReview := domain.Review{
		ProviderName: "openai",
		ModelName:    "gpt-4o",
		Summary:      "Test review",
		Cost:         0.0523,
		Findings:     []domain.Finding{},
	}

	writer := sarif.NewWriter(now)
	artifact := review.SARIFArtifact{
		OutputDir:    tmpDir,
		Repository:   "test-repo",
		BaseRef:      "main",
		TargetRef:    "feature",
		Review:       testReview,
		ProviderName: "openai",
	}

	path, err := writer.Write(context.Background(), artifact)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var sarifDoc map[string]interface{}
	err = json.Unmarshal(content, &sarifDoc)
	require.NoError(t, err)

	// Verify cost is in run properties
	runs := sarifDoc["runs"].([]interface{})
	require.Len(t, runs, 1)

	run := runs[0].(map[string]interface{})
	properties := run["properties"].(map[string]interface{})
	assert.Equal(t, 0.0523, properties["cost"])
	assert.Equal(t, "Test review", properties["summary"])
}

func TestWriter_Write_HandlesInvalidCost(t *testing.T) {
	now := func() string { return "2025-10-20T12-00-00" }

	tests := []struct {
		name          string
		cost          float64
		shouldInclude bool
	}{
		{
			name:          "valid cost",
			cost:          1.23,
			shouldInclude: true,
		},
		{
			name:          "zero cost",
			cost:          0.0,
			shouldInclude: true,
		},
		{
			name:          "NaN cost",
			cost:          math.NaN(),
			shouldInclude: false,
		},
		{
			name:          "positive infinity",
			cost:          math.Inf(1),
			shouldInclude: false,
		},
		{
			name:          "negative infinity",
			cost:          math.Inf(-1),
			shouldInclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			testReview := domain.Review{
				ProviderName: "openai",
				ModelName:    "gpt-4o",
				Summary:      "Test review",
				Cost:         tt.cost,
				Findings:     []domain.Finding{},
			}

			writer := sarif.NewWriter(now)
			artifact := review.SARIFArtifact{
				OutputDir:    tmpDir,
				Repository:   "test-repo",
				BaseRef:      "main",
				TargetRef:    "feature",
				Review:       testReview,
				ProviderName: "openai",
			}

			path, err := writer.Write(context.Background(), artifact)
			require.NoError(t, err)

			content, err := os.ReadFile(path)
			require.NoError(t, err)

			var sarifDoc map[string]interface{}
			err = json.Unmarshal(content, &sarifDoc)
			require.NoError(t, err)

			// Verify cost handling in properties
			runs := sarifDoc["runs"].([]interface{})
			require.Len(t, runs, 1)

			run := runs[0].(map[string]interface{})
			properties := run["properties"].(map[string]interface{})

			if tt.shouldInclude {
				assert.Contains(t, properties, "cost", "valid cost should be included")
				assert.Equal(t, tt.cost, properties["cost"])
			} else {
				assert.NotContains(t, properties, "cost", "invalid cost should be excluded")
			}

			// Summary should always be included
			assert.Equal(t, "Test review", properties["summary"])
		})
	}
}
