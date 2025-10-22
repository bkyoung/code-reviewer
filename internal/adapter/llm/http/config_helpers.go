package http

import (
	"time"

	"github.com/bkyoung/code-reviewer/internal/config"
)

// ParseTimeout parses timeout with fallback chain: provider override > global > default
func ParseTimeout(providerOverride *string, globalTimeout string, defaultVal time.Duration) time.Duration {
	// Provider override takes precedence
	if providerOverride != nil && *providerOverride != "" {
		if d, err := time.ParseDuration(*providerOverride); err == nil {
			return d
		}
	}

	// Try global config
	if globalTimeout != "" {
		if d, err := time.ParseDuration(globalTimeout); err == nil {
			return d
		}
	}

	// Use default
	return defaultVal
}

// BuildRetryConfig creates RetryConfig from provider + global HTTP config
func BuildRetryConfig(provider config.ProviderConfig, httpCfg config.HTTPConfig) RetryConfig {
	// Max retries: provider override > global
	maxRetries := httpCfg.MaxRetries
	if provider.MaxRetries != nil {
		maxRetries = *provider.MaxRetries
	}

	// Initial backoff: provider override > global > default
	initialBackoff := parseDuration(provider.InitialBackoff, httpCfg.InitialBackoff, 2*time.Second)

	// Max backoff: provider override > global > default
	maxBackoff := parseDuration(provider.MaxBackoff, httpCfg.MaxBackoff, 32*time.Second)

	return RetryConfig{
		MaxRetries:     maxRetries,
		InitialBackoff: initialBackoff,
		MaxBackoff:     maxBackoff,
		Multiplier:     httpCfg.BackoffMultiplier,
	}
}

// parseDuration parses duration with fallback chain
func parseDuration(override *string, global string, defaultVal time.Duration) time.Duration {
	if override != nil && *override != "" {
		if d, err := time.ParseDuration(*override); err == nil {
			return d
		}
	}

	if global != "" {
		if d, err := time.ParseDuration(global); err == nil {
			return d
		}
	}

	return defaultVal
}
