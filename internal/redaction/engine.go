package redaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Engine performs regex-based secret detection and redaction.
type Engine struct {
	patterns []*regexp.Regexp
}

// NewEngine creates a new redaction engine with default secret patterns.
func NewEngine() *Engine {
	return &Engine{
		patterns: defaultPatterns(),
	}
}

// Redact scans input for secrets and replaces them with stable placeholders.
func (e *Engine) Redact(input string) (string, error) {
	result := input
	seenSecrets := make(map[string]string) // secret -> placeholder

	for _, pattern := range e.patterns {
		matches := pattern.FindAllString(result, -1)
		for _, match := range matches {
			// Skip if already processed
			if _, seen := seenSecrets[match]; seen {
				continue
			}

			// Generate stable placeholder based on secret hash
			placeholder := e.generatePlaceholder(match)
			seenSecrets[match] = placeholder
		}
	}

	// Replace all secrets with their placeholders
	for secret, placeholder := range seenSecrets {
		result = strings.ReplaceAll(result, secret, placeholder)
	}

	return result, nil
}

// IsRedacted checks if the content contains redaction placeholders.
func (e *Engine) IsRedacted(content string) bool {
	return strings.Contains(content, "<REDACTED:")
}

// generatePlaceholder creates a stable, unique placeholder for a secret.
func (e *Engine) generatePlaceholder(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	hashStr := hex.EncodeToString(hash[:])[:8]
	return fmt.Sprintf("<REDACTED:%s>", hashStr)
}

// defaultPatterns returns the default set of regex patterns for secret detection.
func defaultPatterns() []*regexp.Regexp {
	patterns := []string{
		// OpenAI API keys (flexible length for testing and real keys)
		`sk-[a-zA-Z0-9]{20,}`,
		// Anthropic API keys
		`sk-ant-[a-zA-Z0-9\-]{20,}`,
		// AWS Access Key ID
		`AKIA[0-9A-Z]{16}`,
		// AWS Secret Access Key (generalized high-entropy pattern)
		`aws.{0,20}?['\"][0-9a-zA-Z/+]{40}['\"]`,
		// GitHub tokens
		`gh[posr]_[a-zA-Z0-9]{20,}`,
		// Google API keys
		`AIza[0-9A-Za-z\-_]{35}`,
		// JWT tokens (basic pattern)
		`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`,
		// Private keys (PEM format)
		`-----BEGIN\s+(?:RSA|EC|OPENSSH|DSA|ENCRYPTED)\s+PRIVATE\s+KEY-----[\s\S]*?-----END\s+(?:RSA|EC|OPENSSH|DSA|ENCRYPTED)\s+PRIVATE\s+KEY-----`,
		// Slack tokens
		`xox[baprs]-[a-zA-Z0-9\-]{10,}`,
		// Generic bearer tokens (after "Bearer " keyword)
		`Bearer\s+[a-zA-Z0-9_\-\.]+`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		compiled = append(compiled, re)
	}

	return compiled
}
