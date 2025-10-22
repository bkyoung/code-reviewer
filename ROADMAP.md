# Code Reviewer Roadmap

## Current Status

**v0.1.1 - Production Hardening Complete** ✅

The code reviewer now has:
- ✅ Multi-provider LLM support (OpenAI, Anthropic, Gemini, Ollama)
- ✅ Full HTTP client implementation with retry logic and error handling
- ✅ Comprehensive observability (logging, metrics, cost tracking)
- ✅ Structured logging throughout orchestrator
- ✅ SQLite-based review persistence
- ✅ Multiple output formats (Markdown, JSON, SARIF)
- ✅ Configuration system with environment variable support
- ✅ Secret redaction
- ✅ Deterministic reviews for CI/CD
- ✅ Production-ready retry logic with edge case handling
- ✅ All unit and integration tests passing (125+ tests)
- ✅ Zero data races (verified with race detector)

## Near-Term Enhancements

### 1. Manual Testing & Verification (Optional)
**Priority: Low**

- [ ] Manual testing with real API keys for all 4 providers
- [ ] Verify cost calculations match actual provider billing
- [ ] Test database persistence with real reviews
- [ ] Inspect SQLite database schema and data
- [ ] Performance testing with large diffs

### 2. Configuration Enhancements
**Priority: Low**

- [ ] Add `http.timeout` config option (currently hardcoded to 60s)
- [ ] Add `http.maxRetries` config option (currently hardcoded to 5)
- [ ] Add `http.retryBackoff` config option for customizing backoff strategy
- [ ] Add provider-specific timeout overrides

### 3. Resilience Features
**Priority: Low**

- [ ] Implement circuit breaker pattern for repeated failures
- [ ] Add graceful shutdown handling for in-flight requests
- [ ] Improve context propagation and cancellation support

## Known Issues & Technical Debt

This section tracks issues identified through code reviews and technical debt items to be addressed in future releases.

### Medium Priority

#### 4. Extract Shared JSON Parsing Logic
**Source**: OpenAI code review feedback
**Locations**: All LLM clients
**Status**: Code duplication

Each provider duplicates JSON extraction and parsing logic from markdown code blocks. Should extract to `internal/adapter/llm/http/json_extractor.go`.

**Benefits**: DRY principle, easier maintenance, consistent parsing behavior.

#### 5. Deduplicate ID Generation
**Source**: OpenAI code review feedback
**Locations**: `internal/usecase/review/orchestrator.go`, `internal/store/util.go`
**Status**: Needs investigation

ID generation functions may be duplicated between orchestrator and store utilities. Verify and consolidate if appropriate.

#### 6. Environment Variable Expansion for All Config
**Source**: OpenAI code review feedback
**Location**: `internal/config/loader.go`
**Status**: Incomplete feature

Env var expansion (`${VAR}`) may not be applied to all config sections (merge, redaction, budget). Ensure `expandEnvString` is called recursively on all string fields.

## Recently Fixed Issues

### ✅ OpenAI Retry Bug - Request Body Consumed
**Fixed**: 2025-10-21
**Location**: `internal/adapter/llm/openai/client.go:162-180`
**Severity**: HIGH (broke retry functionality)

**Problem**: The retry operation created request once with `bytes.NewBuffer(jsonData)` then reused the same `req` variable in retry closure. After first HTTP request, `req.Body` was consumed and subsequent retries sent empty bodies.

**Solution**: Moved request creation inside retry operation closure, recreating request body on each attempt (matching Anthropic/Gemini/Ollama pattern).

### ✅ FOREIGN KEY Constraint Failed
**Fixed**: 2025-10-21
**Location**: `internal/usecase/review/orchestrator.go`
**Severity**: CRITICAL (broke review persistence)

**Problem**: CreateRun was called AFTER provider goroutines tried to save reviews, causing foreign key constraint violations.

**Solution**: Moved CreateRun before launching goroutines, added UpdateRunCost method to update total cost after all reviews complete.

### ✅ Production Hardening Sprint (v0.1.1)
**Fixed**: 2025-10-22
**Scope**: Multiple locations across codebase

#### Quick Wins
1. **Magic Number Documentation** - Added named constant `maxInt64Mask` in `internal/determinism/seed.go` for better code readability
2. **SARIF Cost Validation** - Added NaN/Inf validation in `internal/adapter/output/sarif/writer.go` to prevent JSON marshaling errors
3. **API Key Redaction Format** - Improved format from `****cdef` to `[REDACTED-cdef]` in `internal/adapter/llm/http/logger.go`

#### RetryWithBackoff Edge Case
**Location**: `internal/adapter/llm/http/retry.go`

Added test coverage for context cancellation before first attempt. Verified correct error handling when context is already cancelled.

#### Response Body Leak Prevention Audit
**Locations**: All HTTP clients

