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
	// Use first 100 chars of description to allow minor wording changes
	descPrefix := description
	if len(descPrefix) > 100 {
		descPrefix = descPrefix[:100]
	}

	payload := fmt.Sprintf("%s|%s|%s|%s", file, category, severity, descPrefix)
	sum := sha256.Sum256([]byte(payload))
	return FindingFingerprint(hex.EncodeToString(sum[:16])) // 32 hex chars
}

// FingerprintFromFinding creates a fingerprint from an existing Finding.
func FingerprintFromFinding(f Finding) FindingFingerprint {
	return NewFindingFingerprint(f.File, f.Category, f.Severity, f.Description)
}

// TrackedFinding wraps a Finding with tracking metadata.
type TrackedFinding struct {
	Finding     Finding
	Fingerprint FindingFingerprint
	Status      FindingStatus
	FirstSeen   time.Time
	LastSeen    time.Time
	SeenCount   int
}

// TrackedFindingInput captures information needed to create a TrackedFinding.
type TrackedFindingInput struct {
	Finding   Finding
	Status    FindingStatus
	FirstSeen time.Time
	LastSeen  time.Time
	SeenCount int
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

	if input.LastSeen.Before(input.FirstSeen) {
		return TrackedFinding{}, fmt.Errorf("last seen (%v) cannot be before first seen (%v)",
			input.LastSeen, input.FirstSeen)
	}

	fingerprint := FingerprintFromFinding(input.Finding)

	return TrackedFinding{
		Finding:     input.Finding,
		Fingerprint: fingerprint,
		Status:      input.Status,
		FirstSeen:   input.FirstSeen,
		LastSeen:    input.LastSeen,
		SeenCount:   input.SeenCount,
	}, nil
}

// NewTrackedFindingFromFinding creates a new TrackedFinding in open status.
// This is a convenience constructor for first-time findings.
func NewTrackedFindingFromFinding(f Finding, timestamp time.Time) (TrackedFinding, error) {
	return NewTrackedFinding(TrackedFindingInput{
		Finding:   f,
		Status:    FindingStatusOpen,
		FirstSeen: timestamp,
		LastSeen:  timestamp,
		SeenCount: 1,
	})
}

// MarkSeen updates the tracked finding when seen in a new review.
func (tf *TrackedFinding) MarkSeen(seenAt time.Time) {
	tf.LastSeen = seenAt
	tf.SeenCount++
}

// UpdateStatus changes the finding status.
func (tf *TrackedFinding) UpdateStatus(status FindingStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}
	tf.Status = status
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
