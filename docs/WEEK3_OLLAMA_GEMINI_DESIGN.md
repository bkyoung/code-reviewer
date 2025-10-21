# Week 3: Ollama & Gemini HTTP Clients - Technical Design

**Status**: Week 3 Implementation
**Created**: 2025-10-21
**Goal**: Implement production HTTP clients for Ollama (local LLM) and Google Gemini models

## Overview

This document describes the technical design for implementing HTTP clients for:
1. **Ollama** - Local LLM server (no API key, runs on localhost)
2. **Google Gemini** - Google's generative AI models

Both clients follow the same patterns established in Weeks 1-2 (OpenAI, Anthropic).

---

## 1. Ollama HTTP Client

### 1.1 Overview

Ollama is a local LLM server that runs models like LLaMA, Mistral, CodeLLaMA, etc. on your machine. No API key required, just a running Ollama server.

**Key Characteristics**:
- **Local only**: Runs on `http://localhost:11434` by default
- **No authentication**: No API keys needed
- **Streaming**: Supports streaming, but we'll use non-streaming mode
- **Model management**: Can list/pull models via API
- **Format**: Simple JSON request/response

### 1.2 API Specification

**Base URL**: `http://localhost:11434` (configurable via `OLLAMA_HOST`)

**Endpoint**: `POST /api/generate`

**Request Format**:
```json
{
  "model": "codellama",
  "prompt": "Review this code...",
  "stream": false,
  "options": {
    "temperature": 0.0,
    "seed": 12345
  }
}
```

**Response Format**:
```json
{
  "model": "codellama",
  "created_at": "2024-01-01T00:00:00Z",
  "response": "Here is the review...",
  "done": true,
  "context": [1, 2, 3],
  "total_duration": 5000000000,
  "load_duration": 1000000000,
  "prompt_eval_count": 100,
  "eval_count": 200
}
```

**Error Response**:
```json
{
  "error": "model not found"
}
```

### 1.3 Request/Response Types

**File**: `internal/adapter/llm/ollama/types.go`

```go
// GenerateRequest represents a request to Ollama's Generate API
type GenerateRequest struct {
    Model   string                 `json:"model"`
    Prompt  string                 `json:"prompt"`
    Stream  bool                   `json:"stream"`
    Options map[string]interface{} `json:"options,omitempty"`
}

// GenerateResponse represents Ollama's response
type GenerateResponse struct {
    Model            string `json:"model"`
    CreatedAt        string `json:"created_at"`
    Response         string `json:"response"`
    Done             bool   `json:"done"`
    TotalDuration    int64  `json:"total_duration,omitempty"`
    LoadDuration     int64  `json:"load_duration,omitempty"`
    PromptEvalCount  int    `json:"prompt_eval_count,omitempty"`
    EvalCount        int    `json:"eval_count,omitempty"`
}

// ErrorResponse for Ollama errors
type ErrorResponse struct {
    Error string `json:"error"`
}
```

### 1.4 Error Handling

**Connection Refused** (Ollama not running):
- Status: Connection error
- Message: "Ollama server not reachable. Is Ollama running? Try: ollama serve"
- Retryable: No

**Model Not Found**:
- Status: 404
- Message: "Model '{model}' not found. Pull it with: ollama pull {model}"
- Retryable: No

**Server Errors** (500):
- Retryable: Yes (up to 3 retries)

### 1.5 Implementation Details

**File**: `internal/adapter/llm/ollama/client.go`

**Key Methods**:
```go
// NewHTTPClient creates a new Ollama HTTP client
func NewHTTPClient(baseURL, model string) *HTTPClient

// Call makes a request to Ollama's Generate API
func (c *HTTPClient) Call(ctx context.Context, prompt string, options CallOptions) (*APIResponse, error)

// CreateReview implements the Client interface
func (c *HTTPClient) CreateReview(ctx context.Context, req Request) (Response, error)
```

**Configuration**:
- Default URL: `http://localhost:11434`
- Configurable via `OLLAMA_HOST` env var or config
- Default timeout: 120s (local models can be slower)
- No authentication needed

### 1.6 Testing Strategy

**Unit Tests** (13+ test cases):
1. ✅ Successful generation
2. ✅ Connection refused (Ollama not running)
3. ✅ Model not found (404)
4. ✅ Server error with retry (500)
5. ✅ Timeout handling
6. ✅ Context cancellation
7. ✅ Malformed JSON response
8. ✅ Empty response handling
9. ✅ Temperature option setting
10. ✅ Seed option setting
11. ✅ JSON extraction from response
12. ✅ Finding parsing
13. ✅ Multiple retries for transient errors

---

## 2. Google Gemini HTTP Client

### 2.1 Overview

Google Gemini is Google's family of generative AI models, competing with GPT-4 and Claude.

**Key Characteristics**:
- **API Key**: Required (via Google AI Studio or GCP)
- **REST API**: Standard HTTP REST interface
- **Safety Settings**: Content filtering and safety controls
- **Streaming**: Supports streaming, we'll use non-streaming
- **Format**: Structured parts-based content

