package review

import (
	"time"

	"github.com/bkyoung/code-reviewer/internal/domain"
)

// DashboardData aggregates all information needed to render a unified review dashboard.
// This is a platform-agnostic DTO used by the use case layer, allowing adapters
// to render dashboard views appropriate for each platform (GitHub, CLI, etc.).
type DashboardData struct {
	// Target identifies what is being reviewed (repository, PR, branch).
	Target ReviewTarget

	// ReviewedCommits contains SHAs of commits that have been reviewed.
	ReviewedCommits []string

	// Findings maps fingerprints to tracked findings.
	// Used for status counts and findings table.
	Findings map[domain.FindingFingerprint]domain.TrackedFinding

	// LastUpdated is the timestamp of the last dashboard update.
	LastUpdated time.Time

	// ReviewStatus indicates the lifecycle state of the current review.
	// InProgress: Review is running, show simplified dashboard
	// Completed: Review finished, show full findings and summary
	ReviewStatus domain.ReviewStatus

	// Review contains the merged review results.
	// Nil during in-progress state, populated after review completes.
	Review *domain.Review

	// PositionedFindings contains findings with their diff positions.
	// Used for rendering the findings table with file/line links.
	// Nil if not available (e.g., first review or CLI mode).
	PositionedFindings []PositionedFindingData

	// AttentionSeverities maps severity names to whether they trigger blocking.
	// Used to determine which findings appear in "Files Requiring Attention".
	// Keys: "critical", "high", "medium", "low" (lowercase)
	AttentionSeverities map[string]bool

	// ReviewPointerURL is the URL to link from the GitHub review body.
	// This is the URL of the dashboard comment itself.
	ReviewPointerURL string

	// Diff contains the diff used for this review.
	// Used for rendering edge cases (binary files, renames).
	Diff *domain.Diff
}

// PositionedFindingData contains a finding with its position metadata.
// This is a simplified version for the dashboard, without GitHub-specific details.
type PositionedFindingData struct {
	Finding domain.Finding
	InDiff  bool
}

// NewDashboardDataInProgress creates a DashboardData for a review that is starting.
// This produces a minimal "in progress" view.
func NewDashboardDataInProgress(
	target ReviewTarget,
	timestamp time.Time,
	existingCommits []string,
	existingFindings map[domain.FindingFingerprint]domain.TrackedFinding,
) DashboardData {
	// Deep copy to prevent mutation issues
	commits := make([]string, len(existingCommits))
	copy(commits, existingCommits)

	findings := make(map[domain.FindingFingerprint]domain.TrackedFinding, len(existingFindings))
	for k, v := range existingFindings {
		findings[k] = v
	}

	return DashboardData{
		Target:          target,
		ReviewedCommits: commits,
		Findings:        findings,
		LastUpdated:     timestamp,
		ReviewStatus:    domain.ReviewStatusInProgress,
	}
}

// NewDashboardDataCompleted creates a DashboardData for a completed review.
// This produces the full dashboard with findings, costs, and metadata.
// Deep copies mutable data to prevent external modification.
func NewDashboardDataCompleted(
	state TrackingState,
	review *domain.Review,
	positionedFindings []PositionedFindingData,
	attentionSeverities map[string]bool,
	diff *domain.Diff,
) DashboardData {
	// Deep copy ReviewedCommits
	var commits []string
	if len(state.ReviewedCommits) > 0 {
		commits = make([]string, len(state.ReviewedCommits))
		copy(commits, state.ReviewedCommits)
	}

	// Deep copy Findings map
	var findings map[domain.FindingFingerprint]domain.TrackedFinding
	if len(state.Findings) > 0 {
		findings = make(map[domain.FindingFingerprint]domain.TrackedFinding, len(state.Findings))
		for k, v := range state.Findings {
			findings[k] = v
		}
	}

	// Deep copy attentionSeverities map
	var severities map[string]bool
	if len(attentionSeverities) > 0 {
		severities = make(map[string]bool, len(attentionSeverities))
		for k, v := range attentionSeverities {
			severities[k] = v
		}
	}

	// Deep copy positionedFindings slice
	var positioned []PositionedFindingData
	if len(positionedFindings) > 0 {
		positioned = make([]PositionedFindingData, len(positionedFindings))
		copy(positioned, positionedFindings)
	}

	return DashboardData{
		Target:              state.Target,
		ReviewedCommits:     commits,
		Findings:            findings,
		LastUpdated:         state.LastUpdated,
		ReviewStatus:        domain.ReviewStatusCompleted,
		Review:              review,
		PositionedFindings:  positioned,
		AttentionSeverities: severities,
		Diff:                diff,
	}
}

// StatusCounts holds counts of findings by status for summary display.
type StatusCounts struct {
	Open         int
	Resolved     int
	Acknowledged int
	Disputed     int
	Total        int
}

// CountByStatus calculates finding counts by status.
func (d DashboardData) CountByStatus() StatusCounts {
	counts := StatusCounts{}
	for _, f := range d.Findings {
		switch f.Status {
		case domain.FindingStatusOpen:
			counts.Open++
		case domain.FindingStatusResolved:
			counts.Resolved++
		case domain.FindingStatusAcknowledged:
			counts.Acknowledged++
		case domain.FindingStatusDisputed:
			counts.Disputed++
		}
	}
	counts.Total = counts.Open + counts.Resolved + counts.Acknowledged + counts.Disputed
	return counts
}

// SeverityCounts holds counts of findings by severity for badge display.
type SeverityCounts struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Total    int
}

// CountBySeverity calculates finding counts by severity.
// Only counts findings with Open status (active issues).
func (d DashboardData) CountBySeverity() SeverityCounts {
	counts := SeverityCounts{}
	for _, f := range d.Findings {
		if f.Status != domain.FindingStatusOpen {
			continue
		}
		switch f.Finding.Severity {
		case "critical":
			counts.Critical++
		case "high":
			counts.High++
		case "medium":
			counts.Medium++
		case "low":
			counts.Low++
		}
	}
	counts.Total = counts.Critical + counts.High + counts.Medium + counts.Low
	return counts
}

// HasBlockingFindings returns true if any open findings are at blocking severities.
func (d DashboardData) HasBlockingFindings() bool {
	for _, f := range d.Findings {
		if f.Status != domain.FindingStatusOpen {
			continue
		}
		severity := f.Finding.Severity
		if d.AttentionSeverities[severity] {
			return true
		}
	}
	return false
}
