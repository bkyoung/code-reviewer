package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	FileStatusAdded    = "added"
	FileStatusModified = "modified"
	FileStatusDeleted  = "deleted"
	FileStatusRenamed  = "renamed"
)

// Diff represents a cumulative diff between two refs.
type Diff struct {
	FromCommitHash string
	ToCommitHash   string
	Files          []FileDiff
}

// FileDiff captures the change for a single file.
type FileDiff struct {
	Path     string
	OldPath  string // Set when Status == FileStatusRenamed
	Status   string
	Patch    string
	IsBinary bool // True for binary files (patch contains "Binary files differ")
}

// Review is the output from an LLM provider.
type Review struct {
	ProviderName string    `json:"providerName"`
	ModelName    string    `json:"modelName"`
	Summary      string    `json:"summary"`
	Findings     []Finding `json:"findings"`
	Cost         float64   `json:"cost"` // Cost in USD
}

// Finding represents a single issue detected by an LLM.
type Finding struct {
	ID          string `json:"id"`
	File        string `json:"file"`
	LineStart   int    `json:"lineStart"`
	LineEnd     int    `json:"lineEnd"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	Evidence    bool   `json:"evidence"`
}

// FindingInput captures the information required to create a Finding.
type FindingInput struct {
	File        string
	LineStart   int
	LineEnd     int
	Severity    string
	Category    string
	Description string
	Suggestion  string
	Evidence    bool
}

// NewFinding constructs a Finding with a deterministic ID.
func NewFinding(input FindingInput) Finding {
	id := hashFinding(input)
	return Finding{
		ID:          id,
		File:        input.File,
		LineStart:   input.LineStart,
		LineEnd:     input.LineEnd,
		Severity:    input.Severity,
		Category:    input.Category,
		Description: input.Description,
		Suggestion:  input.Suggestion,
		Evidence:    input.Evidence,
	}
}

func hashFinding(input FindingInput) string {
	payload := fmt.Sprintf("%s|%d|%d|%s|%s|%s|%t",
		input.File,
		input.LineStart,
		input.LineEnd,
		input.Severity,
		input.Category,
		input.Description,
		input.Evidence,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// Fingerprint returns a stable identifier for this finding.
// The fingerprint is based on file, category, severity, and description prefix.
// Line numbers are intentionally excluded so the fingerprint remains stable
// when code shifts due to unrelated changes.
func (f Finding) Fingerprint() FindingFingerprint {
	return FingerprintFromFinding(f)
}

// MarkdownArtifact encapsulates the Markdown generation inputs.
type MarkdownArtifact struct {
	OutputDir    string
	Repository   string
	BaseRef      string
	TargetRef    string
	Diff         Diff
	Review       Review
	ProviderName string
}

// JSONArtifact encapsulates the JSON generation inputs.
type JSONArtifact struct {
	OutputDir    string
	Repository   string
	BaseRef      string
	TargetRef    string
	Review       Review
	ProviderName string
}
