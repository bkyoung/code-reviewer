package config

import "strings"

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
	Deduplication DeduplicationConfig       `yaml:"deduplication"`
	SizeGuards    SizeGuardsConfig          `yaml:"sizeGuards"`
}

// ProviderConfig configures a single LLM provider.
type ProviderConfig struct {
	// Enabled controls whether this provider is used for reviews.
	// This is a tri-state field with the following semantics:
	//   - nil (not set in config): Provider is enabled if APIKey is non-empty.
	//     This preserves backward compatibility with configs that only set apiKey.
	//   - true: Provider is explicitly enabled, even without an APIKey.
	//     Use this for keyless providers like Ollama that don't require authentication.
	//   - false: Provider is explicitly disabled, even if APIKey is present.
	//     Use this to temporarily disable a provider without removing credentials.
	Enabled *bool  `yaml:"enabled,omitempty"`
	Model   string `yaml:"model"`
	APIKey  string `yaml:"apiKey"`

	// MaxOutputTokens overrides the default max output tokens for this provider.
	// Use this for models with different output limits (e.g., older models with 8K,
	// or newer models with 128K+). Default: 64000 (works for Claude 4.5, GPT-5.2, Gemini 3).
	MaxOutputTokens *int `yaml:"maxOutputTokens,omitempty"`

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

	// BlockThreshold is syntactic sugar for setting per-severity actions.
	// Valid values: "critical", "high", "medium", "low", "none"
	// - "critical": only critical findings block (request_changes)
	// - "high": critical and high block (default behavior)
	// - "medium": critical, high, and medium block
	// - "low": all severities block
	// - "none": nothing blocks (all findings are informational)
	// Explicit per-severity actions (Actions.OnCritical, etc.) override this threshold.
	BlockThreshold string `yaml:"blockThreshold"`

	// AlwaysBlockCategories lists finding categories that always trigger REQUEST_CHANGES
	// regardless of severity. This is additive - if a finding's category matches,
	// it blocks even if the severity threshold would not.
	// Example: ["security", "bug"] - security and bug findings always block
	AlwaysBlockCategories []string `yaml:"alwaysBlockCategories"`
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

	// Provider is the LLM provider for verification (e.g., "gemini", "anthropic", "openai").
	// Default: "gemini"
	Provider string `yaml:"provider"`

	// Model is the model to use for verification.
	// Default: "gemini-3-flash-preview" (fast, large context, cost-effective)
	Model string `yaml:"model"`

	// MaxTokens is the maximum output tokens for batch verification responses.
	// Default: 64000 (large enough for many findings)
	MaxTokens int `yaml:"maxTokens"`

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
// After merging, threshold expansion and defaults are applied to the Review config.
func Merge(configs ...Config) Config {
	result := Config{}
	for _, cfg := range configs {
		result = merge(result, cfg)
	}
	// Apply threshold expansion and defaults after all merging is complete
	result.Review = processReviewConfig(result.Review)
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
	result.Deduplication = chooseDeduplication(base.Deduplication, overlay.Deduplication)
	result.SizeGuards = chooseSizeGuards(base.SizeGuards, overlay.SizeGuards)
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

	// BlockThreshold: overlay wins if non-empty
	if overlay.BlockThreshold != "" {
		result.BlockThreshold = overlay.BlockThreshold
	}

	// Actions: merge base and overlay (overlay wins for non-empty fields)
	if overlay.Actions.hasAny() {
		result.Actions = mergeReviewActions(base.Actions, overlay.Actions)
	}

	// NOTE: Threshold expansion and defaults are NOT applied here.
	// They are applied in processReviewConfig() which is called by Load()
	// after all config sources are merged. This prevents defaults from one
	// merge iteration from overriding threshold expansion in a later iteration.

	// BotUsername: overlay wins if non-empty
	if overlay.BotUsername != "" {
		result.BotUsername = overlay.BotUsername
	}

	// AlwaysBlockCategories: union of base and overlay (additive)
	result.AlwaysBlockCategories = mergeCategories(base.AlwaysBlockCategories, overlay.AlwaysBlockCategories)

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

// applyActionDefaults fills in empty action slots with sensible defaults.
// Default behavior: critical/high block (request_changes), medium/low don't block (comment),
// clean reviews get approved, non-blocking findings get approved.
func applyActionDefaults(actions ReviewActions) ReviewActions {
	if actions.OnCritical == "" {
		actions.OnCritical = "request_changes"
	}
	if actions.OnHigh == "" {
		actions.OnHigh = "request_changes"
	}
	if actions.OnMedium == "" {
		actions.OnMedium = "comment"
	}
	if actions.OnLow == "" {
		actions.OnLow = "comment"
	}
	if actions.OnClean == "" {
		actions.OnClean = "approve"
	}
	if actions.OnNonBlocking == "" {
		actions.OnNonBlocking = "approve"
	}
	return actions
}

// expandBlockThreshold converts a threshold string to explicit per-severity actions.
// Valid thresholds: "critical", "high", "medium", "low", "none"
// - "critical": only critical blocks
// - "high": critical and high block (matches default behavior)
// - "medium": critical, high, and medium block
// - "low": all severities block
// - "none": nothing blocks (all comment only)
// Returns zero-value ReviewActions if threshold is empty or invalid.
func expandBlockThreshold(threshold string) ReviewActions {
	if threshold == "" {
		return ReviewActions{}
	}

	// Severity levels in order from highest to lowest
	// The threshold means "block at this level and above"
	// "none" is set to 5 (above critical=4) so no severity meets the threshold
	severityLevels := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
		"none":     5, // Above all severities - nothing blocks
	}

	thresholdLevel, ok := severityLevels[strings.ToLower(threshold)]
	if !ok {
		// Invalid threshold - return empty (will use defaults)
		return ReviewActions{}
	}

	var actions ReviewActions

	// A severity blocks if its level >= threshold level
	// e.g., threshold "high" (level 3): critical (4) and high (3) block, medium (2) and low (1) don't
	const (
		criticalLevel = 4
		highLevel     = 3
		mediumLevel   = 2
		lowLevel      = 1
	)

	if criticalLevel >= thresholdLevel {
		actions.OnCritical = "request_changes"
	} else {
		actions.OnCritical = "comment"
	}

	if highLevel >= thresholdLevel {
		actions.OnHigh = "request_changes"
	} else {
		actions.OnHigh = "comment"
	}

	if mediumLevel >= thresholdLevel {
		actions.OnMedium = "request_changes"
	} else {
		actions.OnMedium = "comment"
	}

	if lowLevel >= thresholdLevel {
		actions.OnLow = "request_changes"
	} else {
		actions.OnLow = "comment"
	}

	return actions
}

// mergeCategories returns the union of two category slices, preserving order and deduplicating.
func mergeCategories(base, overlay []string) []string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var result []string

	for _, cat := range base {
		normalized := strings.ToLower(strings.TrimSpace(cat))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			result = append(result, cat)
		}
	}

	for _, cat := range overlay {
		normalized := strings.ToLower(strings.TrimSpace(cat))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			result = append(result, cat)
		}
	}

	return result
}

