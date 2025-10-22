# Configurable HTTP Settings - Technical Design

**Version**: v0.1.5
**Date**: 2025-10-22
**Status**: Implementation

## Problem Statement

HTTP client settings (timeout, retries, backoff) are currently hardcoded in each LLM client. This creates inflexibility for users who need different timeouts for different environments (e.g., slow Ollama models vs fast cloud APIs) or want to customize retry behavior.

## Current State

### Hardcoded Timeout Values

**Location**: Each client's `defaultTimeout` constant

| Client | Timeout | File |
|--------|---------|------|
| OpenAI | 60s | `internal/adapter/llm/openai/client.go:20` |
| Anthropic | 60s | `internal/adapter/llm/anthropic/client.go:20` |
| Gemini | 60s | `internal/adapter/llm/gemini/client.go:20` |
| Ollama | 120s | `internal/adapter/llm/ollama/client.go:19` |

### Hardcoded Retry Values

**Location**: Each client's Call method creates `RetryConfig`

**OpenAI** (`client.go:218-223`):
```go
retryConfig := llmhttp.RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 2 * time.Second,
    MaxBackoff:     32 * time.Second,
    Multiplier:     2.0,
}
```

**Anthropic, Gemini, Ollama**: Similar inline RetryConfig creation

### Problems

❌ **No user control** - Users can't adjust timeouts for their environment
❌ **Hardcoded in code** - Changes require code modifications
❌ **Inconsistent across providers** - OpenAI uses 3 retries, others may differ
❌ **No per-provider customization** - Can't set different timeout for Ollama vs OpenAI

## Design

### Solution: Add HTTP Configuration Section

Add a new `http` section to the config with global defaults and per-provider overrides.

```yaml
# Global HTTP defaults (apply to all providers unless overridden)
http:
  timeout: "60s"              # Default timeout for HTTP requests
  maxRetries: 5               # Default max retry attempts
  initialBackoff: "2s"        # Initial backoff duration
  maxBackoff: "32s"           # Maximum backoff duration
  backoffMultiplier: 2.0      # Backoff multiplier (exponential)

# Provider-specific overrides
providers:
  ollama:
    enabled: true
    model: "llama2"
    timeout: "120s"            # Override: Ollama needs longer timeout
    maxRetries: 3              # Override: Fewer retries for local models

  openai:
    enabled: true
    apiKey: "${OPENAI_API_KEY}"
    model: "gpt-4o"
    # Uses global http defaults
```

### Config Structure

**Update `config.go`**:

```go
type Config struct {
	Providers     map[string]ProviderConfig `yaml:"providers"`
	HTTP          HTTPConfig                `yaml:"http"`      // NEW
	Merge         MergeConfig               `yaml:"merge"`
	Git           GitConfig                 `yaml:"git"`
	// ... rest unchanged
}

// HTTPConfig holds global HTTP client settings.
type HTTPConfig struct {
	Timeout           string  `yaml:"timeout"`           // e.g., "60s", "2m"
	MaxRetries        int     `yaml:"maxRetries"`
	InitialBackoff    string  `yaml:"initialBackoff"`    // e.g., "2s"
	MaxBackoff        string  `yaml:"maxBackoff"`        // e.g., "32s"
	BackoffMultiplier float64 `yaml:"backoffMultiplier"`
}

// ProviderConfig configures a single LLM provider.
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`
	APIKey  string `yaml:"apiKey"`

	// HTTP overrides (optional, nil = use global defaults)
	Timeout        *string `yaml:"timeout,omitempty"`        // NEW
	MaxRetries     *int    `yaml:"maxRetries,omitempty"`     // NEW
	InitialBackoff *string `yaml:"initialBackoff,omitempty"` // NEW
	MaxBackoff     *string `yaml:"maxBackoff,omitempty"`     // NEW
}
```

### Default Values

**In `loader.go` `setDefaults`**:

```go
// HTTP defaults
v.SetDefault("http.timeout", "60s")
v.SetDefault("http.maxRetries", 5)
v.SetDefault("http.initialBackoff", "2s")
v.SetDefault("http.maxBackoff", "32s")
v.SetDefault("http.backoffMultiplier", 2.0)
```

### Client Updates

**Each client (OpenAI, Anthropic, Gemini, Ollama)**:

1. Add `config ProviderConfig` field to HTTPClient struct
2. Update `NewHTTPClient` to accept config
3. Use config.Timeout (with fallback to global default)
4. Build RetryConfig from config values

**Example** (`openai/client.go`):

```go
type HTTPClient struct {
	model      string
	apiKey     string
	baseURL    string
	timeout    time.Duration   // NEW: from config
	retryConf  llmhttp.RetryConfig // NEW: from config
	client     *http.Client
	logger     llmhttp.Logger
	metrics    llmhttp.Metrics
	pricing    llmhttp.Pricing
}

