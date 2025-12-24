package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bkyoung/code-reviewer/internal/config"
)

func TestMergePrioritizesLaterConfigs(t *testing.T) {
	base := config.Config{
		Output: config.OutputConfig{Directory: "default"},
	}
	file := config.Config{
		Output: config.OutputConfig{Directory: "file"},
	}
	final := config.Config{
		Output: config.OutputConfig{Directory: "env"},
	}

	merged := config.Merge(base, file, final)

	if merged.Output.Directory != "env" {
		t.Fatalf("expected env directory to win, got %s", merged.Output.Directory)
	}
}

func TestLoadReadsFromFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cr.yaml")
	if err := os.WriteFile(file, []byte("output:\n  directory: file\n"), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("CR_OUTPUT_DIRECTORY", "env")

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{dir},
		FileName:    "cr",
		EnvPrefix:   "CR",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if cfg.Output.Directory != "env" {
		t.Fatalf("expected env override, got %s", cfg.Output.Directory)
	}
}

func TestObservabilityConfigDefaults(t *testing.T) {
	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{},
		FileName:    "nonexistent",
		EnvPrefix:   "CR",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	// Verify default observability settings
	if !cfg.Observability.Logging.Enabled {
		t.Error("expected logging to be enabled by default")
	}
	if cfg.Observability.Logging.Level != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.Observability.Logging.Level)
	}
	if cfg.Observability.Logging.Format != "human" {
		t.Errorf("expected default log format 'human', got %s", cfg.Observability.Logging.Format)
	}
	if !cfg.Observability.Logging.RedactAPIKeys {
		t.Error("expected API key redaction to be enabled by default")
	}
	if !cfg.Observability.Metrics.Enabled {
		t.Error("expected metrics to be enabled by default")
	}
}

func TestObservabilityConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cr.yaml")
	content := `
observability:
  logging:
    enabled: false
    level: debug
    format: json
    redactAPIKeys: false
  metrics:
    enabled: false
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{dir},
		FileName:    "cr",
		EnvPrefix:   "CR",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	// Verify file overrides defaults
	if cfg.Observability.Logging.Enabled {
		t.Error("expected logging to be disabled from file config")
	}
	if cfg.Observability.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %s", cfg.Observability.Logging.Level)
	}
	if cfg.Observability.Logging.Format != "json" {
		t.Errorf("expected log format 'json', got %s", cfg.Observability.Logging.Format)
	}
	if cfg.Observability.Logging.RedactAPIKeys {
		t.Error("expected API key redaction to be disabled from file config")
	}
	if cfg.Observability.Metrics.Enabled {
		t.Error("expected metrics to be disabled from file config")
	}
}

func TestReviewActionsDefaults(t *testing.T) {
	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{},
		FileName:    "nonexistent",
		EnvPrefix:   "CR",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	// Verify default review actions (sensible defaults)
	if cfg.Review.Actions.OnCritical != "request_changes" {
		t.Errorf("expected OnCritical 'request_changes', got %s", cfg.Review.Actions.OnCritical)
	}
	if cfg.Review.Actions.OnHigh != "request_changes" {
		t.Errorf("expected OnHigh 'request_changes', got %s", cfg.Review.Actions.OnHigh)
	}
	if cfg.Review.Actions.OnMedium != "comment" {
		t.Errorf("expected OnMedium 'comment', got %s", cfg.Review.Actions.OnMedium)
	}
	if cfg.Review.Actions.OnLow != "comment" {
		t.Errorf("expected OnLow 'comment', got %s", cfg.Review.Actions.OnLow)
	}
	if cfg.Review.Actions.OnClean != "approve" {
		t.Errorf("expected OnClean 'approve', got %s", cfg.Review.Actions.OnClean)
	}
}

func TestReviewActionsFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cr.yaml")
	content := `
review:
  actions:
    onCritical: comment
    onHigh: approve
    onMedium: request_changes
    onLow: approve
    onClean: comment
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{dir},
		FileName:    "cr",
		EnvPrefix:   "CR",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	// Verify file overrides defaults
	if cfg.Review.Actions.OnCritical != "comment" {
		t.Errorf("expected OnCritical 'comment', got %s", cfg.Review.Actions.OnCritical)
	}
	if cfg.Review.Actions.OnHigh != "approve" {
		t.Errorf("expected OnHigh 'approve', got %s", cfg.Review.Actions.OnHigh)
	}
	if cfg.Review.Actions.OnMedium != "request_changes" {
		t.Errorf("expected OnMedium 'request_changes', got %s", cfg.Review.Actions.OnMedium)
	}
	if cfg.Review.Actions.OnLow != "approve" {
		t.Errorf("expected OnLow 'approve', got %s", cfg.Review.Actions.OnLow)
	}
	if cfg.Review.Actions.OnClean != "comment" {
		t.Errorf("expected OnClean 'comment', got %s", cfg.Review.Actions.OnClean)
	}
}

