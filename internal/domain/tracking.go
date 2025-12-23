package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// FindingStatus represents the lifecycle state of a tracked finding.
type FindingStatus string

const (
	// FindingStatusOpen indicates the finding needs attention.
	FindingStatusOpen FindingStatus = "open"
	// FindingStatusResolved indicates the finding was fixed.
	FindingStatusResolved FindingStatus = "resolved"
	// FindingStatusAcknowledged indicates the finding is valid but won't be fixed.
	FindingStatusAcknowledged FindingStatus = "acknowledged"
	// FindingStatusDisputed indicates the finding is contested.
	FindingStatusDisputed FindingStatus = "disputed"
)

// IsValid returns true if the status is a recognized value.
func (s FindingStatus) IsValid() bool {
	switch s {
	case FindingStatusOpen, FindingStatusResolved, FindingStatusAcknowledged, FindingStatusDisputed:
		return true
	default:
		return false
	}
}

// FindingFingerprint uniquely identifies a finding across reviews.
// It's stable across line number changes if the code intent remains the same.
type FindingFingerprint string

// NewFindingFingerprint creates a stable identifier for a finding.
// Uses file path + category + severity + normalized description prefix.
// Line numbers are intentionally excluded so the fingerprint remains stable
// when code shifts due to unrelated changes.
func NewFindingFingerprint(file, category, severity, description string) FindingFingerprint {
	// Use first 100 characters of description to allow minor wording changes.
	// Use rune slicing to avoid splitting multi-byte UTF-8 characters.
	descRunes := []rune(description)
	descPrefix := description
	if len(descRunes) > 100 {
		descPrefix = string(descRunes[:100])
	}

	payload := fmt.Sprintf("%s|%s|%s|%s", file, category, severity, descPrefix)
	sum := sha256.Sum256([]byte(payload))
	return FindingFingerprint(hex.EncodeToString(sum[:16])) // 32 hex chars
}

// FingerprintFromFinding creates a fingerprint from an existing Finding.
func FingerprintFromFinding(f Finding) FindingFingerprint {
	return NewFindingFingerprint(f.File, f.Category, f.Severity, f.Description)
}

// MaxStatusReasonLength is the maximum allowed length for a status reason.
const MaxStatusReasonLength = 500

// TrackedFinding wraps a Finding with tracking metadata.
type TrackedFinding struct {
	Finding     Finding
	Fingerprint FindingFingerprint
	Status      FindingStatus
	FirstSeen   time.Time
	LastSeen    time.Time
	SeenCount   int

	// StatusReason provides context for why the finding has its current status.
	// For example: "Intentional design choice" or "Fixed in refactor".
	StatusReason string

	// ReviewCommit is the commit SHA where this finding was first reviewed.
	// This field is immutable after creation.
	ReviewCommit string

	// ResolvedAt records when the finding was resolved.
	// This is cleared when the finding is reopened (transitions to open).
	ResolvedAt *time.Time

	// ResolvedIn is the commit SHA where the finding was fixed.
	// This is cleared when the finding is reopened (transitions to open).
	ResolvedIn *string
}

// TrackedFindingInput captures information needed to create a TrackedFinding.
type TrackedFindingInput struct {
	Finding   Finding
	Status    FindingStatus
	FirstSeen time.Time
	LastSeen  time.Time
	SeenCount int

	// Optional transition metadata
	StatusReason string
	ReviewCommit string
	ResolvedAt   *time.Time
	ResolvedIn   *string
}

