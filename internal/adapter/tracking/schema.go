package tracking

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
	"github.com/bkyoung/code-reviewer/internal/usecase/review"
)

// trackingCommentMarker is the HTML comment that identifies tracking comments.
// This must be unique enough to avoid false matches with user comments.
const trackingCommentMarker = "<!-- CODE_REVIEWER_TRACKING_V1 -->"

// trackingMetadataStart marks the beginning of the embedded base64-encoded JSON metadata.
// The payload is base64 encoded to avoid issues with HTML comment delimiters (-->) in JSON.
const trackingMetadataStart = "<!-- TRACKING_METADATA_B64"

// trackingMetadataEnd marks the end of the embedded metadata.
const trackingMetadataEnd = "-->"

// legacyMetadataStart is the old marker for backwards compatibility.
const legacyMetadataStart = "<!-- TRACKING_METADATA"

// maxMetadataSize limits the size of base64-encoded metadata to prevent DoS.
// GitHub comments are limited to ~65k chars, so 100KB is generous.
const maxMetadataSize = 100 * 1024

// trackingStateJSON is the JSON-serializable form of TrackingState.
type trackingStateJSON struct {
	Version         int                  `json:"version"`
	Repository      string               `json:"repository"`
	PRNumber        int                  `json:"pr_number"`
	Branch          string               `json:"branch"`
	BaseSHA         string               `json:"base_sha"`
	HeadSHA         string               `json:"head_sha"`
	ReviewedCommits []string             `json:"reviewed_commits"`
	Findings        []trackedFindingJSON `json:"findings"`
	LastUpdated     time.Time            `json:"last_updated"`
}

// trackedFindingJSON is the JSON-serializable form of TrackedFinding.
type trackedFindingJSON struct {
	Fingerprint string    `json:"fingerprint"`
	Status      string    `json:"status"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	SeenCount   int       `json:"seen_count"`

	// Finding fields (flattened for readability)
	FindingID   string `json:"finding_id"`
	File        string `json:"file"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	Evidence    bool   `json:"evidence"`
}

// IsTrackingComment returns true if the comment body contains the tracking marker.
func IsTrackingComment(body string) bool {
	return strings.Contains(body, trackingCommentMarker)
}

// ParseTrackingComment extracts TrackingState from a comment body.
// Returns an error if the comment doesn't contain valid tracking metadata.
func ParseTrackingComment(body string) (review.TrackingState, error) {
	// Extract JSON metadata from between markers
	jsonStr, err := extractMetadata(body)
	if err != nil {
		return review.TrackingState{}, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Parse JSON
	var stateJSON trackingStateJSON
	if err := json.Unmarshal([]byte(jsonStr), &stateJSON); err != nil {
		return review.TrackingState{}, fmt.Errorf("failed to parse tracking JSON: %w", err)
	}

	// Convert to domain types
	return jsonToState(stateJSON)
}

// RenderTrackingComment creates a comment body with embedded tracking state.
func RenderTrackingComment(state review.TrackingState) (string, error) {
	// Convert to JSON-serializable form
	stateJSON := stateToJSON(state)

	// Serialize to JSON
	jsonBytes, err := json.MarshalIndent(stateJSON, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize tracking state: %w", err)
	}

	// Build comment body
	var sb strings.Builder

	// Marker for identification
	sb.WriteString(trackingCommentMarker)
	sb.WriteString("\n\n")

	// Human-readable header
	sb.WriteString("## 🤖 Code Review Tracking\n\n")

	// Summary statistics
	activeCount := 0
	resolvedCount := 0
	acknowledgedCount := 0
	disputedCount := 0
	for _, f := range state.Findings {
		switch f.Status {
		case domain.FindingStatusOpen:
			activeCount++
		case domain.FindingStatusResolved:
			resolvedCount++
		case domain.FindingStatusAcknowledged:
			acknowledgedCount++
		case domain.FindingStatusDisputed:
			disputedCount++
		}
	}

	sb.WriteString("| Status | Count |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| 🔴 Open | %d |\n", activeCount))
	sb.WriteString(fmt.Sprintf("| ✅ Resolved | %d |\n", resolvedCount))
	sb.WriteString(fmt.Sprintf("| 💬 Acknowledged | %d |\n", acknowledgedCount))
	sb.WriteString(fmt.Sprintf("| ⚠️ Disputed | %d |\n", disputedCount))
	sb.WriteString("\n")

	// Reviewed commits
	if len(state.ReviewedCommits) > 0 {
		sb.WriteString("<details>\n")
		sb.WriteString("<summary>📋 Reviewed Commits</summary>\n\n")
		for _, sha := range state.ReviewedCommits {
			// Show short SHA
			shortSHA := sha
			if len(sha) > 7 {
				shortSHA = sha[:7]
			}
			sb.WriteString(fmt.Sprintf("- `%s`\n", shortSHA))
		}
		sb.WriteString("\n</details>\n\n")
	}

	// Last updated
	if !state.LastUpdated.IsZero() {
		sb.WriteString(fmt.Sprintf("*Last updated: %s*\n\n", state.LastUpdated.Format(time.RFC3339)))
	}

	// Embedded metadata (hidden from rendered view)
	// Base64 encode to avoid issues with --> in JSON content
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	sb.WriteString(trackingMetadataStart)
	sb.WriteString("\n")
	sb.WriteString(encoded)
	sb.WriteString("\n")
	sb.WriteString(trackingMetadataEnd)

	return sb.String(), nil
}