func TestReviewActionsEnvOverride(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cr.yaml")
	content := `
review:
  actions:
    onCritical: comment
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Environment variable should override file
	t.Setenv("CR_REVIEW_ACTIONS_ONCRITICAL", "approve")

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{dir},
		FileName:    "cr",
		EnvPrefix:   "CR",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	// Verify env var overrides file
	if cfg.Review.Actions.OnCritical != "approve" {
		t.Errorf("expected OnCritical 'approve' from env var, got %s", cfg.Review.Actions.OnCritical)
	}
}

func TestReviewActionsMerge(t *testing.T) {
	base := config.Config{
		Review: config.ReviewConfig{
			Instructions: "base instructions",
			Actions: config.ReviewActions{
				OnCritical: "request_changes",
				OnHigh:     "request_changes",
			},
		},
	}
	overlay := config.Config{
		Review: config.ReviewConfig{
			Actions: config.ReviewActions{
				OnHigh:   "approve",
				OnMedium: "comment",
			},
		},
	}

	merged := config.Merge(base, overlay)

	// Overlay with non-empty actions should replace
	if merged.Review.Actions.OnHigh != "approve" {
		t.Errorf("expected OnHigh 'approve' from overlay, got %s", merged.Review.Actions.OnHigh)
	}
	if merged.Review.Actions.OnMedium != "comment" {
		t.Errorf("expected OnMedium 'comment' from overlay, got %s", merged.Review.Actions.OnMedium)
	}
	// Instructions should be preserved from base (overlay is empty)
	if merged.Review.Instructions != "base instructions" {
		t.Errorf("expected base instructions to be preserved, got %s", merged.Review.Instructions)
	}
}

func TestBotUsernameDefault(t *testing.T) {
	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{},
		FileName:    "nonexistent",
		EnvPrefix:   "CR_TEST_BOTUSER",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if cfg.Review.BotUsername != "github-actions[bot]" {
		t.Errorf("expected default BotUsername 'github-actions[bot]', got %s", cfg.Review.BotUsername)
	}
}

func TestBotUsernameFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cr.yaml")
	content := `
review:
  botUsername: "custom-bot[bot]"
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{dir},
		FileName:    "cr",
		EnvPrefix:   "CR_TEST_BOTUSER2",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if cfg.Review.BotUsername != "custom-bot[bot]" {
		t.Errorf("expected BotUsername 'custom-bot[bot]' from file, got %s", cfg.Review.BotUsername)
	}
}

func TestBotUsernameMerge(t *testing.T) {
	base := config.Config{
		Review: config.ReviewConfig{
			BotUsername: "base-bot[bot]",
		},
	}
	overlay := config.Config{
		Review: config.ReviewConfig{
			BotUsername: "overlay-bot[bot]",
		},
	}

	merged := config.Merge(base, overlay)

	if merged.Review.BotUsername != "overlay-bot[bot]" {
		t.Errorf("expected BotUsername 'overlay-bot[bot]' from overlay, got %s", merged.Review.BotUsername)
	}
}

func TestBotUsernameMergePreservesBase(t *testing.T) {
	base := config.Config{
		Review: config.ReviewConfig{
			BotUsername: "base-bot[bot]",
		},
	}
	overlay := config.Config{
		Review: config.ReviewConfig{
			// Empty BotUsername should preserve base
		},
	}

	merged := config.Merge(base, overlay)

	if merged.Review.BotUsername != "base-bot[bot]" {
		t.Errorf("expected BotUsername 'base-bot[bot]' from base, got %s", merged.Review.BotUsername)
	}
}

