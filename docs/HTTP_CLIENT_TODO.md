# HTTP API Client Implementation Checklist

Status: Week 1 Complete - OpenAI Implemented
Started: 2025-10-21
Updated: 2025-10-21

## Goal
Replace stub/static LLM clients with real HTTP implementations for OpenAI, Anthropic, Gemini, and Ollama, enabling actual code review functionality with production LLM APIs.

## Overview

Currently all providers use static/stub clients:
- OpenAI: Uses `openai.NewStaticClient()` returning canned responses
- Anthropic: Uses `nil` client
- Gemini: Uses `nil` client
- Ollama: Uses `nil` client

This batch implements production HTTP clients with proper error handling, retries, and rate limiting.

## Priority Order

1. **OpenAI** (most common, well-documented API)
2. **Anthropic** (Claude - popular for code review)
3. **Ollama** (local, no API key required)
4. **Gemini** (Google)

## Week 1: OpenAI HTTP Client ✅ COMPLETE

### 1.1 HTTP Client Infrastructure (TDD) ✅
- [x] Create `internal/adapter/llm/openai/client.go`
- [x] Define HTTPClient interface for testing
- [x] Write tests for NewHTTPClient with API key
- [x] Implement NewHTTPClient constructor
- [x] Write tests for API key validation
- [x] Implement API key validation
- [x] Add timeout configuration (default 60s)

### 1.2 Request/Response Types (TDD) ✅
- [x] Define ChatCompletionRequest struct (matches OpenAI API)
- [x] Define ChatCompletionResponse struct
- [x] Define Message struct (role, content)
- [x] Write tests for JSON marshaling/unmarshaling
- [x] Add validation for required fields
- [x] Test error response handling

### 1.3 HTTP Implementation (TDD) ✅
- [x] Write test for successful API call
- [x] Implement Review() method calling OpenAI Chat Completion API
- [x] Write tests for error responses (401, 429, 500, 503)
- [x] Implement error handling with typed errors
- [x] Write tests for timeout scenarios
- [x] Implement timeout handling
- [x] Write tests for malformed responses
- [x] Implement response validation

### 1.4 Retry Logic (TDD) ✅
- [x] Write tests for 429 rate limit with exponential backoff
- [x] Implement exponential backoff (2s, 4s, 8s, 16s, 32s)
- [x] Write tests for 503 service unavailable retry
- [x] Implement 503 retry logic (max 3 retries)
- [x] Write tests for non-retryable errors (400, 401, 403)
- [x] Implement retry decision logic
- [x] Add configurable max retries

### 1.5 Response Parsing (TDD) ✅
- [x] Write tests for parsing review from completion text
- [x] Implement JSON extraction from markdown code blocks
- [x] Write tests for handling partial/malformed JSON
- [x] Implement graceful degradation (return text summary)
- [x] Write tests for finding extraction
- [x] Implement finding parsing with validation
- [x] Test with various response formats

### 1.6 Integration ✅
- [x] Update main.go to use HTTPClient instead of StaticClient
- [x] Add API key loading from config/env
- [x] Write integration test with mock HTTP server
- [ ] Test with real OpenAI API (manual - requires user API key)
- [x] Update configuration documentation
- [x] Add example .env file

## Week 2: Anthropic (Claude) HTTP Client

### 2.1 Anthropic API Client (TDD)
- [ ] Create `internal/adapter/llm/anthropic/client.go`
- [ ] Define MessagesRequest struct (Anthropic API format)
- [ ] Define MessagesResponse struct
- [ ] Write tests for API authentication (x-api-key header)
- [ ] Implement NewHTTPClient with proper headers
- [ ] Write tests for successful message creation
- [ ] Implement Review() calling Messages API

### 2.2 Anthropic-Specific Features (TDD)
- [ ] Write tests for system prompt vs user message
- [ ] Implement system prompt handling
- [ ] Write tests for streaming disabled
- [ ] Ensure non-streaming responses only
- [ ] Write tests for content block handling
- [ ] Implement content block extraction
- [ ] Test with claude-3-5-sonnet and claude-3-5-haiku models

### 2.3 Error Handling (TDD)
- [ ] Write tests for Anthropic error response format
- [ ] Implement Anthropic error parsing
- [ ] Write tests for rate limit handling (429)
- [ ] Implement rate limit retry logic
- [ ] Write tests for overloaded_error (529)
- [ ] Implement overloaded retry logic
- [ ] Test content policy violations (400)

