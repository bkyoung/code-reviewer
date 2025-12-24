package domain

// Classification categorizes verified findings for filtering and reporting.
// Style findings are always discarded. Other classifications determine
// whether the finding blocks the operation or is reported as informational.
type Classification string

const (
	// ClassBlockingBug indicates code that will fail or crash at runtime.
	ClassBlockingBug Classification = "blocking_bug"
	// ClassSecurity indicates a security vulnerability.
	ClassSecurity Classification = "security"
	// ClassPerformance indicates unbounded resource usage or performance issue.
	ClassPerformance Classification = "performance"
	// ClassStyle indicates a style preference or opinion (always discarded).
	ClassStyle Classification = "style"
)

// IsValid returns true if the classification is a recognized value.
func (c Classification) IsValid() bool {
	switch c {
	case ClassBlockingBug, ClassSecurity, ClassPerformance, ClassStyle:
		return true
	default:
		return false
	}
}

// CandidateFinding represents an unverified finding from the discovery phase.
// Multiple LLMs may report the same issue, captured by Sources and AgreementScore.
type CandidateFinding struct {
	Finding        Finding  // The underlying finding from an LLM
	Sources        []string // Which LLM providers reported this (e.g., ["openai", "anthropic"])
	AgreementScore float64  // 0-1, proportion of LLMs that agreed on this finding
}

// VerifiedFinding represents a finding after agent verification.
// The verification agent has read the full codebase and confirmed or rejected
// the candidate finding, providing classification and confidence scoring.
type VerifiedFinding struct {
	Finding         Finding              // The original finding
	Verified        bool                 // Whether verification confirmed the issue exists
	Classification  Classification       // Category: blocking_bug, security, performance, style
	Confidence      int                  // 0-100, agent's confidence in the verification
	Evidence        string               // Agent's explanation and supporting evidence
	BlocksOperation bool                 // Whether this finding should block merge/operation
	VerificationLog []VerificationAction // Record of agent actions during verification
}

// VerificationAction records a single tool invocation by the verification agent.
// This provides an audit trail of how the agent verified (or rejected) a finding.
type VerificationAction struct {
	Tool   string // Tool name: "read", "grep", "glob", "bash"
	Input  string // Tool input: file path, search pattern, command
	Output string // Tool output summary (may be truncated for large outputs)
}

// VerificationResult encapsulates the outcome of verifying a candidate finding.
// This is the return type from the Verifier interface.
type VerificationResult struct {
	Verified        bool                 // Whether the finding was confirmed
	Classification  Classification       // The determined classification
	Confidence      int                  // 0-100 confidence score
	Evidence        string               // Explanation of the verification decision
	BlocksOperation bool                 // Whether this blocks the operation
	Actions         []VerificationAction // Agent actions taken during verification
}
