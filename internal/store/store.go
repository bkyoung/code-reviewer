package store

import (
	"context"
	"time"
)

// Store defines the persistence layer interface for review history and feedback.
type Store interface {
	// Run management
	CreateRun(ctx context.Context, run Run) error
	UpdateRunCost(ctx context.Context, runID string, totalCost float64) error
	GetRun(ctx context.Context, runID string) (Run, error)
	ListRuns(ctx context.Context, limit int) ([]Run, error)

	// Review persistence
	SaveReview(ctx context.Context, review ReviewRecord) error
	GetReview(ctx context.Context, reviewID string) (ReviewRecord, error)
	GetReviewsByRun(ctx context.Context, runID string) ([]ReviewRecord, error)

	// Finding persistence
	SaveFindings(ctx context.Context, findings []FindingRecord) error
	GetFinding(ctx context.Context, findingID string) (FindingRecord, error)
	GetFindingsByReview(ctx context.Context, reviewID string) ([]FindingRecord, error)

	// Feedback management
	RecordFeedback(ctx context.Context, feedback Feedback) error
	GetFeedbackForFinding(ctx context.Context, findingID string) ([]Feedback, error)

	// Precision priors
	GetPrecisionPriors(ctx context.Context) (map[string]map[string]PrecisionPrior, error)
	UpdatePrecisionPrior(ctx context.Context, provider, category string, accepted, rejected int) error

	// Utility
	Close() error
}

// Run represents a single review execution.
type Run struct {
	RunID      string
	Timestamp  time.Time
	Scope      string
	ConfigHash string
	TotalCost  float64
	BaseRef    string
	TargetRef  string
	Repository string
}

// ReviewRecord stores metadata about a review from a single provider.
type ReviewRecord struct {
	ReviewID  string
	RunID     string
	Provider  string
	Model     string
	Summary   string
	CreatedAt time.Time
}

// FindingRecord represents a single finding with all its metadata.
type FindingRecord struct {
	FindingID   string
	ReviewID    string
	FindingHash string
	File        string
	LineStart   int
	LineEnd     int
	Category    string
	Severity    string
	Description string
	Suggestion  string
	Evidence    bool
}

// Feedback records a user's acceptance or rejection of a finding.
type Feedback struct {
	FeedbackID int
	FindingID  string
	Status     string // "accepted" or "rejected"
	Timestamp  time.Time
}

// PrecisionPrior represents the Beta distribution parameters for a provider's accuracy.
type PrecisionPrior struct {
	Provider string
	Category string
	Alpha    float64 // Count of accepted findings
	Beta     float64 // Count of rejected findings
}

// Precision calculates the mean of the Beta distribution (α / (α + β)).
// This represents the estimated precision/accuracy for this provider and category.
func (p PrecisionPrior) Precision() float64 {
	if p.Alpha+p.Beta == 0 {
		return 0.5 // Uniform prior
	}
	return p.Alpha / (p.Alpha + p.Beta)
}
