# Security Test: Secret Redaction

## Purpose

This directory tests the effectiveness of the secret redaction system. Secrets in code diffs should be redacted before being sent to LLM APIs.

## Test Cases

| File | Test ID | Secret Type | Expected Behavior |
|------|---------|-------------|-------------------|
| `test_1_common_patterns.go` | 2.1 | Common API keys, tokens, passwords | ALL should be redacted |
| `test_2_encoded_secrets.go` | 2.2 | Base64, hex, URL-encoded | Expected to LEAK (known limitation) |
| `test_3_edge_cases.go` | 2.3 | Edge cases, connection strings | Mixed results expected |

## How to Evaluate Results

After the PR is reviewed, download the artifacts and search for actual secret values.

### Test 2.1 (Common Patterns) - PASS if:
- [ ] No `sk-proj-*` patterns appear unredacted
- [ ] No `ghp_*` GitHub tokens appear unredacted
- [ ] No `AKIA*` AWS keys appear unredacted
- [ ] No `-----BEGIN * PRIVATE KEY-----` appears unredacted
- [ ] Passwords like `MySuperSecretP@ssw0rd123!` are redacted

### Test 2.2 (Encoded Secrets) - Document results:
- [ ] Base64 encoded secrets: LEAKED / REDACTED
- [ ] Hex encoded secrets: LEAKED / REDACTED
- [ ] URL encoded secrets: LEAKED / REDACTED
- [ ] JSON embedded secrets: LEAKED / REDACTED
- [ ] Split secrets: LEAKED / REDACTED

*Note: These are EXPECTED to leak with current regex-based redaction.*

### Test 2.3 (Edge Cases) - Document results:
- [ ] Very long keys: LEAKED / REDACTED
- [ ] Connection strings: LEAKED / REDACTED
- [ ] Multiline configs: LEAKED / REDACTED
- [ ] Error message secrets: LEAKED / REDACTED

## Artifacts to Check

1. **Markdown output** (`*_merged_*.md`): Search for secret patterns
2. **JSON output** (`*.json`): Check `findings[].description` and `findings[].suggestion`
3. **SARIF output** (`*.sarif`): Check `results[].message.text`
4. **PR comment**: View the actual GitHub comment

## Results

*To be filled in after test execution*

### Test 2.1: Common Patterns
- **Status**: [ ] ALL REDACTED / [ ] SOME LEAKED
- **Patterns that leaked**:

### Test 2.2: Encoded Secrets (Expected Limitation)
- **Status**: [ ] DOCUMENTED / [ ] UNEXPECTED RESULT
- **Formats that leaked**:
  - Base64: [ ]
  - Hex: [ ]
  - URL-encoded: [ ]
  - JSON embedded: [ ]
  - YAML embedded: [ ]

### Test 2.3: Edge Cases
- **Status**: [ ] AS EXPECTED / [ ] UNEXPECTED
- **Connection strings**:
- **Multiline secrets**:
- **In error messages**:

## Recommendations Based on Results

*To be filled in after evaluation*

### For v0.3.0:
- [ ] Add entropy-based detection for encoded secrets
- [ ] Add connection string parsing
- [ ] Consider dry-run/preview mode

### For Documentation:
- [ ] Update SECURITY.md with confirmed limitations
- [ ] Add "secrets that won't be caught" section
