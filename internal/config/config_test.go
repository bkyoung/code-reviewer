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
