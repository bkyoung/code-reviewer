# HTTP API Client Technical Design

Version: 1.0
Date: 2025-10-21
Status: Design Phase

## 1. Overview

This document describes the technical design for implementing HTTP API clients for OpenAI, Anthropic, Gemini, and Ollama LLM providers. These clients replace the current stub implementations to enable actual code review functionality.

### 1.1 Goals

- Enable real LLM API integration for code reviews
- Provide robust error handling and retry logic
- Support multiple providers with consistent interface
- Track costs and token usage
- Maintain testability with mock servers
- Follow security best practices

### 1.2 Non-Goals

- Streaming API support (all responses are non-streaming)
- Custom fine-tuned model management
- Provider-specific advanced features (embeddings, assistants, etc.)
- Real-time chat/conversation threading

## 2. Architecture

### 2.1 Component Structure

```
internal/adapter/llm/
├── http/                      # Shared HTTP utilities
│   ├── client.go             # Base HTTP client with retry logic
│   ├── errors.go             # Typed error hierarchy
│   ├── retry.go              # Exponential backoff implementation
│   └── json_parser.go        # Common JSON extraction utilities
├── openai/
│   ├── client.go             # OpenAI HTTP client
│   ├── types.go              # OpenAI API types
│   ├── provider.go           # Provider interface implementation
│   └── client_test.go        # Unit tests
├── anthropic/
│   ├── client.go             # Anthropic HTTP client
│   ├── types.go              # Anthropic API types
│   ├── provider.go           # Provider interface implementation
│   └── client_test.go        # Unit tests
├── ollama/
│   ├── client.go             # Ollama HTTP client
│   ├── types.go              # Ollama API types
│   ├── provider.go           # Provider interface implementation
│   └── client_test.go        # Unit tests
└── gemini/
    ├── client.go             # Gemini HTTP client
    ├── types.go              # Gemini API types
    ├── provider.go           # Provider interface implementation
    └── client_test.go        # Unit tests

internal/testutil/
└── mockllm/                   # Mock HTTP servers for testing
    ├── openai.go
    ├── anthropic.go
    ├── ollama.go
    └── gemini.go
```

### 2.2 Interface Compliance

All HTTP clients must implement the existing `review.Provider` interface:

```go
type Provider interface {
    Review(ctx context.Context, req ProviderRequest) (domain.Review, error)
}
```

### 2.3 Client Interface

Each provider client implements a common pattern:

```go
// HTTPClient defines the contract for LLM HTTP clients
type HTTPClient interface {
    // Call makes an HTTP request to the LLM API
    Call(ctx context.Context, prompt string, options CallOptions) (*Response, error)

    // Close cleans up resources
    Close() error
}

type CallOptions struct {
    Model       string
    Temperature float64
    Seed        uint64
    MaxTokens   int
    Timeout     time.Duration
}

type Response struct {
    Text      string
    TokensIn  int
    TokensOut int
    Model     string
    FinishReason string
}
```

## 3. Provider-Specific Designs

### 3.1 OpenAI (Chat Completion API)

**Endpoint**: `POST https://api.openai.com/v1/chat/completions`

**Authentication**: Bearer token in `Authorization` header

**Request Format**:
```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "system",
      "content": "You are a code reviewer..."
    },
    {
      "role": "user",
      "content": "<diff content>"
    }
  ],
  "temperature": 0.0,
  "seed": 12345,
  "max_tokens": 16384,
  "response_format": { "type": "json_object" }
}
```

**Response Format**:
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "{ \"summary\": \"...\", \"findings\": [...] }"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 1234,
    "completion_tokens": 567,
    "total_tokens": 1801
  }
}
```

**Error Responses**:
- `401`: Invalid API key → `ErrAuthenticationFailed`
- `429`: Rate limit exceeded → `ErrRateLimitExceeded` (retry with backoff)
- `500`, `503`: Server error → `ErrServiceUnavailable` (retry)
- `400`: Invalid request → `ErrInvalidRequest` (do not retry)

**Retry Strategy**:
- 429: Exponential backoff (2s, 4s, 8s, 16s, 32s)
- 500/503: 3 retries with 2s, 4s, 8s delays
- Check `Retry-After` header if present

### 3.2 Anthropic (Messages API)

**Endpoint**: `POST https://api.anthropic.com/v1/messages`

**Authentication**: `x-api-key` header

**Required Headers**:
- `x-api-key: $ANTHROPIC_API_KEY`
- `anthropic-version: 2023-06-01`
- `Content-Type: application/json`

