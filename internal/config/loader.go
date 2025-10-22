package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	// Expand environment variables in config values
	cfg = expandEnvVars(cfg)

	return cfg, nil
}

// expandEnvVars expands ${VAR} and $VAR syntax in configuration strings.
func expandEnvVars(cfg Config) Config {
	// Expand provider API keys and models
	for name, provider := range cfg.Providers {
		provider.APIKey = expandEnvString(provider.APIKey)
		provider.Model = expandEnvString(provider.Model)

		// Expand provider-specific HTTP overrides
		if provider.Timeout != nil {
			timeout := expandEnvString(*provider.Timeout)
			provider.Timeout = &timeout
		}
		if provider.InitialBackoff != nil {
			backoff := expandEnvString(*provider.InitialBackoff)
			provider.InitialBackoff = &backoff
		}
		if provider.MaxBackoff != nil {
			backoff := expandEnvString(*provider.MaxBackoff)
			provider.MaxBackoff = &backoff
		}

		cfg.Providers[name] = provider
	}

	// Expand HTTP config
	cfg.HTTP.Timeout = expandEnvString(cfg.HTTP.Timeout)
	cfg.HTTP.InitialBackoff = expandEnvString(cfg.HTTP.InitialBackoff)
	cfg.HTTP.MaxBackoff = expandEnvString(cfg.HTTP.MaxBackoff)

	// Expand merge config
	cfg.Merge.Provider = expandEnvString(cfg.Merge.Provider)
	cfg.Merge.Model = expandEnvString(cfg.Merge.Model)
	cfg.Merge.Strategy = expandEnvString(cfg.Merge.Strategy)

	// Expand git config
	cfg.Git.RepositoryDir = expandEnvString(cfg.Git.RepositoryDir)

	// Expand output config
	cfg.Output.Directory = expandEnvString(cfg.Output.Directory)

	// Expand budget config
	cfg.Budget.DegradationPolicy = expandEnvStringSlice(cfg.Budget.DegradationPolicy)

	// Expand redaction config
	cfg.Redaction.DenyGlobs = expandEnvStringSlice(cfg.Redaction.DenyGlobs)
	cfg.Redaction.AllowGlobs = expandEnvStringSlice(cfg.Redaction.AllowGlobs)

	// Expand store config
	cfg.Store.Path = expandEnvString(cfg.Store.Path)

	// Expand observability config
	cfg.Observability.Logging.Level = expandEnvString(cfg.Observability.Logging.Level)
	cfg.Observability.Logging.Format = expandEnvString(cfg.Observability.Logging.Format)

	return cfg
}

// expandEnvString replaces ${VAR} or $VAR with environment variable values.
func expandEnvString(s string) string {
	if s == "" {
		return s
	}

	// Replace ${VAR} syntax
	re := regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match // Keep original if not found
	})

	// Replace $VAR syntax (without braces)
	re = regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[1:] // Remove $
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match // Keep original if not found
	})

	return s
}

// expandEnvStringSlice expands environment variables in a slice of strings.
func expandEnvStringSlice(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = expandEnvString(s)
	}
	return result
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

	// HTTP defaults
	v.SetDefault("http.timeout", "60s")
	v.SetDefault("http.maxRetries", 5)
	v.SetDefault("http.initialBackoff", "2s")
	v.SetDefault("http.maxBackoff", "32s")
	v.SetDefault("http.backoffMultiplier", 2.0)

	// Determinism defaults (Phase 2)
	v.SetDefault("determinism.enabled", true)
	v.SetDefault("determinism.temperature", 0.0)
	v.SetDefault("determinism.useSeed", true)

	// Redaction defaults (Phase 2)
	v.SetDefault("redaction.enabled", true)

	// Merge defaults (Phase 2)
	v.SetDefault("merge.enabled", true)
	v.SetDefault("merge.strategy", "consensus")

	// Store defaults (Phase 3)
	v.SetDefault("store.enabled", true)
	v.SetDefault("store.path", defaultStorePath())

	// Observability defaults (Phase 3)
	v.SetDefault("observability.logging.enabled", true)
	v.SetDefault("observability.logging.level", "info")
	v.SetDefault("observability.logging.format", "human")
	v.SetDefault("observability.logging.redactAPIKeys", true)
	v.SetDefault("observability.metrics.enabled", true)

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

func defaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./reviews.db"
	}
	return filepath.Join(home, ".config", "cr", "reviews.db")
}
