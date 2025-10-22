package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandEnvString(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_API_KEY", "secret-key-123")
	os.Setenv("TEST_PATH", "/path/to/data")
	defer os.Unsetenv("TEST_API_KEY")
	defer os.Unsetenv("TEST_PATH")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "expand ${VAR} syntax",
			input:    "${TEST_API_KEY}",
			expected: "secret-key-123",
		},
		{
			name:     "expand $VAR syntax",
			input:    "$TEST_API_KEY",
			expected: "secret-key-123",
		},
		{
			name:     "expand in middle of string",
			input:    "key:${TEST_API_KEY}:end",
			expected: "key:secret-key-123:end",
		},
		{
			name:     "expand multiple variables",
			input:    "${TEST_API_KEY}:${TEST_PATH}",
			expected: "secret-key-123:/path/to/data",
		},
		{
			name:     "leave non-existent var unchanged",
			input:    "${NONEXISTENT_VAR}",
			expected: "${NONEXISTENT_VAR}",
		},
		{
			name:     "handle empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "handle string without variables",
			input:    "plain-text",
			expected: "plain-text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set test environment variables
	os.Setenv("OPENAI_API_KEY", "sk-test-123")
	os.Setenv("OUTPUT_DIR", "/custom/output")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("OUTPUT_DIR")

	cfg := Config{
		Providers: map[string]ProviderConfig{
			"openai": {
				Enabled: true,
				Model:   "gpt-4o-mini",
				APIKey:  "${OPENAI_API_KEY}",
			},
		},
		Output: OutputConfig{
			Directory: "${OUTPUT_DIR}",
		},
	}

	expanded := expandEnvVars(cfg)

	assert.Equal(t, "sk-test-123", expanded.Providers["openai"].APIKey)
	assert.Equal(t, "/custom/output", expanded.Output.Directory)
}

func TestExpandEnvStringSlice(t *testing.T) {
	// Set test environment variables
	os.Setenv("POLICY_1", "reduce-providers")
	os.Setenv("POLICY_2", "reduce-context")
	os.Setenv("PATTERN", "*.secret")
	defer os.Unsetenv("POLICY_1")
	defer os.Unsetenv("POLICY_2")
	defer os.Unsetenv("PATTERN")

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "expand single element",
			input:    []string{"${POLICY_1}"},
			expected: []string{"reduce-providers"},
		},
		{
			name:     "expand multiple elements",
			input:    []string{"${POLICY_1}", "${POLICY_2}"},
			expected: []string{"reduce-providers", "reduce-context"},
		},
		{
			name:     "expand mixed with plain text",
			input:    []string{"plain", "${PATTERN}", "another"},
			expected: []string{"plain", "*.secret", "another"},
		},
		{
			name:     "handle empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "handle nil slice",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvStringSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandEnvVars_MergeConfig(t *testing.T) {
	os.Setenv("MERGE_PROVIDER", "openai")
	os.Setenv("MERGE_MODEL", "gpt-4")
	os.Setenv("MERGE_STRATEGY", "consensus")
	defer os.Unsetenv("MERGE_PROVIDER")
	defer os.Unsetenv("MERGE_MODEL")
	defer os.Unsetenv("MERGE_STRATEGY")

	cfg := Config{
		Merge: MergeConfig{
			Provider: "${MERGE_PROVIDER}",
			Model:    "${MERGE_MODEL}",
			Strategy: "${MERGE_STRATEGY}",
		},
	}

	expanded := expandEnvVars(cfg)

	assert.Equal(t, "openai", expanded.Merge.Provider)
	assert.Equal(t, "gpt-4", expanded.Merge.Model)
	assert.Equal(t, "consensus", expanded.Merge.Strategy)
}

func TestExpandEnvVars_BudgetConfig(t *testing.T) {
	os.Setenv("POLICY_1", "reduce-providers")
	os.Setenv("POLICY_2", "reduce-context")
	defer os.Unsetenv("POLICY_1")
	defer os.Unsetenv("POLICY_2")

	cfg := Config{
		Budget: BudgetConfig{
			DegradationPolicy: []string{"${POLICY_1}", "${POLICY_2}"},
		},
	}

	expanded := expandEnvVars(cfg)

	assert.Equal(t, []string{"reduce-providers", "reduce-context"}, expanded.Budget.DegradationPolicy)
}

func TestExpandEnvVars_RedactionConfig(t *testing.T) {
	os.Setenv("DENY_PATTERN", "*.secret")
	os.Setenv("ALLOW_PATTERN", "public/*")
	defer os.Unsetenv("DENY_PATTERN")
	defer os.Unsetenv("ALLOW_PATTERN")

	cfg := Config{
		Redaction: RedactionConfig{
			DenyGlobs:  []string{"${DENY_PATTERN}", "private/*"},
			AllowGlobs: []string{"${ALLOW_PATTERN}"},
		},
	}

	expanded := expandEnvVars(cfg)

	assert.Equal(t, []string{"*.secret", "private/*"}, expanded.Redaction.DenyGlobs)
	assert.Equal(t, []string{"public/*"}, expanded.Redaction.AllowGlobs)
}

func TestExpandEnvVars_ObservabilityConfig(t *testing.T) {
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")
	defer os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("LOG_FORMAT")

	cfg := Config{
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{
				Level:  "${LOG_LEVEL}",
				Format: "${LOG_FORMAT}",
			},
		},
	}

	expanded := expandEnvVars(cfg)

	assert.Equal(t, "debug", expanded.Observability.Logging.Level)
	assert.Equal(t, "json", expanded.Observability.Logging.Format)
}