**Request Format**:
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 16384,
  "temperature": 0.0,
  "system": "You are a code reviewer...",
  "messages": [
    {
      "role": "user",
      "content": "<diff content>"
    }
  ]
}
```

**Response Format**:
```json
{
  "id": "msg_...",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "{ \"summary\": \"...\", \"findings\": [...] }"
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 1234,
    "output_tokens": 567
  }
}
```

**Error Responses**:
- `401`: Invalid API key → `ErrAuthenticationFailed`
- `429`: Rate limit exceeded → `ErrRateLimitExceeded` (retry)
- `529`: Overloaded → `ErrServiceUnavailable` (retry with longer backoff)
- `400`: Invalid request → `ErrInvalidRequest` (do not retry)

**Retry Strategy**:
- 429: Exponential backoff (5s, 10s, 20s, 40s, 80s) - Anthropic has stricter limits
- 529: 3 retries with 10s, 20s, 40s delays

### 3.3 Ollama (Generate API)

**Endpoint**: `POST http://localhost:11434/api/generate`

**Authentication**: None (local service)

**Request Format**:
```json
{
  "model": "codellama",
  "prompt": "You are a code reviewer...\n\n<diff content>",
  "stream": false,
  "options": {
    "temperature": 0.0,
    "seed": 12345,
    "num_predict": 16384
  }
}
```

**Response Format**:
```json
{
  "model": "codellama",
  "created_at": "2023-08-04T08:52:19.385406455-07:00",
  "response": "{ \"summary\": \"...\", \"findings\": [...] }",
  "done": true,
  "context": [1, 2, 3, ...],
  "total_duration": 5589157167,
  "load_duration": 3013701500,
  "prompt_eval_count": 46,
  "eval_count": 113
}
```

**Error Responses**:
- Connection refused → `ErrServiceUnavailable` with helpful message "Ollama not running. Start with: ollama serve"
- `404`: Model not found → `ErrModelNotFound` with list of available models
- `500`: Server error → `ErrServiceUnavailable` (retry)

**Retry Strategy**:
- Connection refused: No retry (user must start Ollama)
- 500: 3 retries with 1s, 2s, 4s delays

**Model Check**:
- Before first request, call `GET /api/tags` to list available models
- If model not found, provide helpful error with available models

### 3.4 Gemini (GenerateContent API)

**Endpoint**: `POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={apiKey}`

**Authentication**: API key in URL query parameter

**Request Format**:
```json
{
  "contents": [
    {
      "parts": [
        {
          "text": "You are a code reviewer...\n\n<diff content>"
        }
      ]
    }
  ],
  "generationConfig": {
    "temperature": 0.0,
    "candidateCount": 1,
    "maxOutputTokens": 16384
  },
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_DANGEROUS_CONTENT",
      "threshold": "BLOCK_ONLY_HIGH"
    }
  ]
}
```

**Response Format**:
```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "{ \"summary\": \"...\", \"findings\": [...] }"
          }
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "safetyRatings": [...]
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 1234,
    "candidatesTokenCount": 567,
    "totalTokenCount": 1801
  }
}
```

**Error Responses**:
- `400` with `INVALID_API_KEY` → `ErrAuthenticationFailed`
- `429`: Quota exceeded → `ErrRateLimitExceeded` (retry)
- `500`, `503`: Server error → `ErrServiceUnavailable` (retry)
- Candidate blocked by safety → `ErrContentFiltered` (do not retry)

**Retry Strategy**:
- 429: Exponential backoff (2s, 4s, 8s, 16s, 32s)
- 500/503: 3 retries with 2s, 4s, 8s delays

## 4. Common Components

### 4.1 Error Hierarchy

```go
// Base error type
type Error struct {
    Type       ErrorType
    Message    string
    StatusCode int
    Retryable  bool
    Provider   string
}

type ErrorType int

const (
    ErrTypeAuthentication ErrorType = iota
    ErrTypeRateLimit
    ErrTypeServiceUnavailable
    ErrTypeInvalidRequest
    ErrTypeTimeout
    ErrTypeModelNotFound
    ErrTypeContentFiltered
    ErrTypeUnknown
)
```

### 4.2 Retry Logic

