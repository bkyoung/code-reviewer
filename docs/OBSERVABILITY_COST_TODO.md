# Observability & Cost Tracking Implementation Checklist

Status: Planning → In Progress
Started: 2025-10-21
Updated: 2025-10-21

## Goal
Add observability (logging, metrics, duration tracking) and cost tracking (token counting, cost estimation) to all HTTP LLM clients. This provides visibility into API usage, helps debug issues, and prepares for Phase 4 budget enforcement.

## Overview

Currently the HTTP clients lack:
- Request/response logging for debugging
- Duration tracking for performance monitoring
- Token usage tracking for cost estimation
- Cost calculation per provider
- Structured output for analysis

This batch adds these capabilities while maintaining clean architecture and testability.

## Week 1: Observability Infrastructure (Days 1-3)

### 1.1 Logging Infrastructure (TDD) ⏳
- [ ] Create `internal/adapter/llm/http/logger.go`
- [ ] Define Logger interface with methods: LogRequest, LogResponse, LogError
- [ ] Write tests for request logging (redacts API keys)
- [ ] Implement request logger with structured output
- [ ] Write tests for response logging (includes duration, tokens)
- [ ] Implement response logger
- [ ] Write tests for API key redaction (show only last 4 chars)
- [ ] Implement redaction logic
- [ ] Add log level support (debug, info, error)

### 1.2 Metrics Infrastructure (TDD) ⏳
- [ ] Create `internal/adapter/llm/http/metrics.go`
- [ ] Define Metrics interface with methods: RecordRequest, RecordDuration, RecordTokens
- [ ] Write tests for duration tracking
- [ ] Implement duration tracking with time.Now()
- [ ] Write tests for token counting
- [ ] Implement token counter
- [ ] Write tests for error counting by type
- [ ] Implement error counter

### 1.3 Integration with Existing Clients ⏳
- [ ] Update OpenAI client to use logger
- [ ] Update OpenAI client to track duration
- [ ] Update OpenAI client to track tokens
- [ ] Update Anthropic client to use logger
- [ ] Update Anthropic client to track duration
- [ ] Update Anthropic client to track tokens
- [ ] Update Ollama client to use logger
- [ ] Update Ollama client to track duration
- [ ] Update Ollama client to track tokens
- [ ] Update Gemini client to use logger
- [ ] Update Gemini client to track duration
- [ ] Update Gemini client to track tokens

## Week 2: Cost Tracking (Days 4-6)

### 2.1 Cost Calculation Infrastructure (TDD) ⏳
- [ ] Create `internal/adapter/llm/http/pricing.go`
- [ ] Define Pricing interface with GetCost(model, tokensIn, tokensOut) method
- [ ] Write tests for OpenAI pricing (gpt-4o, gpt-4o-mini, o1, etc.)
- [ ] Implement OpenAI pricing calculator
- [ ] Write tests for Anthropic pricing (claude-3-5-sonnet, haiku, etc.)
- [ ] Implement Anthropic pricing calculator
- [ ] Write tests for Ollama pricing (free/local)
- [ ] Implement Ollama pricing (returns $0)
- [ ] Write tests for Gemini pricing (gemini-1.5-pro, flash, etc.)
- [ ] Implement Gemini pricing calculator
- [ ] Document pricing data sources and update dates

### 2.2 Cost Tracking in Responses ⏳
- [ ] Add Cost field to domain.Review struct
- [ ] Update OpenAI client to calculate and return cost
- [ ] Write tests for OpenAI cost calculation
- [ ] Update Anthropic client to calculate and return cost
- [ ] Write tests for Anthropic cost calculation
- [ ] Update Ollama client to return $0 cost
- [ ] Write tests for Ollama cost (free)
- [ ] Update Gemini client to calculate and return cost
- [ ] Write tests for Gemini cost calculation

