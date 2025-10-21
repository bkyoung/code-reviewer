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