### 2.2 API Specification

**Base URL**: `https://generativelanguage.googleapis.com`

**Endpoint**: `POST /v1beta/models/{model}:generateContent?key={apiKey}`

**Request Format**:
```json
{
  "contents": [
    {
      "parts": [
        {
          "text": "Review this code..."
        }
      ]
    }
  ],
  "generationConfig": {
    "temperature": 0.0,
    "maxOutputTokens": 4096,
    "candidateCount": 1
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
            "text": "Here is the code review..."
          }
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "safetyRatings": [...]
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 100,
    "candidatesTokenCount": 200,
    "totalTokenCount": 300
  }
}
```

**Error Response**:
```json
{
  "error": {
    "code": 400,
    "message": "API key not valid",
    "status": "INVALID_ARGUMENT"
  }
}
```

### 2.3 Request/Response Types

**File**: `internal/adapter/llm/gemini/types.go`

```go
// GenerateContentRequest represents a Gemini API request
type GenerateContentRequest struct {
    Contents         []Content         `json:"contents"`
    GenerationConfig *GenerationConfig `json:"generationConfig,omitempty"`
    SafetySettings   []SafetySetting   `json:"safetySettings,omitempty"`
}

// Content represents content in the request
type Content struct {
    Parts []Part `json:"parts"`
    Role  string `json:"role,omitempty"` // "user" or "model"
}

// Part represents a part of the content
type Part struct {
    Text string `json:"text"`
}

// GenerationConfig controls generation parameters
type GenerationConfig struct {
    Temperature      float64 `json:"temperature,omitempty"`
    MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
    CandidateCount   int     `json:"candidateCount,omitempty"`
}

// SafetySetting configures content filtering
type SafetySetting struct {
    Category  string `json:"category"`
    Threshold string `json:"threshold"`
}

// GenerateContentResponse from Gemini
type GenerateContentResponse struct {
    Candidates     []Candidate    `json:"candidates"`
    UsageMetadata  UsageMetadata  `json:"usageMetadata"`
}

// Candidate represents a generated candidate
type Candidate struct {
    Content       Content  `json:"content"`
    FinishReason  string   `json:"finishReason"`
}

// UsageMetadata contains token usage info
type UsageMetadata struct {
    PromptTokenCount      int `json:"promptTokenCount"`
    CandidatesTokenCount  int `json:"candidatesTokenCount"`
    TotalTokenCount       int `json:"totalTokenCount"`
}

// ErrorResponse for Gemini errors
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Status  string `json:"status"`
}
```

### 2.4 Error Handling

**Authentication Error** (401/403):
- Message: "Invalid API key"
- Retryable: No

**Rate Limit** (429):
- Message: "Resource exhausted"
- Retryable: Yes
- Backoff: Exponential (2s, 4s, 8s, 16s, 32s)

**Content Filtered** (400 with SAFETY block):
- FinishReason: "SAFETY"
- Message: "Content blocked by safety filters"
- Retryable: No

**Quota Exceeded**:
- Status: RESOURCE_EXHAUSTED
- Retryable: No (daily quota)

### 2.5 Implementation Details

**File**: `internal/adapter/llm/gemini/client.go`

**Key Methods**:
```go
// NewHTTPClient creates a new Gemini HTTP client
func NewHTTPClient(apiKey, model string) *HTTPClient

// Call makes a request to Gemini's generateContent API
func (c *HTTPClient) Call(ctx context.Context, prompt string, options CallOptions) (*APIResponse, error)

// CreateReview implements the Client interface
func (c *HTTPClient) CreateReview(ctx context.Context, req Request) (Response, error)
```

**Configuration**:
- API Key: Via URL parameter or header
- Default safety settings: Block only high-severity content
- Default max tokens: 4096
- Temperature: Configurable (0.0 for determinism)
- Candidate count: 1

### 2.6 Testing Strategy

**Unit Tests** (14+ test cases):
1. ✅ Successful content generation
2. ✅ Authentication error (401)
3. ✅ Rate limit with retry (429)
4. ✅ Invalid request (400)
5. ✅ Content filtered by safety (SAFETY finish reason)
6. ✅ Quota exceeded (RESOURCE_EXHAUSTED)
7. ✅ Timeout handling
8. ✅ Context cancellation
9. ✅ Malformed JSON response
10. ✅ Empty candidates array
11. ✅ Multiple parts concatenation
12. ✅ Temperature setting
13. ✅ Max tokens setting
14. ✅ JSON extraction and finding parsing

---

## 3. Integration Plan

### 3.1 main.go Updates