// NewTrackedFinding constructs a TrackedFinding with validation.
func NewTrackedFinding(input TrackedFindingInput) (TrackedFinding, error) {
	if input.Finding.ID == "" {
		return TrackedFinding{}, fmt.Errorf("finding ID is required")
	}

	if !input.Status.IsValid() {
		return TrackedFinding{}, fmt.Errorf("invalid status: %s", input.Status)
	}

	if input.SeenCount < 1 {
		return TrackedFinding{}, fmt.Errorf("seen count must be >= 1, got %d", input.SeenCount)
	}

	if input.FirstSeen.IsZero() {
		return TrackedFinding{}, fmt.Errorf("first seen timestamp is required")
	}

	if input.LastSeen.IsZero() {
		return TrackedFinding{}, fmt.Errorf("last seen timestamp is required")
	}

	if input.LastSeen.Before(input.FirstSeen) {
		return TrackedFinding{}, fmt.Errorf("last seen (%v) cannot be before first seen (%v)",
			input.LastSeen, input.FirstSeen)
	}

	if len(input.StatusReason) > MaxStatusReasonLength {
		return TrackedFinding{}, fmt.Errorf("status reason exceeds %d characters: got %d",
			MaxStatusReasonLength, len(input.StatusReason))
	}

	// Validate consistency between status and resolution fields
	if input.Status == FindingStatusResolved && (input.ResolvedAt == nil || input.ResolvedAt.IsZero()) {
		return TrackedFinding{}, fmt.Errorf("resolved status requires valid ResolvedAt timestamp")
	}
	if input.Status != FindingStatusResolved && (input.ResolvedAt != nil || input.ResolvedIn != nil) {
		return TrackedFinding{}, fmt.Errorf("ResolvedAt/ResolvedIn should only be set when status is resolved")
	}

	fingerprint := FingerprintFromFinding(input.Finding)

	return TrackedFinding{
		Finding:      input.Finding,
		Fingerprint:  fingerprint,
		Status:       input.Status,
		FirstSeen:    input.FirstSeen,
		LastSeen:     input.LastSeen,
		SeenCount:    input.SeenCount,
		StatusReason: input.StatusReason,
		ReviewCommit: input.ReviewCommit,
		ResolvedAt:   input.ResolvedAt,
		ResolvedIn:   input.ResolvedIn,
	}, nil
}

// NewTrackedFindingFromFinding creates a new TrackedFinding in open status.
// This is a convenience constructor for first-time findings.
// The reviewCommit parameter records which commit this finding was first reviewed in.
func NewTrackedFindingFromFinding(f Finding, timestamp time.Time, reviewCommit string) (TrackedFinding, error) {
	return NewTrackedFinding(TrackedFindingInput{
		Finding:      f,
		Status:       FindingStatusOpen,
		FirstSeen:    timestamp,
		LastSeen:     timestamp,
		SeenCount:    1,
		ReviewCommit: reviewCommit,
	})
}

// MarkSeen updates the tracked finding when seen in a new review.
func (tf *TrackedFinding) MarkSeen(seenAt time.Time) {
	tf.LastSeen = seenAt
	tf.SeenCount++
}

// UpdateStatus changes the finding status with appropriate side effects.
//
// Transition behaviors:
//   - Any → open: Clears StatusReason, ResolvedAt, and ResolvedIn (reopening).
//     The reason, currentCommit, and timestamp parameters are ignored for this transition.
//   - Any → resolved: Sets ResolvedAt to timestamp, and ResolvedIn if currentCommit is provided
//   - Any → acknowledged/disputed: Updates status and reason, clears ResolvedAt and ResolvedIn
//
// The reason parameter provides context for the status change (max 500 chars).
// The currentCommit parameter is used when transitioning to resolved.
// The timestamp parameter is used for ResolvedAt when transitioning to resolved.
func (tf *TrackedFinding) UpdateStatus(status FindingStatus, reason string, currentCommit string, timestamp time.Time) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	// Handle transition to open (reopen) - clear resolution metadata
	// Note: reason, currentCommit, and timestamp are ignored for this transition
	if status == FindingStatusOpen {
		tf.Status = status
		tf.StatusReason = ""
		tf.ResolvedAt = nil
		tf.ResolvedIn = nil
		return nil
	}

	// Validate reason length only for statuses that use it
	if len(reason) > MaxStatusReasonLength {
		return fmt.Errorf("status reason exceeds %d characters: got %d",
			MaxStatusReasonLength, len(reason))
	}

	// Handle transition to resolved - set resolution metadata
	if status == FindingStatusResolved {
		tf.Status = status
		tf.StatusReason = reason
		tf.ResolvedAt = &timestamp
		if currentCommit != "" {
			tf.ResolvedIn = &currentCommit
		} else {
			tf.ResolvedIn = nil
		}
		return nil
	}

	// For other statuses (acknowledged, disputed), update status/reason and clear resolution fields
	tf.Status = status
	tf.StatusReason = reason
	tf.ResolvedAt = nil
	tf.ResolvedIn = nil
	return nil
}

// IsActive returns true if the finding requires attention.
func (tf TrackedFinding) IsActive() bool {
	return tf.Status == FindingStatusOpen
}

// IsResolved returns true if the finding has been resolved.
func (tf TrackedFinding) IsResolved() bool {
	return tf.Status == FindingStatusResolved
}
