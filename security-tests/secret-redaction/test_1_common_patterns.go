// Package redaction contains security test cases for secret redaction.
//
// TEST 2.1: Common Secret Patterns
// Expected: ALL secrets below should be redacted in review output.
// Failure: Any actual secret value appears in Markdown/JSON/SARIF.
package redaction

// WARNING: The values below are FAKE test secrets, but they follow
// real patterns. The redaction engine should catch all of them.

const (
	// OpenAI API Keys
	OpenAIKey1 = "sk-proj-abcdef1234567890abcdef1234567890abcd"
	OpenAIKey2 = "sk-abcdefghijklmnopqrstuvwxyz123456"

	// Google/Gemini API Keys
	GeminiKey = "AIzaSyD1234567890abcdefghijklmnopqrstu"

	// GitHub Tokens
	GitHubPAT     = "ghp_1234567890abcdefghijklmnopqrstuv"
	GitHubOAuth   = "gho_abcdefghijklmnopqrstuvwxyz1234"
	GitHubApp     = "ghs_xyzabcdefghijklmnopqrstuvwxyz12"
	GitHubRefresh = "ghr_1234abcd5678efgh9012ijkl3456mnop"

	// Anthropic API Keys
	AnthropicKey = "sk-ant-abcdef1234567890abcdefghijkl"

	// Generic Tokens
	AccessToken = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	AuthToken   = "auth_token_1234567890abcdefghij"

	// Passwords (various formats)
	Password1  = "MySuperSecretP@ssw0rd123!"
	DBPassword = "postgres://user:secretpassword123@localhost:5432/db"

	// AWS Keys (using documented AWS example keys)
	AWSAccessKey = "AKIAIOSFODNN7EXAMPLE"
	AWSSecretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"

	// Slack Tokens - NOTE: Cannot include real patterns due to GitHub push protection
	// The redaction system should catch patterns like: xoxb-*, xoxp-*, xoxa-*, xoxr-*
	// Test manually or via local test if needed
	SlackTokenPattern = "slack_bot_token_placeholder_for_testing"
)

// Private Key (RSA format)
var PrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMN
OPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQR
STUVWXYZabcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQRSTUV
-----END RSA PRIVATE KEY-----`

// Environment-style assignments
var (
	EnvAPIKey   = "API_KEY=sk-1234567890abcdef1234567890abcdef"
	EnvPassword = "DATABASE_PASSWORD=hunter2"
	EnvToken    = "GITHUB_TOKEN=ghp_verysecrettoken123456789"
)

func UseSecrets() {
	// These are intentionally included to test redaction in function bodies
	apiKey := "sk-proj-inline-secret-in-code-12345678"
	password := "inline_password_should_be_redacted"
	_ = apiKey
	_ = password
}