```go
type RetryConfig struct {
    MaxRetries     int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    Multiplier     float64
}

func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxRetries:     5,
        InitialBackoff: 2 * time.Second,
        MaxBackoff:     32 * time.Second,
        Multiplier:     2.0,
    }
}

// ExponentialBackoff calculates wait time with jitter
func ExponentialBackoff(attempt int, config RetryConfig) time.Duration {
    backoff := config.InitialBackoff * time.Duration(math.Pow(config.Multiplier, float64(attempt)))
    if backoff > config.MaxBackoff {
        backoff = config.MaxBackoff
    }

    // Add jitter (±25%)
    jitter := time.Duration(rand.Float64() * 0.5 * float64(backoff))
    return backoff + jitter - time.Duration(0.25*float64(backoff))
}
```

### 4.3 JSON Parsing

All providers should return JSON in a consistent format:

```json
{
  "summary": "Overall code review summary",
  "findings": [
    {
      "file": "path/to/file.go",
      "lineStart": 10,
      "lineEnd": 15,
      "severity": "high",
      "category": "security",
      "description": "SQL injection vulnerability found",
      "suggestion": "Use parameterized queries",
      "evidence": true
    }
  ]
}
```

**Parsing Strategy**:
1. Extract text content from provider-specific response
2. Look for JSON in markdown code blocks (```json ... ```)
3. If no code blocks, try parsing entire response as JSON
4. If parsing fails, return text summary with no findings
5. Validate all findings have required fields

### 4.4 Request Logging

```go
type RequestLog struct {
    Timestamp    time.Time
    Provider     string
    Model        string
    PromptTokens int
    Duration     time.Duration
    StatusCode   int
    Error        string
}

// Log format (structured JSON for production)
{
  "level": "info",
  "timestamp": "2025-10-21T10:30:45Z",
  "provider": "openai",
  "model": "gpt-4o-mini",
  "prompt_tokens": 1234,
  "completion_tokens": 567,
  "duration_ms": 3456,
  "status_code": 200,
  "api_key": "sk-...xyz" // Only last 3 chars
}
```

**PII Redaction**:
- API keys: Show only last 4 characters
- Diff content: Never log full content (log token count only)
- Responses: Log summary statistics, not full text

## 5. Security

### 5.1 API Key Management

**Loading Priority**:
1. Environment variable (e.g., `OPENAI_API_KEY`)
2. Config file `providers.openai.apiKey`
3. Error if not found

**Storage**:
- Never commit API keys to version control
- Never log full API keys
- Use OS keychain for persistent storage (future enhancement)

**Validation**:
- Check API key format matches provider pattern
- Test with a lightweight API call on first use
- Cache validation result for session

### 5.2 HTTPS Enforcement

- All providers except Ollama must use HTTPS
- Ollama localhost can use HTTP
- Verify TLS certificates (no `InsecureSkipVerify`)
- Support custom CA certificates via environment

### 5.3 Request Sanitization

- Redact secrets from diffs before sending (use Redaction Engine)
- Never send raw environment variables or credentials
- Truncate excessively large diffs (>100k tokens)

## 6. Cost Tracking

### 6.1 Token Counting

**Input Tokens**:
- Count system prompt + user message
- Use provider-specific tokenizer if available
- Fall back to character count / 4 for estimation

**Output Tokens**:
- Extract from provider usage metadata
- Track actual tokens, not estimates

### 6.2 Cost Calculation

**Pricing (as of 2025-10-21)**:
```go
var ProviderPricing = map[string]Pricing{
    "gpt-4o-mini": {
        InputPer1M:  0.15,  // $0.15 per 1M tokens
        OutputPer1M: 0.60,  // $0.60 per 1M tokens
    },
    "claude-3-5-sonnet-20241022": {
        InputPer1M:  3.00,
        OutputPer1M: 15.00,
    },
    "gemini-pro": {
        InputPer1M:  0.50,
        OutputPer1M: 1.50,
    },
    "codellama": {
        InputPer1M:  0.00,  // Local, free
        OutputPer1M: 0.00,
    },
}

func CalculateCost(tokensIn, tokensOut int, model string) float64 {
    pricing := ProviderPricing[model]
    costIn := float64(tokensIn) / 1_000_000.0 * pricing.InputPer1M
    costOut := float64(tokensOut) / 1_000_000.0 * pricing.OutputPer1M
    return costIn + costOut
}
```

### 6.3 Cost Reporting

Add cost information to `domain.Review`:

```go
type Review struct {
    // ... existing fields ...
    Cost        float64 // USD
    TokensIn    int
    TokensOut   int
}
```