Comprehensive audit of all 4 LLM HTTP clients (OpenAI, Anthropic, Gemini, Ollama). Verified all clients properly use `defer resp.Body.Close()` pattern. Ran race detector tests - zero data races found.

#### Structured Logging Throughout
**Locations**: `internal/usecase/review/logger.go`, `internal/adapter/observability/logger.go`, `cmd/cr/main.go`, `internal/usecase/review/orchestrator.go`

- Created `review.Logger` interface in use case layer
- Implemented `observability.ReviewLogger` adapter
- Replaced all `fmt.Printf` calls in orchestrator with conditional structured logging
- Graceful fallback to `log.Printf` when logger is nil
- Comprehensive test coverage for logger adapter

**Impact**: Better production observability, consistent log formats, easier log aggregation and filtering.

## Future Features (Deferred)

### Phase 3 Continuation: TUI & Intelligence (Weeks 2-4)

**Status: Deferred - Store infrastructure complete, TUI not yet implemented**

#### TUI Implementation (Week 2)
- [ ] Add Bubble Tea, Bubbles, and Lipgloss dependencies
- [ ] Create `internal/adapter/tui/` package
- [ ] Implement review list view (load runs from store)
- [ ] Implement finding list view (show findings by severity)
- [ ] Implement finding detail view (scrollable viewport)
- [ ] Add navigation and key bindings
- [ ] Add `tui` command to CLI

#### Feedback & Intelligence (Week 3)
- [ ] Add feedback capture in TUI ('a' accept, 'r' reject)
- [ ] Implement feedback processor use case
- [ ] Create statistics view showing precision by provider
- [ ] Implement intelligent merger v2 (uses precision priors)
- [ ] Update merger configuration (`strategy: "intelligent"`)
- [ ] Wire precision priors into scoring algorithm

#### Enhanced Redaction (Week 4)
- [ ] Implement entropy-based secret detection
- [ ] Add Shannon entropy calculation
- [ ] Integrate entropy detector into redaction engine
- [ ] Add config options for entropy threshold
- [ ] Combine regex + entropy detection for better coverage

### Phase 4: Budget Enforcement & Cost Control

**Status: Not Started - Cost tracking infrastructure complete**

- [ ] Add budget.hardCapUSD config option
- [ ] Implement pre-flight cost estimation
- [ ] Add degradation policies (reduce providers, reduce context)
- [ ] Create budget tracking middleware
- [ ] Add warnings when approaching budget limits
- [ ] Reject reviews that would exceed hard cap

### Phase 5: Multi-Repository & CI/CD

**Status: Not Started**

- [ ] Support reviewing multiple repositories
- [ ] Add PR/MR integration (GitHub, GitLab)
- [ ] Implement GitHub Actions workflow
- [ ] Add GitLab CI template
- [ ] Create Docker image for containerized reviews
- [ ] Add webhook support for automatic reviews

### Advanced Features (Backlog)

#### Model Experimentation
- [ ] Add model comparison mode (compare outputs side-by-side)
- [ ] Implement A/B testing framework
- [ ] Add custom prompt templates
- [ ] Support for fine-tuned models

#### Collaboration
- [ ] Export/import review history
- [ ] Share precision priors across teams
- [ ] Generate team-wide statistics
- [ ] Create learning datasets from feedback

#### Performance
- [ ] Implement request batching for large diffs
- [ ] Add streaming support for faster feedback
- [ ] Optimize token usage with smart chunking
- [ ] Add caching for repeated diff analysis

#### Integration
- [ ] Prometheus metrics export
- [ ] OpenTelemetry tracing support
- [ ] Slack/Discord notifications
- [ ] Email digest of review summaries

## Completed Work (Archive)

See `docs/archive/` for detailed implementation checklists:

- **Phase 1**: Core architecture and domain model
- **Phase 2**: Git integration and basic review workflow
- **HTTP Clients**: All 4 providers (OpenAI, Anthropic, Gemini, Ollama)
- **Observability**: Logging, metrics, and cost tracking
- **Store Integration**: SQLite persistence with full orchestrator integration
- **Configuration**: Complete config system with environment variable expansion

## Contributing

When adding new features:

1. Follow TDD (test-driven development)
2. Maintain clean architecture principles
3. Update documentation
4. Ensure all tests pass (`mage ci`)
5. Update this roadmap

## Release Planning

### v0.1.0 (Released)
- Core review functionality
- Multi-provider support (OpenAI, Anthropic, Gemini, Ollama)
- Observability and cost tracking
- Review persistence (SQLite)

### v0.1.1 (Current)
- Production hardening
- Structured logging throughout
- Edge case handling in retry logic
- Code quality improvements
- Zero data races

### v0.2.0 (Future)
- TUI for review history
- Feedback and intelligent merging
- Enhanced secret detection

### v0.3.0 (Future)
- Budget enforcement
- Advanced cost controls

### v1.0.0 (Future)
- Production-ready
- CI/CD integrations
- Comprehensive documentation
- Performance optimized
