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
	Finding        Finding  `json:"finding"`        // The underlying finding from an LLM
	Sources        []string `json:"sources"`        // Which LLM providers reported this (e.g., ["openai", "anthropic"])
	AgreementScore float64  `json:"agreementScore"` // 0-1, proportion of LLMs that agreed on this finding
}

// VerifiedFinding represents a finding after agent verification.
// The verification agent has read the full codebase and confirmed or rejected
// the candidate finding, providing classification and confidence scoring.
type VerifiedFinding struct {
	Finding         Finding              `json:"finding"`         // The original finding
	Verified        bool                 `json:"verified"`        // Whether verification confirmed the issue exists
	Classification  Classification       `json:"classification"`  // Category: blocking_bug, security, performance, style
	Confidence      int                  `json:"confidence"`      // 0-100, agent's confidence in the verification
	Evidence        string               `json:"evidence"`        // Agent's explanation and supporting evidence
	BlocksOperation bool                 `json:"blocksOperation"` // Whether this finding should block merge/operation
	VerificationLog []VerificationAction `json:"verificationLog"` // Record of agent actions during verification
}

// VerificationAction records a single tool invocation by the verification agent.
// This provides an audit trail of how the agent verified (or rejected) a finding.
type VerificationAction struct {
	Tool   string `json:"tool"`   // Tool name: "read", "grep", "glob", "bash"
	Input  string `json:"input"`  // Tool input: file path, search pattern, command
	Output string `json:"output"` // Tool output summary (truncation applied at creation time)
}

// VerificationResult encapsulates the outcome of verifying a candidate finding.
// This is the return type from the Verifier interface.
type VerificationResult struct {
	Verified        bool                 `json:"verified"`        // Whether the finding was confirmed
	Classification  Classification       `json:"classification"`  // The determined classification
	Confidence      int                  `json:"confidence"`      // 0-100 confidence score
	Evidence        string               `json:"evidence"`        // Explanation of the verification decision
	BlocksOperation bool                 `json:"blocksOperation"` // Whether this blocks the operation
	Actions         []VerificationAction `json:"actions"`         // Agent actions taken during verification
}