**Ollama Provider**:
```go
// Ollama provider (local LLM)
if cfg, ok := providersConfig["ollama"]; ok && cfg.Enabled {
    model := cfg.Model
    if model == "" {
        model = "codellama"
    }

    // Use configured host or default to localhost
    host := cfg.Host // New field
    if host == "" {
        host = os.Getenv("OLLAMA_HOST")
    }
    if host == "" {
        host = "http://localhost:11434"
    }

    providers["ollama"] = ollama.NewProvider(model, ollama.NewHTTPClient(host, model))
}
```

**Gemini Provider**:
```go
// Google Gemini provider
if cfg, ok := providersConfig["gemini"]; ok && cfg.Enabled {
    model := cfg.Model
    if model == "" {
        model = "gemini-1.5-pro"
    }

    apiKey := cfg.APIKey
    if apiKey == "" {
        log.Println("Gemini: No API key provided, skipping provider")
    } else {
        providers["gemini"] = gemini.NewProvider(model, gemini.NewHTTPClient(apiKey, model))
    }
}
```

### 3.2 Configuration Changes

**Add to config.go**:
```go
type ProviderConfig struct {
    Enabled bool   `mapstructure:"enabled"`
    Model   string `mapstructure:"model"`
    APIKey  string `mapstructure:"apiKey"`
    Host    string `mapstructure:"host"` // For Ollama
}
```

**Example config**:
```yaml
providers:
  ollama:
    enabled: true
    model: "codellama"
    host: "http://localhost:11434"  # Optional, defaults to this

  gemini:
    enabled: true
    model: "gemini-1.5-pro"
    apiKey: "${GEMINI_API_KEY}"
```

---

## 4. Comparison Matrix

| Feature | Ollama | Gemini | OpenAI | Anthropic |
|---------|--------|--------|--------|-----------|
| **Authentication** | None | API key in URL | Bearer token | x-api-key header |
| **Base URL** | localhost:11434 | generativelanguage.googleapis.com | api.openai.com | api.anthropic.com |
| **API Format** | Simple JSON | Parts-based | Messages | Messages |
| **Streaming** | Yes (disabled) | Yes (disabled) | Yes (disabled) | Yes (disabled) |
| **Temperature** | ✅ | ✅ | ✅ | ✅ |
| **Seed** | ✅ | ❌ | ✅ (non-o1) | ❌ |
| **Max Tokens** | ✅ | ✅ | ✅ | ✅ (required) |
| **Safety Filters** | ❌ | ✅ | ❌ | ❌ |
| **Error Retry** | Limited | ✅ | ✅ | ✅ |
| **Cost** | Free (local) | Paid | Paid | Paid |

---

## 5. Testing Approach

### 5.1 Ollama Testing

**Mock Server Tests**:
- Simulate Ollama server responses
- Test connection refused scenario
- Test model not found
- Test successful generation

**Error Scenarios**:
- Connection refused (friendly error message)
- Model not found (suggest `ollama pull`)
- Server errors with retry

### 5.2 Gemini Testing

**Mock Server Tests**:
- Simulate Gemini API responses
- Test all error codes
- Test safety filtering
- Test parts concatenation

**Error Scenarios**:
- Invalid API key
- Rate limiting
- Content filtering
- Quota exceeded

---

## 6. Documentation Updates

### 6.1 CONFIGURATION.md

Add sections for:
- Ollama setup and configuration
- Gemini API key setup
- Model recommendations
- Troubleshooting (Ollama not running, etc.)

### 6.2 HTTP_CLIENT_TODO.md

Mark Week 3 tasks as completed:
- All Ollama client tasks ✅
- All Gemini client tasks ✅

---

## 7. Implementation Order

1. **Ollama First** (simpler, no auth)
   - Create types
   - Write tests
   - Implement client
   - Integrate with main.go
   - Test locally

2. **Gemini Second**
   - Create types
   - Write tests
   - Implement client
   - Integrate with main.go
   - Test with API key

3. **Final Integration**
   - Update documentation
   - Run full CI suite
   - Manual testing with all 4 providers
   - Commit and update checklists

---

## 8. Success Criteria

- ✅ Ollama client: 13+ tests passing
- ✅ Gemini client: 14+ tests passing
- ✅ Full CI suite passes
- ✅ Integration with main.go complete
- ✅ Documentation updated
- ✅ Manual testing with local Ollama server (if available)
- ✅ Manual testing with Gemini API key (if available)
- ✅ All 4 providers (OpenAI, Anthropic, Ollama, Gemini) working

---

## 9. Known Limitations

### Ollama
- Requires Ollama server running locally
- Models must be pre-pulled (`ollama pull codellama`)
- Performance varies based on hardware
- No cloud fallback

### Gemini
- Content filtering may block legitimate code review requests
- No seed support for determinism
- Different pricing tiers
- Quota limits vary by tier

---

## 10. Future Enhancements (Post-Week 3)

- [ ] Ollama: Add model availability check before request
- [ ] Ollama: Support custom system prompts
- [ ] Gemini: Fine-tune safety settings per use case
- [ ] Gemini: Support multiple candidates and selection
- [ ] Both: Add streaming support (optional)
- [ ] Both: Add caching for repeated requests