### 2.4 Response Parsing (TDD)
- [ ] Write tests for text content extraction
- [ ] Implement content[0].text parsing
- [ ] Write tests for handling multiple content blocks
- [ ] Implement multi-block concatenation
- [ ] Write tests for JSON extraction from responses
- [ ] Reuse OpenAI JSON parsing logic
- [ ] Test finding extraction

### 2.5 Integration
- [ ] Update main.go to create real Anthropic client
- [ ] Add API key loading (ANTHROPIC_API_KEY)
- [ ] Write integration test with mock server
- [ ] Test with real Anthropic API (manual)
- [ ] Update docs with Claude-specific configuration
- [ ] Add rate limit guidance

## Week 3: Ollama & Gemini Clients

### 3.1 Ollama HTTP Client (TDD)
- [ ] Create `internal/adapter/llm/ollama/client.go`
- [ ] Define GenerateRequest struct (Ollama format)
- [ ] Define GenerateResponse struct
- [ ] Write tests for local connection (http://localhost:11434)
- [ ] Implement NewHTTPClient with localhost default
- [ ] Write tests for /api/generate endpoint
- [ ] Implement Review() calling generate API
- [ ] Write tests for connection refused error
- [ ] Implement friendly "Ollama not running" error message

### 3.2 Ollama Features (TDD)
- [ ] Write tests for model availability check
- [ ] Implement /api/tags endpoint call (list models)
- [ ] Write tests for streaming disabled
- [ ] Ensure stream: false in request
- [ ] Write tests for context handling
- [ ] Implement system prompt in context
- [ ] Test with codellama and llama2 models

### 3.3 Gemini HTTP Client (TDD)
- [ ] Create `internal/adapter/llm/gemini/client.go`
- [ ] Define GenerateContentRequest struct (Gemini format)
- [ ] Define GenerateContentResponse struct
- [ ] Write tests for API key in URL params
- [ ] Implement NewHTTPClient with URL construction
- [ ] Write tests for generateContent endpoint
- [ ] Implement Review() calling Gemini API
- [ ] Write tests for parts[] content handling

### 3.4 Gemini Features (TDD)
- [ ] Write tests for safety settings
- [ ] Implement safety settings for code review context
- [ ] Write tests for generation config
- [ ] Implement temperature and candidate count settings
- [ ] Write tests for content filtering responses
- [ ] Handle SAFETY and RECITATION blocks
- [ ] Test with gemini-pro and gemini-1.5-pro models

### 3.5 Integration
- [ ] Update main.go for Ollama with localhost URL
- [ ] Update main.go for Gemini with API key
- [ ] Add OLLAMA_HOST env var support
- [ ] Add GEMINI_API_KEY env var support
- [ ] Write integration tests for both
- [ ] Test with real Ollama (manual)
- [ ] Test with real Gemini API (manual)
- [ ] Update documentation

## Week 4: Polish & Production Readiness

### 4.1 Shared HTTP Infrastructure
- [ ] Create `internal/adapter/llm/http/` package
- [ ] Extract common retry logic
- [ ] Extract common error handling
- [ ] Extract common JSON parsing
- [ ] Write tests for shared utilities
- [ ] Refactor all clients to use shared code
- [ ] Reduce code duplication

### 4.2 Observability (TDD)
- [ ] Write tests for request logging (debug mode)
- [ ] Implement structured logging for all requests
- [ ] Write tests for response logging
- [ ] Implement response logging (with PII redaction)
- [ ] Write tests for duration tracking
- [ ] Add request timing metrics
- [ ] Write tests for token usage tracking
- [ ] Implement token counting for cost estimation

### 4.3 Configuration
- [ ] Add http.timeout config option
- [ ] Add http.maxRetries config option
- [ ] Add http.retryBackoff config option
- [ ] Add provider-specific timeout overrides
- [ ] Write tests for config loading
- [ ] Add validation for config values
- [ ] Document all HTTP-related config options

### 4.4 Error Handling & Resilience
- [ ] Create typed error hierarchy (ErrRateLimit, ErrTimeout, etc.)
- [ ] Write tests for circuit breaker pattern
- [ ] Implement circuit breaker (optional, configurable)
- [ ] Write tests for request cancellation via context
- [ ] Ensure proper context propagation
- [ ] Write tests for graceful shutdown
- [ ] Implement cleanup in Close() methods

### 4.5 Testing Infrastructure
- [ ] Create mock HTTP server for tests (`internal/testutil/mockllm/`)
- [ ] Implement OpenAI mock endpoints
- [ ] Implement Anthropic mock endpoints
- [ ] Implement Ollama mock endpoints
- [ ] Implement Gemini mock endpoints
- [ ] Write integration tests using mock server
- [ ] Add test utilities for common scenarios

### 4.6 Security
- [ ] Write tests for API key redaction in logs
- [ ] Implement API key masking (show only last 4 chars)
- [ ] Write tests for HTTPS enforcement
- [ ] Ensure all clients use HTTPS (except Ollama localhost)
- [ ] Write tests for TLS verification
- [ ] Implement TLS certificate validation
- [ ] Add security best practices documentation

### 4.7 Documentation
- [ ] Update ARCHITECTURE.md with HTTP client layer
- [ ] Create HTTP_CLIENT_DESIGN.md with API details
- [ ] Document all supported models per provider
- [ ] Add troubleshooting guide (API key issues, timeouts, rate limits)
- [ ] Create examples for each provider
- [ ] Document cost estimation formulas
- [ ] Add rate limit guidance by provider

### 4.8 Cost Tracking (Preparation for Phase 4)
- [ ] Write tests for token counting by provider
- [ ] Implement token estimation (input + output)
- [ ] Write tests for cost calculation
- [ ] Implement cost calculation per provider pricing
- [ ] Add cost tracking to Review response
- [ ] Write tests for budget tracking (preparation)
- [ ] Document pricing as of implementation date

## Dependencies to Add

```bash
# No new dependencies needed - using stdlib net/http
# All providers use REST APIs compatible with net/http
```

## Testing Commands

```bash
# Unit tests (with mocks)
go test ./internal/adapter/llm/openai/...
go test ./internal/adapter/llm/anthropic/...
go test ./internal/adapter/llm/ollama/...
go test ./internal/adapter/llm/gemini/...

# Integration tests (with mock HTTP server)
go test -tags=integration ./internal/adapter/llm/...

# Manual tests (requires API keys and real services)
# OpenAI
OPENAI_API_KEY=sk-... ./cr review branch main --target HEAD

# Anthropic
ANTHROPIC_API_KEY=sk-ant-... ./cr review branch main --target HEAD

# Ollama (requires Ollama running locally)
ollama serve &
./cr review branch main --target HEAD

# Gemini
GEMINI_API_KEY=... ./cr review branch main --target HEAD
```

## Completion Criteria

- [ ] All unit tests passing
- [ ] All integration tests passing
- [ ] Manual tests successful with real APIs
- [ ] Code coverage >80% for HTTP client packages
- [ ] Error handling comprehensive (timeouts, rate limits, auth failures)
- [ ] Retry logic tested and working
- [ ] Response parsing handles all known formats
- [ ] API keys loaded from config and environment
- [ ] Documentation complete
- [ ] Security review passed (no API keys in logs)

## Success Metrics

- **Reliability**: <1% failure rate for transient errors (should retry successfully)
- **Performance**: 95th percentile response time <30 seconds for typical review
- **Error Handling**: All error types have clear, actionable messages
- **Cost Tracking**: Accurate token counting within 5% of provider billing
- **Documentation**: Each provider has working example in docs

## Notes

- Start with OpenAI (most mature API, best docs)
- Test thoroughly with mock servers before hitting real APIs
- Implement rate limit handling from day 1 (429 responses are common)
- Use context.Context throughout for cancellation support
- Keep API-specific code isolated to each provider package
- Extract common logic to shared http utilities package
- Never log full API keys (mask to last 4 chars only)
- Consider implementing request/response recording for debugging
- Add instrumentation for observability (duration, tokens, cost)

## Risk Mitigation

**API Changes**: Version lock known-good API versions in docs
**Rate Limits**: Implement exponential backoff and retry logic
**Costs**: Add token counting and cost estimation (Phase 4 budget enforcement)
**Security**: Never log credentials, use environment variables
**Reliability**: Circuit breaker pattern for repeated failures
**Testing**: Comprehensive mock server for CI without API keys