// extractMetadata extracts the JSON string from between metadata markers.
// Supports both new base64-encoded format and legacy raw JSON format.
func extractMetadata(body string) (string, error) {
	// Try new base64 format first
	startIdx := strings.Index(body, trackingMetadataStart)
	isBase64 := true

	// Fall back to legacy format if new format not found
	if startIdx == -1 {
		startIdx = strings.Index(body, legacyMetadataStart)
		isBase64 = false
	}

	if startIdx == -1 {
		return "", fmt.Errorf("tracking metadata start marker not found")
	}

	// Determine marker length based on format
	markerLen := len(trackingMetadataStart)
	if !isBase64 {
		markerLen = len(legacyMetadataStart)
	}

	// Skip past the start marker
	contentStart := startIdx + markerLen

	// Find end marker (after start)
	remaining := body[contentStart:]
	endIdx := strings.Index(remaining, trackingMetadataEnd)
	if endIdx == -1 {
		return "", fmt.Errorf("tracking metadata end marker not found")
	}

	// Extract and trim the content
	content := strings.TrimSpace(remaining[:endIdx])
	if content == "" {
		return "", fmt.Errorf("empty tracking metadata")
	}

	// Check size limit before decoding to prevent DoS
	if len(content) > maxMetadataSize {
		return "", fmt.Errorf("metadata too large: %d bytes (max %d)", len(content), maxMetadataSize)
	}

	// Decode if base64 encoded
	if isBase64 {
		// Use Strict decoding to reject malformed padding
		decoded, err := base64.StdEncoding.Strict().DecodeString(content)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 metadata: %w", err)
		}
		return string(decoded), nil
	}

	return content, nil
}

// stateToJSON converts a TrackingState to its JSON-serializable form.
// Findings are sorted by fingerprint to ensure deterministic output.
func stateToJSON(state review.TrackingState) trackingStateJSON {
	// Collect and sort fingerprints for deterministic ordering
	fingerprints := make([]string, 0, len(state.Findings))
	for fp := range state.Findings {
		fingerprints = append(fingerprints, string(fp))
	}
	sort.Strings(fingerprints)

	// Build findings slice in sorted order
	findings := make([]trackedFindingJSON, 0, len(state.Findings))
	for _, fpStr := range fingerprints {
		f := state.Findings[domain.FindingFingerprint(fpStr)]
		findings = append(findings, trackedFindingJSON{
			Fingerprint: string(f.Fingerprint),
			Status:      string(f.Status),
			FirstSeen:   f.FirstSeen,
			LastSeen:    f.LastSeen,
			SeenCount:   f.SeenCount,
			FindingID:   f.Finding.ID,
			File:        f.Finding.File,
			LineStart:   f.Finding.LineStart,
			LineEnd:     f.Finding.LineEnd,
			Severity:    f.Finding.Severity,
			Category:    f.Finding.Category,
			Description: f.Finding.Description,
			Suggestion:  f.Finding.Suggestion,
			Evidence:    f.Finding.Evidence,
		})
	}

	return trackingStateJSON{
		Version:         1,
		Repository:      state.Target.Repository,
		PRNumber:        state.Target.PRNumber,
		Branch:          state.Target.Branch,
		BaseSHA:         state.Target.BaseSHA,
		HeadSHA:         state.Target.HeadSHA,
		ReviewedCommits: state.ReviewedCommits,
		Findings:        findings,
		LastUpdated:     state.LastUpdated,
	}
}

// jsonToState converts JSON-serializable form back to TrackingState.
func jsonToState(stateJSON trackingStateJSON) (review.TrackingState, error) {
	target := review.ReviewTarget{
		Repository: stateJSON.Repository,
		PRNumber:   stateJSON.PRNumber,
		Branch:     stateJSON.Branch,
		BaseSHA:    stateJSON.BaseSHA,
		HeadSHA:    stateJSON.HeadSHA,
	}

	findings := make(map[domain.FindingFingerprint]domain.TrackedFinding, len(stateJSON.Findings))
	for _, fJSON := range stateJSON.Findings {
		// Skip findings with empty fingerprints to prevent map key collisions
		if fJSON.Fingerprint == "" {
			log.Printf("warning: skipping finding with empty fingerprint (file=%s, line=%d) - possible data corruption",
				fJSON.File, fJSON.LineStart)
			continue
		}

		// Reconstruct the Finding
		finding := domain.Finding{
			ID:          fJSON.FindingID,
			File:        fJSON.File,
			LineStart:   fJSON.LineStart,
			LineEnd:     fJSON.LineEnd,
			Severity:    fJSON.Severity,
			Category:    fJSON.Category,
			Description: fJSON.Description,
			Suggestion:  fJSON.Suggestion,
			Evidence:    fJSON.Evidence,
		}

		fingerprint := domain.FindingFingerprint(fJSON.Fingerprint)
		status := domain.FindingStatus(fJSON.Status)

		// Validate status
		if !status.IsValid() {
			log.Printf("warning: invalid status %q for finding %s, defaulting to 'open' - possible data corruption",
				fJSON.Status, fJSON.Fingerprint)
			status = domain.FindingStatusOpen
		}

		findings[fingerprint] = domain.TrackedFinding{
			Finding:     finding,
			Fingerprint: fingerprint,
			Status:      status,
			FirstSeen:   fJSON.FirstSeen,
			LastSeen:    fJSON.LastSeen,
			SeenCount:   fJSON.SeenCount,
		}
	}

	// Ensure ReviewedCommits is not nil
	reviewedCommits := stateJSON.ReviewedCommits
	if reviewedCommits == nil {
		reviewedCommits = []string{}
	}

	return review.TrackingState{
		Target:          target,
		ReviewedCommits: reviewedCommits,
		Findings:        findings,
		LastUpdated:     stateJSON.LastUpdated,
	}, nil
}