func chooseVerification(base, overlay VerificationConfig) VerificationConfig {
	result := base

	// If overlay has any verification config set, use its Enabled value
	// This allows overlay to disable verification (Enabled=false) when other fields are set
	if hasAnyVerificationConfig(overlay) {
		result.Enabled = overlay.Enabled
	}

	// Provider: overlay wins if non-empty
	if overlay.Provider != "" {
		result.Provider = overlay.Provider
	}

	// Model: overlay wins if non-empty
	if overlay.Model != "" {
		result.Model = overlay.Model
	}

	// MaxTokens: overlay wins if non-zero
	if overlay.MaxTokens != 0 {
		result.MaxTokens = overlay.MaxTokens
	}

	// Depth: overlay wins if non-empty
	if overlay.Depth != "" {
		result.Depth = overlay.Depth
	}

	// CostCeiling: overlay wins if non-zero
	if overlay.CostCeiling != 0 {
		result.CostCeiling = overlay.CostCeiling
	}

	// Confidence: overlay wins if any field is set
	if hasConfidenceThresholds(overlay.Confidence) {
		result.Confidence = overlay.Confidence
	}

	return result
}

// hasAnyVerificationConfig returns true if any verification field is set in the config.
// This is used to determine if the Enabled field should be respected from the overlay.
func hasAnyVerificationConfig(vc VerificationConfig) bool {
	return vc.Enabled ||
		vc.Provider != "" ||
		vc.Model != "" ||
		vc.MaxTokens != 0 ||
		vc.Depth != "" ||
		vc.CostCeiling != 0 ||
		hasConfidenceThresholds(vc.Confidence)
}

