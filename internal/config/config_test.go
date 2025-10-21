package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brandon/code-reviewer/internal/config"
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