func NewHTTPClient(apiKey string, config ProviderConfig, httpConfig HTTPConfig, logger llmhttp.Logger, metrics llmhttp.Metrics, pricing llmhttp.Pricing) (*HTTPClient, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	// Determine timeout: provider override > global default > hardcoded fallback
	timeout := parseTimeout(config.Timeout, httpConfig.Timeout, 60*time.Second)

	// Build retry config from global + provider overrides
	retryConf := buildRetryConfig(config, httpConfig)

	return &HTTPClient{
		model:      config.Model,
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		timeout:    timeout,
		retryConf:  retryConf,
		client:     &http.Client{Timeout: timeout},
		logger:     logger,
		metrics:    metrics,
		pricing:    pricing,
	}, nil
}

// In Call method, replace hardcoded RetryConfig with c.retryConf
err = llmhttp.RetryWithBackoff(ctx, operation, c.retryConf)
```

### Helper Functions

```go
// parseTimeout parses timeout with fallback chain: override > global > default
func parseTimeout(override *string, global string, defaultVal time.Duration) time.Duration {
	if override != nil && *override != "" {
		if d, err := time.ParseDuration(*override); err == nil {
			return d
		}
	}
	if global != "" {
		if d, err := time.ParseDuration(global); err == nil {
			return d
		}
	}
	return defaultVal
}

// buildRetryConfig creates RetryConfig from provider + global config
func buildRetryConfig(provider ProviderConfig, global HTTPConfig) llmhttp.RetryConfig {
	maxRetries := global.MaxRetries
	if provider.MaxRetries != nil {
		maxRetries = *provider.MaxRetries
	}

	initialBackoff := parseDuration(provider.InitialBackoff, global.InitialBackoff, 2*time.Second)
	maxBackoff := parseDuration(provider.MaxBackoff, global.MaxBackoff, 32*time.Second)

	return llmhttp.RetryConfig{
		MaxRetries:     maxRetries,
		InitialBackoff: initialBackoff,
		MaxBackoff:     maxBackoff,
		Multiplier:     global.BackoffMultiplier,
	}
}
```

## Testing Strategy

### Unit Tests (TDD Approach)

**Test file**: `internal/config/loader_test.go`

1. **Test HTTP config defaults**
   - Load empty config
   - Verify: http.timeout="60s", maxRetries=5, etc.

2. **Test HTTP config from file**
   - Config with custom HTTP section
   - Verify: Values are loaded correctly

3. **Test provider timeout override**
   - Provider with timeout override
   - Verify: Provider timeout takes precedence

4. **Test env var expansion in HTTP config**
   - Config: `{http: {timeout: "${TIMEOUT}"}}`
   - Verify: Env var is expanded

**Integration Tests**: Update client tests to verify config is used

## Implementation Plan (TDD)

### Phase 1: Config Structure (1 hour)
1. Add HTTPConfig to config.go
2. Add optional HTTP fields to ProviderConfig (pointers for nil detection)
3. Add defaults in loader.go
4. Write tests (Red phase)
5. Update expandEnvVars for HTTP section
6. Tests pass (Green phase)

### Phase 2: Update Clients (1 hour)
One client at a time:
1. Update OpenAI client constructor and Call method
2. Run OpenAI tests
3. Update Anthropic client
4. Update Gemini client
5. Update Ollama client (special case: keep 120s default if no config)

### Phase 3: Update main.go (15 min)
- Pass HTTP config when creating clients
- Update CLI initialization

### Phase 4: Verification (15 min)
- Full test suite
- Race detector
- Format, build, CI

### Phase 5: Documentation (15 min)
- Update ROADMAP.md
- Archive design doc
- Tag v0.1.5

## Benefits

✅ **Flexibility** - Users can customize HTTP behavior per environment
✅ **Per-provider control** - Different timeouts for Ollama vs cloud providers
✅ **Environment-aware** - Use env vars: `timeout: "${REQUEST_TIMEOUT}"`
✅ **Backwards compatible** - Defaults match current behavior
✅ **Consistent** - Single source of truth for HTTP settings

## Non-Goals

- NOT changing retry algorithm (keep exponential backoff)
- NOT adding circuit breakers (future enhancement)
- NOT adding connection pooling config (future enhancement)

## Backwards Compatibility

**100% backwards compatible**:
- Defaults match current hardcoded values
- Existing configs continue to work
- Only adds new optional fields

## Time Estimate

- Phase 1: Config structure + tests: 1 hour
- Phase 2: Update clients: 1 hour
- Phase 3: main.go updates: 15 min
- Phase 4: Verification: 15 min
- Phase 5: Documentation: 15 min
- **Total**: ~2.5 hours

## Success Criteria

✅ HTTPConfig section added to config with defaults
✅ Provider-specific HTTP overrides work
✅ All clients use config values instead of hardcoded
✅ Env var expansion works for HTTP config
✅ All 140+ tests still passing
✅ Zero data races
✅ CI passes
