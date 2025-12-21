// Package redaction contains security test cases for secret redaction.
//
// TEST 2.3: Edge Cases and Unusual Formats
// Expected: Mixed results - some caught, some missed.
// Purpose: Identify boundaries of current redaction system.
package redaction

import (
	"fmt"
	"log"
)

const (
	// Very long key (exceeds typical patterns)
	VeryLongKey = "sk-proj-abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz"

	// Very short key (might not match length requirements)
	ShortKey = "sk-abc"

	// Mixed case variations
	MixedCaseKey = "SK-PROJ-ABCDEF1234567890"

	// With special characters
	SpecialCharsKey = "sk-proj-abc_def-123.456"

	// Whitespace around secrets
	WhitespaceKey = "  sk-proj-whitespace123  "

	// Multiple secrets on one line
	MultipleSecrets = "first=sk-proj-first123 second=sk-proj-second456"

	// Comment-embedded secret
	// The key is: sk-proj-incomment12345

	// Secrets in different quote styles
	SingleQuoteKey  = "'sk-proj-singlequote123'"
	DoubleQuoteKey  = "\"sk-proj-doublequote123\""
	BacktickKey     = "`sk-proj-backtick123`"
	NoQuoteKey      = "sk-proj-noquote12345"

	// With URL context
	WebhookURL = "https://api.example.com/webhook?key=sk-proj-inurl123&other=param"

	// Database connection strings
	PostgresConn = "postgres://user:password123@localhost:5432/database?sslmode=disable"
	MongoConn    = "mongodb://admin:secretpass@cluster.mongodb.net:27017/db"
	MySQLConn    = "mysql://root:mysqlpassword@tcp(localhost:3306)/db"
	RedisConn    = "redis://:redispassword@localhost:6379/0"

	// Cloud provider specifics
	AzureConnStr = "DefaultEndpointsProtocol=https;AccountName=myaccount;AccountKey=abc123def456ghi789jkl012mno345pqr678stu901vwx234yz==;EndpointSuffix=core.windows.net"
	GCPServiceAccount = `{
		"type": "service_account",
		"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBA..."
	}`
)

// Multiline with secrets scattered
var MultilineConfig = `
line 1: no secrets here
line 2: api_key=sk-proj-multiline123
line 3: also clean
line 4: password=anothersecret456
line 5: nothing here
`

// Secrets in error messages (common leak vector)
func MightLeakInError() error {
	apiKey := "sk-proj-inerror12345"
	return fmt.Errorf("failed to connect with key: %s", apiKey)
}

// Logging secrets (common leak vector)
func MightLeakInLog() {
	token := "ghp_inlog1234567890abcdefgh"
	log.Printf("Using token: %s", token)
}
