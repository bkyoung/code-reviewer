# Code Reviewer Roadmap

## Current Status

**v0.1.3 - Code Quality Improvements Complete** ✅

The code reviewer now has:
- ✅ Multi-provider LLM support (OpenAI, Anthropic, Gemini, Ollama)
- ✅ Full HTTP client implementation with retry logic and error handling
- ✅ Comprehensive observability (logging, metrics, cost tracking)
- ✅ True structured logging - JSON and human-readable formats throughout
- ✅ **Shared JSON parsing utilities** - Zero duplication across LLM clients
- ✅ SQLite-based review persistence
- ✅ Multiple output formats (Markdown, JSON, SARIF)
- ✅ Configuration system with environment variable support
- ✅ Secret redaction
- ✅ Deterministic reviews for CI/CD
- ✅ Production-ready retry logic with edge case handling
- ✅ **Clean architecture integrity** - Intentional duplication documented
- ✅ All unit and integration tests passing (135+ tests)
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

#### 4. Environment Variable Expansion for All Config
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

### ✅ Structured Logging Fix (v0.1.2)
**Fixed**: 2025-10-22
**Locations**: `internal/adapter/llm/http/logger.go`, `internal/adapter/observability/logger.go`
**Severity**: MEDIUM (incomplete feature from v0.1.1)

**Problem**: ReviewLogger received an llmhttp.Logger but never used it, falling back to unstructured `log.Printf`. The llmhttp.Logger interface lacked generic LogWarning/LogInfo methods needed by the orchestrator.

**Solution**: Extended llmhttp.Logger interface with LogWarning/LogInfo methods, implemented both JSON and human-readable formats in DefaultLogger, updated ReviewLogger to delegate properly.

**Changes**:
- Extended Logger interface with LogWarning and LogInfo methods
- Implemented JSON format: `{"level":"warning","timestamp":"...","message":"...","field1":"value1"}`
- Implemented human format: `[WARN] 2025-10-22T10:30:45Z message field1=value1 field2=value2`
- ReviewLogger now delegates to injected logger instead of using log.Printf
- Log level filtering (Debug/Info/Error) works correctly
- Comprehensive test coverage (60+ logger tests, 130+ total tests)
- Zero data races verified with race detector

**Impact**: True structured logging throughout application. Logs now use consistent formats (JSON or human-readable) with proper timestamps and structured fields, making production debugging and log aggregation significantly easier.

**Feedback sources**: OpenAI o4-mini and Anthropic Claude reviews (Oct 22, 2025)

### ✅ Code Quality Improvements (v0.1.3)
**Fixed**: 2025-10-22
**Scope**: Multiple locations across codebase
**Severity**: MEDIUM (code duplication and architecture clarity)

**Problem 1: JSON Parsing Duplication**
All 4 LLM clients (OpenAI, Anthropic, Gemini, Ollama) duplicated JSON extraction and parsing logic from markdown code blocks. Each client had its own `parseReviewJSON` and `extractJSONFromMarkdown` functions with slightly different regex patterns, causing maintenance burden.

**Solution 1: Shared JSON Utilities**
- Created `internal/adapter/llm/http/json.go` with shared utilities
- `ExtractJSONFromMarkdown`: Handles both ```json and ``` code blocks
- `ParseReviewResponse`: Parses JSON into summary and findings
- Updated all 4 clients to use shared parsing
- Removed ~80 lines of duplicated code across clients
- Comprehensive test coverage (17 tests for JSON parsing)

**Problem 2: ID Generation "Duplication"**
ID generation functions appeared duplicated between `internal/usecase/review/store_helpers.go` and `internal/store/util.go`, flagged as potential code duplication.

**Solution 2: Documentation & Testing**
After investigation, determined duplication is INTENTIONAL and correct:
- Maintains clean architecture (use case layer cannot import adapter layer)
- Prevents circular dependencies
- Added comprehensive documentation explaining design decision
- Created sync test `TestIDGenerationMatchesStorePackage` to ensure implementations stay aligned
- Test will fail if implementations accidentally diverge

**Changes**:
- Created shared JSON parsing utilities in http package
- Updated OpenAI, Anthropic, Gemini, Ollama clients
- Removed 4 `extractJSONFromMarkdown` functions
- Simplified all `parseReviewJSON` implementations
- Removed unused regexp imports from all clients
- Added ID generation sync test with 18+ assertions
- Documented clean architecture principles for ID generation
- All 135+ tests passing with zero data races

**Impact**: Zero JSON parsing duplication across LLM clients. Easier maintenance and consistent parsing behavior. Clean architecture integrity documented and protected by tests. Better code clarity and reduced maintenance burden.

**Feedback sources**: OpenAI code review feedback (Oct 22, 2025)

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

### v0.1.1 (Released)
- Production hardening
- Quick wins (magic numbers, SARIF validation, API key format)
- Edge case handling in retry logic
- Code quality improvements
- Zero data races

### v0.1.2 (Released)
- Complete structured logging implementation
- Extended Logger interface with generic methods
- JSON and human-readable log formats
- Full delegation from ReviewLogger to llmhttp.Logger
- 130+ tests passing with zero data races

### v0.1.3 (Current)
- Shared JSON parsing utilities
- Zero code duplication across LLM clients
- ID generation duplication documented as intentional (clean architecture)
- Sync test prevents implementation divergence
- 135+ tests passing with zero data races

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
