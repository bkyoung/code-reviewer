package domain

import (
	"testing"
	"time"
)

func TestFindingStatus_IsValid(t *testing.T) {
	tests := []struct {
		status FindingStatus
		want   bool
	}{
		{FindingStatusOpen, true},
		{FindingStatusResolved, true},
		{FindingStatusAcknowledged, true},
		{FindingStatusDisputed, true},
		{FindingStatus("invalid"), false},
		{FindingStatus(""), false},
		{FindingStatus("OPEN"), false}, // Case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("FindingStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestNewFindingFingerprint_Deterministic(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")
	fp2 := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")

	if fp1 != fp2 {
		t.Errorf("fingerprints should be deterministic: %s != %s", fp1, fp2)
	}
}

func TestNewFindingFingerprint_UniqueAcrossFiles(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")
	fp2 := NewFindingFingerprint("db.go", "security", "high", "SQL injection risk")

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different files")
	}
}

func TestNewFindingFingerprint_UniqueAcrossCategories(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "Issue description")
	fp2 := NewFindingFingerprint("main.go", "performance", "high", "Issue description")

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different categories")
	}
}

func TestNewFindingFingerprint_UniqueAcrossSeverities(t *testing.T) {
	fp1 := NewFindingFingerprint("main.go", "security", "high", "Issue description")
	fp2 := NewFindingFingerprint("main.go", "security", "low", "Issue description")

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different severities")
	}
}

func TestNewFindingFingerprint_TruncatesLongDescriptions(t *testing.T) {
	longDesc := make([]byte, 200)
	for i := range longDesc {
		longDesc[i] = 'a'
	}

	// Two descriptions that differ only after char 100 should have same fingerprint
	desc1 := string(longDesc)
	longDesc[150] = 'b'
	desc2 := string(longDesc)

	fp1 := NewFindingFingerprint("main.go", "security", "high", desc1)
	fp2 := NewFindingFingerprint("main.go", "security", "high", desc2)

	if fp1 != fp2 {
		t.Error("fingerprints should match when descriptions only differ after 100 chars")
	}
}

func TestFingerprintFromFinding(t *testing.T) {
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     15,
		Severity:    "high",
		Category:    "security",
		Description: "SQL injection risk",
		Suggestion:  "Use parameterized queries",
		Evidence:    true,
	})

	fp := FingerprintFromFinding(finding)
	expected := NewFindingFingerprint("main.go", "security", "high", "SQL injection risk")

	if fp != expected {
		t.Errorf("FingerprintFromFinding() = %s, want %s", fp, expected)
	}
}

func TestNewTrackedFinding_Valid(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "SQL injection risk",
		Suggestion:  "Use parameterized queries",
		Evidence:    true,
	})

	tf, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 1,
	})

	if err != nil {
		t.Fatalf("NewTrackedFinding() error = %v", err)
	}

	if tf.Status != FindingStatusOpen {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusOpen)
	}

	if tf.SeenCount != 1 {
		t.Errorf("SeenCount = %d, want 1", tf.SeenCount)
	}

	if tf.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}
}

func TestNewTrackedFinding_EmptyFindingID(t *testing.T) {
	now := time.Now()
	finding := Finding{} // Empty ID

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 1,
	})

	if err == nil {
		t.Error("expected error for empty finding ID")
	}
}

func TestNewTrackedFinding_InvalidStatus(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatus("invalid"),
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 1,
	})

	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestNewTrackedFinding_ZeroSeenCount(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now,
		SeenCount: 0,
	})

	if err == nil {
		t.Error("expected error for zero seen count")
	}
}

func TestNewTrackedFinding_LastSeenBeforeFirstSeen(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	_, err := NewTrackedFinding(TrackedFindingInput{
		Finding:   finding,
		Status:    FindingStatusOpen,
		FirstSeen: now,
		LastSeen:  now.Add(-time.Hour), // Before first seen
		SeenCount: 1,
	})

	if err == nil {
		t.Error("expected error when LastSeen is before FirstSeen")
	}
}

func TestNewTrackedFindingFromFinding(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, err := NewTrackedFindingFromFinding(finding, now)
	if err != nil {
		t.Fatalf("NewTrackedFindingFromFinding() error = %v", err)
	}

	if tf.Status != FindingStatusOpen {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusOpen)
	}

	if tf.SeenCount != 1 {
		t.Errorf("SeenCount = %d, want 1", tf.SeenCount)
	}
}

func TestTrackedFinding_MarkSeen(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now)
	tf.MarkSeen(later)

	if tf.SeenCount != 2 {
		t.Errorf("SeenCount = %d, want 2", tf.SeenCount)
	}

	if tf.LastSeen != later {
		t.Errorf("LastSeen = %v, want %v", tf.LastSeen, later)
	}

	if tf.FirstSeen != now {
		t.Errorf("FirstSeen should not change: got %v, want %v", tf.FirstSeen, now)
	}
}

func TestTrackedFinding_UpdateStatus(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now)

	if err := tf.UpdateStatus(FindingStatusResolved); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if tf.Status != FindingStatusResolved {
		t.Errorf("Status = %v, want %v", tf.Status, FindingStatusResolved)
	}
}

func TestTrackedFinding_UpdateStatus_Invalid(t *testing.T) {
	now := time.Now()
	finding := NewFinding(FindingInput{
		File:        "main.go",
		LineStart:   10,
		LineEnd:     10,
		Severity:    "high",
		Category:    "security",
		Description: "Test",
		Suggestion:  "",
		Evidence:    false,
	})

	tf, _ := NewTrackedFindingFromFinding(finding, now)

	if err := tf.UpdateStatus(FindingStatus("invalid")); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestTrackedFinding_IsActive(t *testing.T) {
	tests := []struct {
		status FindingStatus
		want   bool
	}{
		{FindingStatusOpen, true},
		{FindingStatusResolved, false},
		{FindingStatusAcknowledged, false},
		{FindingStatusDisputed, false},
	}

	now := time.Now()
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			finding := NewFinding(FindingInput{
				File:        "main.go",
				LineStart:   10,
				LineEnd:     10,
				Severity:    "high",
				Category:    "security",
				Description: "Test",
				Suggestion:  "",
				Evidence:    false,
			})

			tf, _ := NewTrackedFinding(TrackedFindingInput{
				Finding:   finding,
				Status:    tt.status,
				FirstSeen: now,
				LastSeen:  now,
				SeenCount: 1,
			})

			if got := tf.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackedFinding_IsResolved(t *testing.T) {
	tests := []struct {
		status FindingStatus
		want   bool
	}{
		{FindingStatusOpen, false},
		{FindingStatusResolved, true},
		{FindingStatusAcknowledged, false},
		{FindingStatusDisputed, false},
	}

	now := time.Now()
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			finding := NewFinding(FindingInput{
				File:        "main.go",
				LineStart:   10,
				LineEnd:     10,
				Severity:    "high",
				Category:    "security",
				Description: "Test",
				Suggestion:  "",
				Evidence:    false,
			})

			tf, _ := NewTrackedFinding(TrackedFindingInput{
				Finding:   finding,
				Status:    tt.status,
				FirstSeen: now,
				LastSeen:  now,
				SeenCount: 1,
			})

			if got := tf.IsResolved(); got != tt.want {
				t.Errorf("IsResolved() = %v, want %v", got, tt.want)
			}
		})
	}
}
