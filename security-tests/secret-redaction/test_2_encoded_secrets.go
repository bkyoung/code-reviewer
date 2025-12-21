// Package redaction contains security test cases for secret redaction.
//
// TEST 2.2: Encoded Secrets
// Expected: These WILL LIKELY LEAK (known limitation).
// Purpose: Document what encoding formats bypass redaction.
package redaction

// WARNING: The following secrets use various encoding schemes.
// Current regex-based redaction is NOT expected to catch these.
// This test documents the gap for future enhancement (entropy-based detection).

const (
	// Base64 encoded "sk-proj-abcdef1234567890"
	Base64Secret = "c2stcHJvai1hYmNkZWYxMjM0NTY3ODkw"

	// Hex encoded "sk-proj-abcdef1234567890"
	HexSecret = "736b2d70726f6a2d6162636465663132333435363738393"

	// URL encoded API key
	URLEncodedSecret = "api_key%3Dsk-proj-abcdef1234567890"

	// ROT13 encoded (simple obfuscation)
	ROT13Secret = "fx-cebw-nopqrs1234567890" // sk-proj-abcdef1234567890

	// Split across multiple strings (evasion technique)
	SecretPart1 = "sk-proj-"
	SecretPart2 = "abcdef"
	SecretPart3 = "1234567890"

	// Unicode escape sequences
	UnicodeSecret = "\u0073\u006b\u002d\u0070\u0072\u006f\u006a" // "sk-proj"

	// Reversed string
	ReversedSecret = "0987654321fedcba-jorp-ks"
)

// JSON embedded secrets
var JSONConfig = `{
	"apiKey": "sk-proj-embedded-in-json-12345",
	"nested": {
		"auth": {
			"token": "ghp_nestedtoken1234567890abcd"
		}
	}
}`

// YAML embedded secrets (as a string)
var YAMLConfig = `
database:
  password: secretpassword123
api:
  key: sk-proj-yaml-embedded-secret
`

// Environment file content
var EnvFile = `
# Database configuration
DB_HOST=localhost
DB_PASSWORD=verysecretpassword
API_KEY=sk-proj-envfile-secret-12345
`

func ConcatenatedSecret() string {
	// Building secret at runtime - cannot be statically detected
	return SecretPart1 + SecretPart2 + SecretPart3
}
