package config

// Config represents the full application configuration.
type Config struct {
	Providers     map[string]ProviderConfig `yaml:"providers"`
	HTTP          HTTPConfig                `yaml:"http"`
	Merge         MergeConfig               `yaml:"merge"`
	Planning      PlanningConfig            `yaml:"planning"`
	Git           GitConfig                 `yaml:"git"`
	Output        OutputConfig              `yaml:"output"`
	Budget        BudgetConfig              `yaml:"budget"`
	Redaction     RedactionConfig           `yaml:"redaction"`
	Determinism   DeterminismConfig         `yaml:"determinism"`
	Store         StoreConfig               `yaml:"store"`
	Observability ObservabilityConfig       `yaml:"observability"`
	Review        ReviewConfig              `yaml:"review"`
	Verification  VerificationConfig        `yaml:"verification"`
}

// ProviderConfig configures a single LLM provider.
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`
	APIKey  string `yaml:"apiKey"`

	// HTTP overrides (optional, use global HTTP config if not set)
	Timeout        *string `yaml:"timeout,omitempty"`
	MaxRetries     *int    `yaml:"maxRetries,omitempty"`
	InitialBackoff *string `yaml:"initialBackoff,omitempty"`
	MaxBackoff     *string `yaml:"maxBackoff,omitempty"`
}

// HTTPConfig holds global HTTP client settings.
type HTTPConfig struct {
	Timeout           string  `yaml:"timeout"`
	MaxRetries        int     `yaml:"maxRetries"`
	InitialBackoff    string  `yaml:"initialBackoff"`
	MaxBackoff        string  `yaml:"maxBackoff"`
	BackoffMultiplier float64 `yaml:"backoffMultiplier"`
}

type MergeConfig struct {
	Enabled  bool               `yaml:"enabled"`
	Provider string             `yaml:"provider"`
	Model    string             `yaml:"model"`
	Strategy string             `yaml:"strategy"`
	Weights  map[string]float64 `yaml:"weights"`
}

// PlanningConfig configures the interactive planning agent.
// The planning agent asks clarifying questions before starting the review
// to improve context and focus. Only runs in interactive (TTY) mode.
type PlanningConfig struct {
	Enabled      bool   `yaml:"enabled"`      // Enable interactive planning
	Provider     string `yaml:"provider"`     // LLM provider for planning (e.g., "openai", "anthropic")
	Model        string `yaml:"model"`        // Model for planning (e.g., "gpt-4o-mini", "claude-3-5-haiku")
	MaxQuestions int    `yaml:"maxQuestions"` // Maximum questions to ask (default: 5)
	Timeout      string `yaml:"timeout"`      // Timeout for planning phase (default: "30s")
}

type GitConfig struct {
	RepositoryDir string `yaml:"repositoryDir"`
}

type OutputConfig struct {
	Directory string `yaml:"directory"`
}

type BudgetConfig struct {
	HardCapUSD        float64  `yaml:"hardCapUSD"`
	DegradationPolicy []string `yaml:"degradationPolicy"`
}

type RedactionConfig struct {
	Enabled    bool     `yaml:"enabled"`
	DenyGlobs  []string `yaml:"denyGlobs"`
	AllowGlobs []string `yaml:"allowGlobs"`
}

type DeterminismConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Temperature float64 `yaml:"temperature"`
	UseSeed     bool    `yaml:"useSeed"`
}

// StoreConfig configures the persistence layer.
type StoreConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// ObservabilityConfig configures logging, metrics, and cost tracking.
type ObservabilityConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
}

// LoggingConfig configures request/response logging.
type LoggingConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Level         string `yaml:"level"`         // debug, info, error
	Format        string `yaml:"format"`        // json, human
	RedactAPIKeys bool   `yaml:"redactAPIKeys"` // Redact API keys in logs
}

// MetricsConfig configures performance and cost metrics tracking.
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ReviewConfig configures the code review behavior.
type ReviewConfig struct {
	// Instructions are custom instructions included in all review prompts.
	// These guide the LLM on what to look for during code review.
	Instructions string `yaml:"instructions"`

	// Actions configures the GitHub review action based on finding severity.
	Actions ReviewActions `yaml:"actions"`

	// BotUsername is the GitHub username of the bot for auto-dismissing stale reviews.
	// When set, previous reviews from this user are dismissed AFTER the new review
	// posts successfully. This ensures the PR always maintains review signal.
	// Set to "none" to explicitly disable auto-dismiss.
	// Default: "github-actions[bot]"
	BotUsername string `yaml:"botUsername"`
}

// ReviewActions maps finding severities to GitHub review actions.
// Valid action values (case-insensitive): approve, comment, request_changes.
type ReviewActions struct {
	// OnCritical is the action when any critical severity finding is present.
	OnCritical string `yaml:"onCritical"`

	// OnHigh is the action when any high severity finding is present (and no critical).
	OnHigh string `yaml:"onHigh"`

	// OnMedium is the action when any medium severity finding is present (and no higher).
	OnMedium string `yaml:"onMedium"`

	// OnLow is the action when any low severity finding is present (and no higher).
	OnLow string `yaml:"onLow"`

	// OnClean is the action when no findings are present in the diff.
	OnClean string `yaml:"onClean"`

	// OnNonBlocking is the action when findings exist but none trigger REQUEST_CHANGES.
	// This allows posting APPROVE with informational comments for low-severity issues.
	OnNonBlocking string `yaml:"onNonBlocking"`
}

// VerificationConfig configures the agent verification behavior.
// When enabled, candidate findings from discovery are verified by an agent
// before being reported.
type VerificationConfig struct {
	// Enabled toggles agent verification of findings.
	Enabled bool `yaml:"enabled"`

	// Depth controls how thoroughly the agent verifies findings.
	// Valid values: "quick" (read file only), "medium" (read + grep), "deep" (run build/tests).
	Depth string `yaml:"depth"`

	// CostCeiling is the maximum USD to spend on verification per review.
	// When reached, remaining candidates are reported as unverified with lower confidence.
	CostCeiling float64 `yaml:"costCeiling"`

	// Confidence contains per-severity confidence thresholds.
	Confidence ConfidenceThresholds `yaml:"confidence"`
}

// ConfidenceThresholds define minimum confidence levels (0-100) for reporting findings.
// Findings below the threshold for their severity level are discarded.
type ConfidenceThresholds struct {
	// Default is used when a severity-specific threshold is not set.
	Default int `yaml:"default"`

	// Critical is the threshold for critical severity findings.
	Critical int `yaml:"critical"`

	// High is the threshold for high severity findings.
	High int `yaml:"high"`

	// Medium is the threshold for medium severity findings.
	Medium int `yaml:"medium"`

	// Low is the threshold for low severity findings.
	Low int `yaml:"low"`
}

// Merge combines multiple configuration instances, prioritising the latter ones.
func Merge(configs ...Config) Config {
	result := Config{}
	for _, cfg := range configs {
		result = merge(result, cfg)
	}
	return result
}

func merge(base, overlay Config) Config {
	result := base

	result.HTTP = chooseHTTP(base.HTTP, overlay.HTTP)
	result.Output = chooseOutput(base.Output, overlay.Output)
	result.Git = chooseGit(base.Git, overlay.Git)
	result.Budget = chooseBudget(base.Budget, overlay.Budget)
	result.Redaction = chooseRedaction(base.Redaction, overlay.Redaction)
	result.Determinism = chooseDeterminism(base.Determinism, overlay.Determinism)
	result.Merge = chooseMerge(base.Merge, overlay.Merge)
	result.Planning = choosePlanning(base.Planning, overlay.Planning)
	result.Store = chooseStore(base.Store, overlay.Store)
	result.Observability = chooseObservability(base.Observability, overlay.Observability)
	result.Review = chooseReview(base.Review, overlay.Review)
	result.Verification = chooseVerification(base.Verification, overlay.Verification)
	result.Providers = mergeProviders(base.Providers, overlay.Providers)

	return result
}

func mergeProviders(base, overlay map[string]ProviderConfig) map[string]ProviderConfig {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	result := make(map[string]ProviderConfig, len(base)+len(overlay))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		result[key] = value
	}
	return result
}

func chooseOutput(base, overlay OutputConfig) OutputConfig {
	if overlay.Directory != "" {
		return overlay
	}
	return base
}

func chooseGit(base, overlay GitConfig) GitConfig {
	if overlay.RepositoryDir != "" {
		return overlay
	}
	return base
}

func chooseHTTP(base, overlay HTTPConfig) HTTPConfig {
	if overlay.Timeout != "" || overlay.MaxRetries != 0 || overlay.InitialBackoff != "" || overlay.MaxBackoff != "" || overlay.BackoffMultiplier != 0 {
		return overlay
	}
	return base
}

func chooseBudget(base, overlay BudgetConfig) BudgetConfig {
	if overlay.HardCapUSD != 0 || len(overlay.DegradationPolicy) > 0 {
		return overlay
	}
	return base
}

func chooseRedaction(base, overlay RedactionConfig) RedactionConfig {
	if overlay.Enabled || len(overlay.DenyGlobs) > 0 || len(overlay.AllowGlobs) > 0 {
		return overlay
	}
	return base
}

func chooseDeterminism(base, overlay DeterminismConfig) DeterminismConfig {
	if overlay.Enabled || overlay.Temperature != 0 || overlay.UseSeed {
		return overlay
	}
	return base
}

func chooseMerge(base, overlay MergeConfig) MergeConfig {
	if overlay.Enabled || overlay.Provider != "" || overlay.Model != "" || overlay.Strategy != "" || len(overlay.Weights) > 0 {
		return overlay
	}
	return base
}

func choosePlanning(base, overlay PlanningConfig) PlanningConfig {
	if overlay.Enabled || overlay.Provider != "" || overlay.Model != "" || overlay.MaxQuestions != 0 || overlay.Timeout != "" {
		return overlay
	}
	return base
}

func chooseStore(base, overlay StoreConfig) StoreConfig {
	if overlay.Enabled || overlay.Path != "" {
		return overlay
	}
	return base
}

func chooseObservability(base, overlay ObservabilityConfig) ObservabilityConfig {
	result := base

	// Merge logging config
	if overlay.Logging.Enabled || overlay.Logging.Level != "" || overlay.Logging.Format != "" {
		result.Logging = overlay.Logging
	}

	// Merge metrics config
	if overlay.Metrics.Enabled {
		result.Metrics = overlay.Metrics
	}

	return result
}

func chooseReview(base, overlay ReviewConfig) ReviewConfig {
	result := base

	// Instructions: overlay wins if non-empty
	if overlay.Instructions != "" {
		result.Instructions = overlay.Instructions
	}

	// Actions: overlay wins if any field is non-empty
	if overlay.Actions.hasAny() {
		result.Actions = mergeReviewActions(base.Actions, overlay.Actions)
	}

	// BotUsername: overlay wins if non-empty
	if overlay.BotUsername != "" {
		result.BotUsername = overlay.BotUsername
	}

	return result
}

// hasAny returns true if any action field is non-empty.
func (a ReviewActions) hasAny() bool {
	return a.OnCritical != "" || a.OnHigh != "" || a.OnMedium != "" || a.OnLow != "" || a.OnClean != "" || a.OnNonBlocking != ""
}

// mergeReviewActions merges two ReviewActions, with overlay taking precedence for non-empty fields.
func mergeReviewActions(base, overlay ReviewActions) ReviewActions {
	result := base
	if overlay.OnCritical != "" {
		result.OnCritical = overlay.OnCritical
	}
	if overlay.OnHigh != "" {
		result.OnHigh = overlay.OnHigh
	}
	if overlay.OnMedium != "" {
		result.OnMedium = overlay.OnMedium
	}
	if overlay.OnLow != "" {
		result.OnLow = overlay.OnLow
	}
	if overlay.OnClean != "" {
		result.OnClean = overlay.OnClean
	}
	if overlay.OnNonBlocking != "" {
		result.OnNonBlocking = overlay.OnNonBlocking
	}
	return result
}

func chooseVerification(base, overlay VerificationConfig) VerificationConfig {
	if overlay.Enabled || overlay.Depth != "" || overlay.CostCeiling != 0 || hasConfidenceThresholds(overlay.Confidence) {
		return overlay
	}
	return base
}

func hasConfidenceThresholds(ct ConfidenceThresholds) bool {
	return ct.Default != 0 || ct.Critical != 0 || ct.High != 0 || ct.Medium != 0 || ct.Low != 0
}