func TestExpandEnvVars_Comprehensive(t *testing.T) {
	// Set all test environment variables
	os.Setenv("OPENAI_KEY", "sk-123")
	os.Setenv("MERGE_PROVIDER", "anthropic")
	os.Setenv("POLICY", "reduce-providers")
	os.Setenv("DENY_GLOB", "*.key")
	os.Setenv("LOG_LEVEL", "error")
	os.Setenv("STORE_PATH", "/data/reviews.db")
	defer os.Unsetenv("OPENAI_KEY")
	defer os.Unsetenv("MERGE_PROVIDER")
	defer os.Unsetenv("POLICY")
	defer os.Unsetenv("DENY_GLOB")
	defer os.Unsetenv("LOG_LEVEL")
	defer os.Unsetenv("STORE_PATH")

	cfg := Config{
		Providers: map[string]ProviderConfig{
			"openai": {APIKey: "${OPENAI_KEY}"},
		},
		Merge: MergeConfig{
			Provider: "${MERGE_PROVIDER}",
		},
		Budget: BudgetConfig{
			DegradationPolicy: []string{"${POLICY}"},
		},
		Redaction: RedactionConfig{
			DenyGlobs: []string{"${DENY_GLOB}"},
		},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{
				Level: "${LOG_LEVEL}",
			},
		},
		Store: StoreConfig{
			Path: "${STORE_PATH}",
		},
	}

	expanded := expandEnvVars(cfg)

	// Verify all expansions
	assert.Equal(t, "sk-123", expanded.Providers["openai"].APIKey)
	assert.Equal(t, "anthropic", expanded.Merge.Provider)
	assert.Equal(t, []string{"reduce-providers"}, expanded.Budget.DegradationPolicy)
	assert.Equal(t, []string{"*.key"}, expanded.Redaction.DenyGlobs)
	assert.Equal(t, "error", expanded.Observability.Logging.Level)
	assert.Equal(t, "/data/reviews.db", expanded.Store.Path)
}

func TestHTTPConfigDefaults(t *testing.T) {
	cfg, err := Load(LoaderOptions{
		ConfigPaths: []string{"testdata"},
		FileName:    "nonexistent", // Should use defaults
	})
	assert.NoError(t, err)

	// Verify HTTP defaults
	assert.Equal(t, "60s", cfg.HTTP.Timeout)
	assert.Equal(t, 5, cfg.HTTP.MaxRetries)
	assert.Equal(t, "2s", cfg.HTTP.InitialBackoff)
	assert.Equal(t, "32s", cfg.HTTP.MaxBackoff)
	assert.Equal(t, 2.0, cfg.HTTP.BackoffMultiplier)
}

func TestExpandEnvVars_HTTPConfig(t *testing.T) {
	os.Setenv("HTTP_TIMEOUT", "120s")
	os.Setenv("HTTP_BACKOFF", "5s")
	defer os.Unsetenv("HTTP_TIMEOUT")
	defer os.Unsetenv("HTTP_BACKOFF")

	cfg := Config{
		HTTP: HTTPConfig{
			Timeout:        "${HTTP_TIMEOUT}",
			InitialBackoff: "${HTTP_BACKOFF}",
			MaxBackoff:     "30s", // Plain string
		},
	}

	expanded := expandEnvVars(cfg)

	assert.Equal(t, "120s", expanded.HTTP.Timeout)
	assert.Equal(t, "5s", expanded.HTTP.InitialBackoff)
	assert.Equal(t, "30s", expanded.HTTP.MaxBackoff)
}

func TestExpandEnvVars_ProviderHTTPOverrides(t *testing.T) {
	os.Setenv("OLLAMA_TIMEOUT", "180s")
	defer os.Unsetenv("OLLAMA_TIMEOUT")

	timeout := "${OLLAMA_TIMEOUT}"
	maxRetries := 3

	cfg := Config{
		Providers: map[string]ProviderConfig{
			"ollama": {
				Enabled:    true,
				Model:      "llama2",
				Timeout:    &timeout,
				MaxRetries: &maxRetries,
			},
		},
	}

	expanded := expandEnvVars(cfg)

	assert.NotNil(t, expanded.Providers["ollama"].Timeout)
	assert.Equal(t, "180s", *expanded.Providers["ollama"].Timeout)
	assert.NotNil(t, expanded.Providers["ollama"].MaxRetries)
	assert.Equal(t, 3, *expanded.Providers["ollama"].MaxRetries)
}

func TestExpandEnvString_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	assert.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "expand tilde at start",
			input:    "~/.config/cr/reviews.db",
			expected: home + "/.config/cr/reviews.db",
		},
		{
			name:     "expand tilde alone",
			input:    "~",
			expected: home,
		},
		{
			name:     "expand tilde with trailing slash",
			input:    "~/",
			expected: home + "/",
		},
		{
			name:     "do not expand tilde in middle",
			input:    "/path/~/file",
			expected: "/path/~/file", // Tilde only expands at start
		},
		{
			name:     "do not expand escaped tilde",
			input:    "\\~/.config",
			expected: "\\~/.config", // Escaped tilde stays literal
		},
		{
			name:     "expand tilde with env var",
			input:    "~/data/${TEST_VAR}",
			expected: home + "/data/${TEST_VAR}", // Both should work together
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvString(tt.input)
			assert.Equal(t, tt.expected, result, "input: %s", tt.input)
		})
	}
}

func TestExpandEnvVars_StorePathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	assert.NoError(t, err)

	cfg := Config{
		Store: StoreConfig{
			Enabled: true,
			Path:    "~/.config/cr/reviews.db",
		},
	}

	expanded := expandEnvVars(cfg)

	expected := home + "/.config/cr/reviews.db"
	assert.Equal(t, expected, expanded.Store.Path, "Tilde in store.path should be expanded to home directory")
}