func hasConfidenceThresholds(ct ConfidenceThresholds) bool {
	return ct.Default != 0 || ct.Critical != 0 || ct.High != 0 || ct.Medium != 0 || ct.Low != 0
}

// DeduplicationConfig configures semantic deduplication of findings.
// When enabled, findings that overlap spatially but have different fingerprints
// are compared using an LLM to detect semantic duplicates.
type DeduplicationConfig struct {
	// Semantic configures the LLM-based semantic deduplication (stage 2).
	// Stage 1 (fingerprint matching) is always enabled and has no configuration.
	Semantic SemanticDeduplicationConfig `yaml:"semantic"`
}

// SemanticDeduplicationConfig configures LLM-based semantic deduplication.
type SemanticDeduplicationConfig struct {
	// Enabled toggles semantic deduplication.
	// Default: true
	Enabled *bool `yaml:"enabled,omitempty"`

	// Provider is the LLM provider for semantic comparison (e.g., "anthropic", "openai").
	// Default: "anthropic"
	Provider string `yaml:"provider"`

	// Model is the model to use for semantic comparison.
	// Default: "claude-haiku-4-5-latest"
	Model string `yaml:"model"`

	// MaxTokens is the maximum output tokens for the deduplication response.
	// Default: 64000 (sufficient for all current Claude/GPT/Gemini models)
	MaxTokens int `yaml:"maxTokens"`

	// LineThreshold is the maximum line distance for findings to be considered
	// potentially duplicate. Findings further apart are not compared.
	// Default: 10
	LineThreshold int `yaml:"lineThreshold"`

	// MaxCandidates is the maximum number of candidate pairs to send for
	// semantic comparison per review. This acts as a cost guard.
	// Default: 50
	MaxCandidates int `yaml:"maxCandidates"`
}

// chooseDeduplication merges DeduplicationConfig with overlay taking precedence.
func chooseDeduplication(base, overlay DeduplicationConfig) DeduplicationConfig {
	result := base

	// Merge semantic config
	result.Semantic = chooseSemanticDeduplication(base.Semantic, overlay.Semantic)

	return result
}

// chooseSemanticDeduplication merges SemanticDeduplicationConfig.
func chooseSemanticDeduplication(base, overlay SemanticDeduplicationConfig) SemanticDeduplicationConfig {
	result := base

	// Enabled: overlay wins if set (not nil)
	if overlay.Enabled != nil {
		result.Enabled = overlay.Enabled
	}

	// Provider: overlay wins if non-empty
	if overlay.Provider != "" {
		result.Provider = overlay.Provider
	}

	// Model: overlay wins if non-empty
	if overlay.Model != "" {
		result.Model = overlay.Model
	}

	// MaxTokens: overlay wins if non-zero
	if overlay.MaxTokens != 0 {
		result.MaxTokens = overlay.MaxTokens
	}

	// LineThreshold: overlay wins if non-zero
	if overlay.LineThreshold != 0 {
		result.LineThreshold = overlay.LineThreshold
	}

	// MaxCandidates: overlay wins if non-zero
	if overlay.MaxCandidates != 0 {
		result.MaxCandidates = overlay.MaxCandidates
	}

	return result
}

