package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// LoaderOptions describes how configuration should be discovered.
type LoaderOptions struct {
	ConfigPaths []string
	FileName    string
	EnvPrefix   string
}

// Load returns the merged configuration from files and environment variables.
func Load(opts LoaderOptions) (Config, error) {
	v := viper.New()

	name := opts.FileName
	if name == "" {
		name = "cr"
	}

	configFile := locateConfigFile(name, opts.ConfigPaths)
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName(name)
	}

	prefix := opts.EnvPrefix
	if prefix == "" {
		prefix = "CR"
	}
	v.SetEnvPrefix(prefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AllowEmptyEnv(true)

	setDefaults(v)

	if configFile != "" {
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", configFile, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

func locateConfigFile(name string, paths []string) string {
	searchPaths := append([]string{}, paths...)
	searchPaths = append(searchPaths, ".")
	for _, dir := range searchPaths {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name+".yaml")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("output.directory", "out")

	// Determinism defaults (Phase 2)
	v.SetDefault("determinism.enabled", true)
	v.SetDefault("determinism.temperature", 0.0)
	v.SetDefault("determinism.useSeed", true)

	// Redaction defaults (Phase 2)
	v.SetDefault("redaction.enabled", true)

	// Merge defaults (Phase 2)
	v.SetDefault("merge.enabled", true)
	v.SetDefault("merge.strategy", "consensus")

	// Provider defaults (Phase 1 + Phase 2)
	v.SetDefault("providers.openai.enabled", false)
	v.SetDefault("providers.openai.model", "gpt-4o")
	v.SetDefault("providers.anthropic.enabled", false)
	v.SetDefault("providers.anthropic.model", "claude-3-5-sonnet-20241022")
	v.SetDefault("providers.gemini.enabled", false)
	v.SetDefault("providers.gemini.model", "gemini-pro")
	v.SetDefault("providers.ollama.enabled", false)
	v.SetDefault("providers.ollama.model", "llama2")
	v.SetDefault("providers.static.enabled", true)
	v.SetDefault("providers.static.model", "static-v1")
}