### 2.3 Cost Aggregation in Orchestrator ⏳
- [ ] Update orchestrator to sum costs from all providers
- [ ] Add TotalCost field to merged review output
- [ ] Write tests for cost aggregation
- [ ] Update Markdown writer to include cost summary
- [ ] Update JSON writer to include cost data
- [ ] Update SARIF writer to include cost metadata

## Week 3: Configuration & Polish (Days 7-9)

### 3.1 Configuration Support ⏳
- [ ] Add `observability` section to config
- [ ] Add `observability.logging.enabled` option (default: true)
- [ ] Add `observability.logging.level` option (debug/info/error)
- [ ] Add `observability.logging.redact_api_keys` option (default: true)
- [ ] Add `observability.metrics.enabled` option (default: true)
- [ ] Write tests for config loading
- [ ] Update config loader with validation
- [ ] Document all observability config options

### 3.2 Output Formats ⏳
- [ ] Design structured log format (JSON lines)
- [ ] Write tests for JSON log output
- [ ] Implement JSON logger
- [ ] Write tests for human-readable log output
- [ ] Implement human-readable logger
- [ ] Add log output destination config (stdout, file)

### 3.3 Documentation ⏳
- [ ] Create OBSERVABILITY.md with logging examples
- [ ] Document log format and fields
- [ ] Create COST_TRACKING.md with pricing tables
- [ ] Document cost calculation formulas
- [ ] Add troubleshooting guide for log analysis
- [ ] Update CONFIGURATION.md with observability options
- [ ] Add examples to README

### 3.4 Testing & Validation ⏳
- [ ] Write integration tests for logging
- [ ] Write integration tests for metrics
- [ ] Write integration tests for cost tracking
- [ ] Verify API key redaction works
- [ ] Test with all 4 providers manually
- [ ] Verify cost calculations match provider billing
- [ ] Run full CI suite

## Dependencies

No new external dependencies needed:
- Logging: stdlib `log` package
- Metrics: stdlib types
- Cost: simple multiplication

## Testing Commands

```bash
# Unit tests
go test ./internal/adapter/llm/http/... -v
go test ./internal/adapter/llm/openai/... -v
go test ./internal/adapter/llm/anthropic/... -v
go test ./internal/adapter/llm/ollama/... -v
go test ./internal/adapter/llm/gemini/... -v

# Integration tests
go test ./internal/usecase/review/... -v

# Manual testing with observability
DEBUG=1 ./cr review branch HEAD --base HEAD~1

# Check cost tracking
./cr review branch HEAD --base HEAD~1 | grep -i cost
```

## Completion Criteria

- [ ] All unit tests passing (target: 50+ new tests)
- [ ] All integration tests passing
- [ ] API keys properly redacted in all logs
- [ ] Duration tracking works for all providers
- [ ] Token counts match provider API responses
- [ ] Cost calculations within 5% of actual billing
- [ ] Documentation complete with examples
- [ ] Manual testing with all 4 providers successful
- [ ] No performance regression (<10ms overhead per request)

## Success Metrics

- **API Key Security**: 0% API key exposure in logs
- **Cost Accuracy**: ≤5% variance from provider billing
- **Performance**: <10ms overhead for logging/metrics
- **Coverage**: >80% test coverage for new code
- **Usability**: Clear, actionable log messages

## Notes

- Keep logging opt-in via config (don't spam output by default)
- Use structured logging (JSON) for machine parsing
- Redact API keys even in debug mode
- Document pricing data source and update frequency
- Consider future: export metrics to Prometheus/OpenTelemetry
- Pricing may change - document as of implementation date
- Token counting from API responses (not estimated locally)

## Risk Mitigation

**API Key Leaks**: Comprehensive redaction tests, code review
**Performance**: Benchmark logging overhead, use conditional logging
**Pricing Changes**: Document data sources, add update dates
**Log Volume**: Make logging configurable, support log levels
**Cost Calculation Errors**: Validate against real billing data