// SizeGuardsConfig configures PR size limits and truncation behavior.
// This prevents context overflow when reviewing large PRs by warning at
// a threshold and truncating at a maximum.
type SizeGuardsConfig struct {
	// Enabled toggles size guard functionality.
	// Default: true
	Enabled *bool `yaml:"enabled,omitempty"`

	// WarnTokens is the token count at which to emit a warning.
	// The review continues but includes a note about size.
	// Default: 150000 (targets Claude 4.5's 200k context with margin)
	WarnTokens int `yaml:"warnTokens"`

	// MaxTokens is the maximum token count before truncation.
	// Files are removed by priority until under this limit.
	// Default: 200000 (Claude 4.5's context limit)
	MaxTokens int `yaml:"maxTokens"`

	// Providers allows per-provider override of size limits.
	// Use this when targeting providers with different context limits
	// (e.g., Gemini 1.5 Pro has 1M+ tokens, older GPT-4 has 128k).
	Providers map[string]ProviderSizeConfig `yaml:"providers,omitempty"`
}

// ProviderSizeConfig allows per-provider size limit overrides.
type ProviderSizeConfig struct {
	// WarnTokens overrides the global warn threshold for this provider.
	WarnTokens int `yaml:"warnTokens,omitempty"`

	// MaxTokens overrides the global max threshold for this provider.
	MaxTokens int `yaml:"maxTokens,omitempty"`
}

// GetLimitsForProvider returns the warn and max token limits for a specific provider.
// If the provider has overrides configured, those are used; otherwise global defaults apply.
// If warn > max (misconfiguration), the values are swapped to maintain the invariant.
func (c SizeGuardsConfig) GetLimitsForProvider(provider string) (warn, max int) {
	warn, max = c.WarnTokens, c.MaxTokens

	// Apply global defaults if not set
	if warn == 0 {
		warn = 150000
	}
	if max == 0 {
		max = 200000
	}

	// Apply provider-specific overrides
	if pc, ok := c.Providers[provider]; ok {
		if pc.WarnTokens > 0 {
			warn = pc.WarnTokens
		}
		if pc.MaxTokens > 0 {
			max = pc.MaxTokens
		}
	}

	// Ensure warn <= max invariant (swap if misconfigured)
	if warn > max {
		warn, max = max, warn
	}

	return warn, max
}

// IsEnabled returns whether size guards are enabled.
// Defaults to true if not explicitly set.
func (c SizeGuardsConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true // Default enabled
	}
	return *c.Enabled
}

// chooseSizeGuards merges SizeGuardsConfig with overlay taking precedence.
func chooseSizeGuards(base, overlay SizeGuardsConfig) SizeGuardsConfig {
	result := base

	// Enabled: overlay wins if set (not nil)
	if overlay.Enabled != nil {
		result.Enabled = overlay.Enabled
	}

	// WarnTokens: overlay wins if non-zero
	if overlay.WarnTokens != 0 {
		result.WarnTokens = overlay.WarnTokens
	}

	// MaxTokens: overlay wins if non-zero
	if overlay.MaxTokens != 0 {
		result.MaxTokens = overlay.MaxTokens
	}

	// Providers: merge maps
	result.Providers = mergeProviderSizeConfigs(base.Providers, overlay.Providers)

	return result
}

// mergeProviderSizeConfigs merges provider size config maps.
func mergeProviderSizeConfigs(base, overlay map[string]ProviderSizeConfig) map[string]ProviderSizeConfig {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	result := make(map[string]ProviderSizeConfig, len(base)+len(overlay))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		// Merge individual provider configs
		if existing, ok := result[key]; ok {
			if value.WarnTokens != 0 {
				existing.WarnTokens = value.WarnTokens
			}
			if value.MaxTokens != 0 {
				existing.MaxTokens = value.MaxTokens
			}
			result[key] = existing
		} else {
			result[key] = value
		}
	}
	return result
}
