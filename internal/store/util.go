package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GenerateRunID creates a unique, time-ordered run ID.
// Format: run-<timestamp>-<hash>
// Example: run-20251021T143052Z-a3f9c2
func GenerateRunID(timestamp time.Time, baseRef, targetRef string) string {
	// Use UTC timestamp in ISO format for consistent ordering
	ts := timestamp.UTC().Format("20060102T150405Z")

	// Create short hash from refs and nanoseconds for uniqueness
	input := fmt.Sprintf("%s|%s|%d", baseRef, targetRef, timestamp.UnixNano())
	hash := sha256.Sum256([]byte(input))
	shortHash := hex.EncodeToString(hash[:3]) // 6 character hash

	return fmt.Sprintf("run-%s-%s", ts, shortHash)
}

// GenerateFindingHash creates a deterministic hash for a finding.
// Findings with the same hash are considered duplicates.
// Description is normalized (lowercase, trimmed, whitespace collapsed) for better matching.
func GenerateFindingHash(file string, lineStart, lineEnd int, description string) string {
	// Normalize description: lowercase, trim, and collapse multiple spaces
	normalized := strings.ToLower(strings.TrimSpace(description))

	// Collapse multiple spaces to single space
	normalized = strings.Join(strings.Fields(normalized), " ")

	// Create hash input: file:lineStart-lineEnd:description
	input := fmt.Sprintf("%s:%d-%d:%s", file, lineStart, lineEnd, normalized)
	hash := sha256.Sum256([]byte(input))

	return hex.EncodeToString(hash[:])
}

// GenerateReviewID creates a unique ID for a review.
// Format: review-<run_id>-<provider>
func GenerateReviewID(runID, provider string) string {
	return fmt.Sprintf("review-%s-%s", runID, provider)
}

// GenerateFindingID creates a unique ID for a finding.
// Format: finding-<review_id>-<index>
// Index is zero-padded to 4 digits for proper sorting.
func GenerateFindingID(reviewID string, index int) string {
	return fmt.Sprintf("finding-%s-%04d", reviewID, index)
}

// CalculateConfigHash creates a deterministic hash of a configuration.
// This allows tracking which config was used for each run.
// The input should be JSON-serializable.
func CalculateConfigHash(config interface{}) (string, error) {
	// Serialize config to JSON (Go's JSON marshaling sorts map keys for determinism)
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Hash the serialized config
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
