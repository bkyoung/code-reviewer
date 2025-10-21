package review_test

import (
	"context"
	"errors"
	"testing"

	"github.com/brandon/code-reviewer/internal/domain"
	"github.com/brandon/code-reviewer/internal/usecase/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore implements review.Store for testing
type mockStore struct {
	runs     []review.StoreRun
	reviews  []review.StoreReview
	findings []review.StoreFinding
	saveErr  error
}

func (m *mockStore) CreateRun(ctx context.Context, run review.StoreRun) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.runs = append(m.runs, run)
	return nil
}

func (m *mockStore) SaveReview(ctx context.Context, r review.StoreReview) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.reviews = append(m.reviews, r)
	return nil
}

func (m *mockStore) SaveFindings(ctx context.Context, findings []review.StoreFinding) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.findings = append(m.findings, findings...)
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func TestOrchestrator_SaveReviewToStore(t *testing.T) {
	t.Run("saves review and findings correctly", func(t *testing.T) {
		store := &mockStore{}
		orchestrator := createTestOrchestrator(store)

		domainReview := domain.Review{
			ProviderName: "openai",
			ModelName:    "gpt-4o-mini",
			Summary:      "Test summary",
			Findings: []domain.Finding{
				{
					File:        "main.go",
					LineStart:   10,
					LineEnd:     15,
					Category:    "security",
					Severity:    "high",
					Description: "SQL injection vulnerability",
					Suggestion:  "Use parameterized queries",
					Evidence:    true,
				},
			},
		}

		runID := "run-test-123"
		err := orchestrator.SaveReviewToStore(context.Background(), runID, domainReview)
		require.NoError(t, err)

		// Verify review was saved
		require.Len(t, store.reviews, 1)
		assert.Equal(t, "review-run-test-123-openai", store.reviews[0].ReviewID)
		assert.Equal(t, runID, store.reviews[0].RunID)
		assert.Equal(t, "openai", store.reviews[0].Provider)
		assert.Equal(t, "gpt-4o-mini", store.reviews[0].Model)
		assert.Equal(t, "Test summary", store.reviews[0].Summary)

		// Verify findings were saved
		require.Len(t, store.findings, 1)
		assert.Equal(t, "finding-review-run-test-123-openai-0000", store.findings[0].FindingID)
		assert.Equal(t, "review-run-test-123-openai", store.findings[0].ReviewID)
		assert.Equal(t, "main.go", store.findings[0].File)
		assert.Equal(t, 10, store.findings[0].LineStart)
		assert.Equal(t, 15, store.findings[0].LineEnd)
		assert.Equal(t, "security", store.findings[0].Category)
		assert.Equal(t, "high", store.findings[0].Severity)
		assert.NotEmpty(t, store.findings[0].FindingHash)
	})

	t.Run("handles empty findings list", func(t *testing.T) {
		store := &mockStore{}
		orchestrator := createTestOrchestrator(store)

		domainReview := domain.Review{
			ProviderName: "openai",
			ModelName:    "gpt-4o-mini",
			Summary:      "No issues found",
			Findings:     []domain.Finding{},
		}

		runID := "run-test-123"
		err := orchestrator.SaveReviewToStore(context.Background(), runID, domainReview)
		require.NoError(t, err)

		// Verify review was saved
		require.Len(t, store.reviews, 1)

		// Verify no findings were saved
		assert.Len(t, store.findings, 0)
	})

	t.Run("returns error on save failure", func(t *testing.T) {
		store := &mockStore{
			saveErr: errors.New("database error"),
		}
		orchestrator := createTestOrchestrator(store)

		domainReview := domain.Review{
			ProviderName: "openai",
			ModelName:    "gpt-4o-mini",
			Summary:      "Test",
			Findings:     []domain.Finding{},
		}

		runID := "run-test-123"
		err := orchestrator.SaveReviewToStore(context.Background(), runID, domainReview)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save review")
	})

	t.Run("works with nil store", func(t *testing.T) {
		orchestrator := createTestOrchestrator(nil)

		domainReview := domain.Review{
			ProviderName: "openai",
			ModelName:    "gpt-4o-mini",
			Summary:      "Test",
			Findings:     []domain.Finding{},
		}

		runID := "run-test-123"
		err := orchestrator.SaveReviewToStore(context.Background(), runID, domainReview)
		assert.NoError(t, err, "should not fail with nil store")
	})

	t.Run("generates correct finding hash", func(t *testing.T) {
		store := &mockStore{}
		orchestrator := createTestOrchestrator(store)

		domainReview := domain.Review{
			ProviderName: "openai",
			ModelName:    "gpt-4o-mini",
			Summary:      "Test",
			Findings: []domain.Finding{
				{
					File:        "main.go",
					LineStart:   10,
					LineEnd:     15,
					Description: "Test Finding",
				},
			},
		}

		runID := "run-test-123"
		err := orchestrator.SaveReviewToStore(context.Background(), runID, domainReview)
		require.NoError(t, err)

		// Finding hash should be consistent
		hash1 := store.findings[0].FindingHash

		// Save again with same finding
		store.findings = nil
		err = orchestrator.SaveReviewToStore(context.Background(), runID, domainReview)
		require.NoError(t, err)

		hash2 := store.findings[0].FindingHash
		assert.Equal(t, hash1, hash2, "finding hash should be deterministic")
	})
}

// Helper to create a test orchestrator with minimal deps
func createTestOrchestrator(store review.Store) *review.Orchestrator {
	return review.NewOrchestrator(review.OrchestratorDeps{
		Git:           &mockGitEngine{},
		Providers:     map[string]review.Provider{},
		Merger:        &mockMerger{},
		Markdown:      &mockMarkdownWriter{},
		JSON:          &mockJSONWriter{},
		SARIF:         &mockSARIFWriter{},
		SeedGenerator: func(baseRef, targetRef string) uint64 { return 12345 },
		PromptBuilder: func(diff domain.Diff, req review.BranchRequest) (review.ProviderRequest, error) {
			return review.ProviderRequest{}, nil
		},
		Store: store,
	})
}