## 7. Testing Strategy

### 7.1 Unit Tests

- Test request construction
- Test response parsing
- Test error handling for all error types
- Test retry logic with simulated failures
- Test cost calculation accuracy
- Test API key validation

### 7.2 Integration Tests

- Mock HTTP server responding with known payloads
- Test full request/response cycle
- Test timeout handling
- Test connection failures
- Test malformed responses
- Run in CI without real API keys

### 7.3 Mock Server Implementation

```go
// internal/testutil/mockllm/server.go
type MockServer struct {
    *httptest.Server
    Requests []Request
    Responses []Response
}

func NewOpenAIMockServer() *MockServer {
    mux := http.NewServeMux()

    mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
        // Validate auth header
        // Parse request
        // Return canned response
        // Record request for verification
    })

    return &MockServer{
        Server: httptest.NewServer(mux),
    }
}
```

### 7.4 Manual Testing

- Test with real APIs (developer's own API keys)
- Verify costs match billing
- Test rate limit handling (intentionally exceed limits)
- Test with various model configurations
- Test with large diffs (>10k tokens)

## 8. Configuration

### 8.1 Provider Configuration

```yaml
providers:
  openai:
    enabled: true
    model: "gpt-4o-mini"
    apiKey: "${OPENAI_API_KEY}"
    timeout: 60s
    maxRetries: 5

  anthropic:
    enabled: true
    model: "claude-3-5-sonnet-20241022"
    apiKey: "${ANTHROPIC_API_KEY}"
    timeout: 90s  # Claude can be slower
    maxRetries: 3

  ollama:
    enabled: true
    model: "codellama"
    host: "http://localhost:11434"
    timeout: 120s  # Local models can be slow

  gemini:
    enabled: true
    model: "gemini-pro"
    apiKey: "${GEMINI_API_KEY}"
    timeout: 60s
    maxRetries: 5
```

### 8.2 HTTP Configuration

```yaml
http:
  timeout: 60s           # Default for all providers
  maxRetries: 5          # Default max retry attempts
  initialBackoff: 2s     # Initial backoff duration
  maxBackoff: 32s        # Maximum backoff duration
  userAgent: "code-reviewer/1.0"
```

## 9. Implementation Plan

### Phase 1: Foundation (Days 1-3)
1. Create shared HTTP utilities (`internal/adapter/llm/http/`)
2. Implement error types
3. Implement retry logic
4. Create mock server infrastructure
5. Write tests for shared components

### Phase 2: OpenAI (Days 4-6)
1. Implement OpenAI client
2. Write comprehensive tests
3. Test with mock server
4. Manual testing with real API
5. Document OpenAI-specific configuration

### Phase 3: Anthropic (Days 7-9)
1. Implement Anthropic client
2. Write comprehensive tests
3. Test with mock server
4. Manual testing with real API
5. Document Anthropic-specific configuration

### Phase 4: Ollama & Gemini (Days 10-14)
1. Implement both clients
2. Write comprehensive tests
3. Test with mock servers
4. Manual testing with real services
5. Document both providers

### Phase 5: Polish (Days 15-17)
1. Refactor common code
2. Add observability (logging, metrics)
3. Security review
4. Performance testing
5. Documentation completion
6. Integration with main application

## 10. Success Criteria

- ✅ All unit tests passing (>80% coverage)
- ✅ All integration tests passing
- ✅ Manual tests successful with all providers
- ✅ Error handling covers all known error types
- ✅ Retry logic tested and working
- ✅ Cost tracking accurate within 5%
- ✅ Security review passed (no API key leaks)
- ✅ Documentation complete
- ✅ Performance: <30s per review (95th percentile)
- ✅ Reliability: <1% transient failure rate

## 11. Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| API changes | High | Medium | Version lock APIs in config, monitor changelogs |
| Rate limits | Medium | High | Implement exponential backoff, document limits |
| Costs | High | Low | Add token counting, cost estimation, budget alerts |
| Security | Critical | Low | Never log credentials, use env vars, code review |
| Reliability | Medium | Medium | Circuit breaker, comprehensive retry logic |
| Testing | Medium | Medium | Comprehensive mock servers, CI without API keys |

## 12. Future Enhancements

- Streaming API support for real-time feedback
- Request/response caching for unchanged code
- Provider fallback (use Claude if OpenAI fails)
- Custom model endpoints (Azure OpenAI, self-hosted)
- Batch API support for cost reduction
- Token count optimization (diff compression)