func TestVerificationConfigDefaults(t *testing.T) {
	cfg, err := config.Load(config.LoaderOptions{
		EnvPrefix: "CR_TEST_VERIF_DEFAULTS",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	// Check defaults are applied
	if !cfg.Verification.Enabled {
		t.Error("expected Verification.Enabled to be true by default")
	}
	if cfg.Verification.Depth != "medium" {
		t.Errorf("expected Verification.Depth 'medium', got %s", cfg.Verification.Depth)
	}
	if cfg.Verification.CostCeiling != 0.50 {
		t.Errorf("expected Verification.CostCeiling 0.50, got %f", cfg.Verification.CostCeiling)
	}
	if cfg.Verification.Confidence.Default != 75 {
		t.Errorf("expected Verification.Confidence.Default 75, got %d", cfg.Verification.Confidence.Default)
	}
	if cfg.Verification.Confidence.Critical != 60 {
		t.Errorf("expected Verification.Confidence.Critical 60, got %d", cfg.Verification.Confidence.Critical)
	}
	if cfg.Verification.Confidence.High != 70 {
		t.Errorf("expected Verification.Confidence.High 70, got %d", cfg.Verification.Confidence.High)
	}
	if cfg.Verification.Confidence.Medium != 75 {
		t.Errorf("expected Verification.Confidence.Medium 75, got %d", cfg.Verification.Confidence.Medium)
	}
	if cfg.Verification.Confidence.Low != 85 {
		t.Errorf("expected Verification.Confidence.Low 85, got %d", cfg.Verification.Confidence.Low)
	}
}

func TestVerificationConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cr.yaml")
	content := `
verification:
  enabled: true
  depth: deep
  costCeiling: 1.25
  confidence:
    default: 80
    critical: 50
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := config.Load(config.LoaderOptions{
		ConfigPaths: []string{dir},
		FileName:    "cr",
		EnvPrefix:   "CR_TEST_VERIF_FILE",
	})
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	if !cfg.Verification.Enabled {
		t.Error("expected Verification.Enabled to be true from file")
	}
	if cfg.Verification.Depth != "deep" {
		t.Errorf("expected Verification.Depth 'deep', got %s", cfg.Verification.Depth)
	}
	if cfg.Verification.CostCeiling != 1.25 {
		t.Errorf("expected Verification.CostCeiling 1.25, got %f", cfg.Verification.CostCeiling)
	}
	if cfg.Verification.Confidence.Default != 80 {
		t.Errorf("expected Verification.Confidence.Default 80, got %d", cfg.Verification.Confidence.Default)
	}
	if cfg.Verification.Confidence.Critical != 50 {
		t.Errorf("expected Verification.Confidence.Critical 50, got %d", cfg.Verification.Confidence.Critical)
	}
}

func TestVerificationConfigMerge(t *testing.T) {
	base := config.Config{
		Verification: config.VerificationConfig{
			Enabled:     false,
			Depth:       "quick",
			CostCeiling: 0.25,
			Confidence: config.ConfidenceThresholds{
				Default:  70,
				Critical: 55,
				High:     65,
				Medium:   70,
				Low:      80,
			},
		},
	}
	overlay := config.Config{
		Verification: config.VerificationConfig{
			Enabled: true,
			Depth:   "deep",
		},
	}

	merged := config.Merge(base, overlay)

	// Field-by-field merge: overlay fields override base, unset fields preserved from base
	if !merged.Verification.Enabled {
		t.Error("expected Verification.Enabled to be true from overlay")
	}
	if merged.Verification.Depth != "deep" {
		t.Errorf("expected Verification.Depth 'deep' from overlay, got %s", merged.Verification.Depth)
	}
	// CostCeiling not set in overlay, should be preserved from base
	if merged.Verification.CostCeiling != 0.25 {
		t.Errorf("expected Verification.CostCeiling 0.25 from base, got %f", merged.Verification.CostCeiling)
	}
	// Confidence thresholds not set in overlay, should be preserved from base
	if merged.Verification.Confidence.Default != 70 {
		t.Errorf("expected Verification.Confidence.Default 70 from base, got %d", merged.Verification.Confidence.Default)
	}
}

func TestVerificationConfigMergePreservesBase(t *testing.T) {
	base := config.Config{
		Verification: config.VerificationConfig{
			Enabled:     true,
			Depth:       "quick",
			CostCeiling: 0.25,
			Confidence: config.ConfidenceThresholds{
				Default: 70,
			},
		},
	}
	overlay := config.Config{
		// Empty verification config - should preserve base
	}

	merged := config.Merge(base, overlay)

	if !merged.Verification.Enabled {
		t.Error("expected Verification.Enabled to be preserved from base")
	}
	if merged.Verification.Depth != "quick" {
		t.Errorf("expected Verification.Depth 'quick' from base, got %s", merged.Verification.Depth)
	}
	if merged.Verification.CostCeiling != 0.25 {
		t.Errorf("expected Verification.CostCeiling 0.25 from base, got %f", merged.Verification.CostCeiling)
	}
	if merged.Verification.Confidence.Default != 70 {
		t.Errorf("expected Verification.Confidence.Default 70 from base, got %d", merged.Verification.Confidence.Default)
	}
}

func TestVerificationConfigMergeCanDisable(t *testing.T) {
	base := config.Config{
		Verification: config.VerificationConfig{
			Enabled:     true,
			Depth:       "medium",
			CostCeiling: 0.50,
		},
	}
	overlay := config.Config{
		Verification: config.VerificationConfig{
			Enabled: false,      // Explicitly disable
			Depth:   "disabled", // Set another field to signal intentional config
		},
	}

	merged := config.Merge(base, overlay)

	// Overlay should be able to disable verification when other fields are set
	if merged.Verification.Enabled {
		t.Error("expected Verification.Enabled to be false (disabled by overlay)")
	}
	if merged.Verification.Depth != "disabled" {
		t.Errorf("expected Verification.Depth 'disabled' from overlay, got %s", merged.Verification.Depth)
	}
}
