# Code Reviewer Roadmap

## Current Status

**v0.1.6 - Security & Reliability Improvements** ✅

The code reviewer now has:
- ✅ Multi-provider LLM support (OpenAI, Anthropic, Gemini, Ollama)
- ✅ Full HTTP client implementation with retry logic and error handling
- ✅ Comprehensive observability (logging, metrics, cost tracking)
- ✅ True structured logging - JSON and human-readable formats throughout
- ✅ Shared JSON parsing utilities - Zero duplication across LLM clients
- ✅ **Complete environment variable expansion** - All config sections supported
- ✅ **Configurable HTTP settings** - Global and per-provider timeout, retry, backoff config
- ✅ **Graceful shutdown** - SIGINT/SIGTERM cancels in-flight requests promptly
- ✅ **API key protection** - URL secrets redacted from all error outputs
- ✅ **Shell path expansion** - Tilde (~) expansion for home directory paths
- ✅ SQLite-based review persistence
- ✅ Multiple output formats (Markdown, JSON, SARIF)
- ✅ Configuration system with full env var support (${VAR} and $VAR syntax)
- ✅ Secret redaction
- ✅ Deterministic reviews for CI/CD
- ✅ Production-ready retry logic with edge case handling
- ✅ Clean architecture integrity - Intentional duplication documented
- ✅ All unit and integration tests passing (187+ tests)
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
**Status: Complete** ✅

- ✅ Add `http.timeout` config option (default: 60s)
- ✅ Add `http.maxRetries` config option (default: 5)
- ✅ Add `http.initialBackoff` and `http.maxBackoff` config options
- ✅ Add `http.backoffMultiplier` config option
- ✅ Add provider-specific timeout, retry, and backoff overrides
- ✅ Environment variable expansion for all HTTP config fields

### 3. Resilience Features
**Priority: Low**

- [ ] Implement circuit breaker pattern for repeated failures
- [ ] Add graceful shutdown handling for in-flight requests
- [ ] Improve context propagation and cancellation support

## Known Issues & Technical Debt

This section tracks issues identified through code reviews and technical debt items to be addressed in future releases.

**No medium or high priority technical debt items remaining.** 🎉

All known code quality issues and configuration inconsistencies have been addressed through v0.1.1-v0.1.4.

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

### ✅ Complete Environment Variable Expansion (v0.1.4)
**Fixed**: 2025-10-22
**Location**: `internal/config/loader.go`
**Severity**: MEDIUM (incomplete feature)

**Problem**:
Environment variable expansion (`${VAR}` and `$VAR` syntax) was only applied to providers, git, output, and store config sections. The merge, budget, redaction, and observability sections did not support env var expansion, creating inconsistent behavior and limiting CI/CD flexibility.

**Solution**:
- Created `expandEnvStringSlice` function for array/slice expansion
- Updated `expandEnvVars` to handle all missing string fields:
  - Merge config: provider, model, strategy
  - Budget config: degradationPolicy ([]string)
  - Redaction config: denyGlobs, allowGlobs ([]string)
  - Observability config: logging.level, logging.format
- Comprehensive test coverage (6 new test functions, 20+ assertions)

**Changes**:
- Add expandEnvStringSlice helper function
- Expand merge.provider, merge.model, merge.strategy
- Expand budget.degradationPolicy array
- Expand redaction.denyGlobs and redaction.allowGlobs arrays
- Expand observability.logging.level and observability.logging.format
- All 140+ tests passing with zero data races

**Impact**: Complete and consistent environment variable support across all configuration sections. Users can now externalize any config value via environment variables, making the tool fully CI/CD ready with flexible environment-specific configurations.

**Feedback source**: OpenAI code review feedback (Oct 22, 2025)

### ✅ Gemini JSON Parsing Fix (v0.1.5 bugfix)
**Fixed**: 2025-10-22
**Locations**: `internal/adapter/llm/gemini/client.go`, `internal/adapter/llm/http/json.go`, `internal/adapter/llm/http/json_test.go`
**Severity**: HIGH (Gemini returned empty findings in all manual tests)

**Problem 1: Missing System Instruction**
Gemini API client had no system instruction telling it to return JSON format. Unlike OpenAI and Anthropic which have implicit JSON formatting, Gemini requires explicit instructions in the `systemInstruction` field.

**Problem 2: Nested Code Blocks in Suggestions**
The shared JSON extraction regex used non-greedy matching `([\\s\\S]*?)` which stopped at the FIRST closing backticks. When Gemini suggestions contained nested code blocks (e.g., "Use this code:\\n\\n```go\\nfunc main() {}\\n```"), the regex truncated the JSON at the nested closing backticks instead of the outer ones, causing "unexpected end of JSON input" errors.

**Solution**:
1. Added `SystemInstruction` field to `GenerateContentRequest` struct
2. Added explicit JSON schema instruction to Gemini client with example format
3. Changed regex from non-greedy `([\\s\\S]*?)` to greedy `([\\s\\S]*)` to match to LAST backticks
4. Added comprehensive warning logging when JSON parsing fails (logs full response for debugging)
5. Added test `TestExtractJSONFromMarkdown_NestedCodeBlocks` to verify fix
6. Updated `TestExtractJSONFromMarkdown_MultipleCodeBlocks` expectations

**Changes**:
- Added SystemInstruction field to gemini/types.go
- Updated Gemini client Call method with explicit JSON format instruction
- Changed jsonBlockRegex from non-greedy to greedy matching
- Added warning log with full response text when parsing fails
- Updated test expectations for greedy matching behavior
- All 160+ tests passing with zero data races

**Impact**: Gemini now returns structured findings like other providers. The greedy regex correctly handles real-world LLM responses where suggestions contain nested code blocks. Better debugging capability through comprehensive logging when parsing fails.

**Discovery**: Manual testing by user after v0.1.5 release revealed Gemini consistently returned empty findings while other providers worked correctly.

### ✅ Gemini Extended Thinking Token Exhaustion (v0.1.5 bugfix)
**Fixed**: 2025-10-22
**Locations**: `internal/usecase/review/prompt.go`, `internal/adapter/llm/gemini/client.go`
**Severity**: CRITICAL (Gemini 2.5 Pro returned 0 output tokens, empty responses)

**Problem**: After fixing JSON parsing, Gemini 2.5 Pro still returned empty responses. Investigation revealed:
- `finishReason: "MAX_TOKENS"` - Hit token limit before generating output
- `thoughtsTokenCount: 4095` - Used almost all 4096 tokens for internal reasoning
- `tokensOut: 0` - No tokens left for actual response
- `content.parts: []` - Empty parts array (no text generated)

Gemini 2.5 Pro has **extended thinking** capabilities (like OpenAI o1/o4) where the model uses tokens for internal reasoning before generating the response. With `defaultMaxTokens = 4096`, Gemini exhausted its token budget on thinking alone.

**Solution**:
1. Increased `defaultMaxTokens` from 4096 to **16384** to accommodate extended thinking models
2. Added enhanced logging when Gemini returns empty responses (logs full API response body)
3. Extracted Gemini system instruction to named constant `systemInstruction` for maintainability
4. Documented extended thinking requirements in code comments

**Changes**:
- Update defaultMaxTokens with detailed comment explaining extended thinking needs
- Add empty response warning log with finishReason, tokensOut, and raw response body
- Extract system instruction to package-level constant
- All 160+ tests passing with zero data races

**Impact**: Gemini 2.5 Pro can now both think (up to ~4k tokens) and generate complete JSON responses (up to ~12k tokens). The increased limit remains compatible with all other providers (Claude Sonnet supports up to 8k output tokens).

**Discovery**: Manual testing after JSON parsing fix revealed Gemini still returned empty responses. Log analysis showed MAX_TOKENS finish reason with 4095 tokens used for thinking.

### ✅ Code Quality Improvements from LLM Review Feedback (v0.1.5)
**Fixed**: 2025-10-22
**Locations**: `internal/adapter/llm/http/json.go`, `internal/adapter/llm/http/config_helpers_test.go`, `.gitignore`
**Severity**: MEDIUM (code quality and test coverage)

**Issues Identified by LLM Code Review**:
1. **ExtractJSONFromMarkdown lacks documentation** - Greedy matching behavior not documented
2. **config_helpers.go lacks unit tests** - No dedicated tests for ParseTimeout and BuildRetryConfig edge cases
3. **Database file in version control** - `~/.config/cr/reviews.db` was committed (security/merge risk)

**Solution**:
1. **Enhanced GoDoc for ExtractJSONFromMarkdown**:
   - Document greedy matching behavior (first to LAST backticks)
   - Explain why greedy matching needed for nested code blocks
   - Document assumption that LLMs return single JSON blocks
   - Clarify trade-offs of greedy approach

2. **Added 15 comprehensive unit tests for config_helpers.go**:
   - `ParseTimeout`: Test all fallback chain paths (provider > global > default)
   - Edge cases: invalid durations, nil pointers, empty strings, zero/negative values
   - `BuildRetryConfig`: Test all configuration precedence scenarios
   - Mixed overrides and fallbacks

3. **Database file cleanup**:
   - Removed `~/.config/cr/reviews.db` from version control
   - Added `*.db`, `*.sqlite`, `*.sqlite3` to `.gitignore`

**Changes**:
- Comprehensive GoDoc with examples for ExtractJSONFromMarkdown
- 15 new unit tests in config_helpers_test.go
- Updated .gitignore to prevent database files
- All 175+ tests passing with zero data races

**Impact**: Better code documentation for maintainability. Comprehensive test coverage for configuration helpers ensures fallback chains work correctly. Database files can no longer be accidentally committed.

**Feedback sources**: Anthropic, OpenAI, and Gemini code reviews (Oct 22, 2025) identified these improvements during self-review of v0.1.5 changes.

### ✅ OpenAI Reasoning Model Support and Code Review Remediation (v0.1.5)
**Fixed**: 2025-10-22
**Locations**: Multiple files across codebase
**Severity**: CRITICAL (OpenAI o3/o4 models failing) + MEDIUM (security and code quality)

This work included three phases of fixes based on comprehensive code review feedback from all three LLM providers:

#### Phase 1: Critical Security & Bug Fixes

**1. Response Logging Security Issue**
**Location**: `internal/adapter/llm/gemini/client.go`, new file `internal/adapter/llm/http/logging.go`
**Severity**: HIGH (security/privacy risk)

**Problem**: Gemini client was logging full API responses (including user source code and potentially secrets) to log aggregators without truncation.

**Solution**:
- Created `internal/adapter/llm/http/logging.go` with truncation utilities
- `TruncateForLogging()` limits logged responses to 200 characters max
- `SafeLogResponse()` wrapper for safe logging throughout codebase
- Updated Gemini client to use SafeLogResponse() at two logging locations
- Comprehensive test coverage (6 new tests)

**Impact**: Prevents accidental exposure of sensitive data (source code, API keys, secrets) in production logs and log aggregators.

**2. Negative Duration Validation**
**Location**: `internal/adapter/llm/http/config_helpers.go`
**Severity**: CRITICAL (runtime panic risk)

**Problem**: `ParseTimeout` and `parseDuration` accepted negative duration values which cause runtime panics when set on `http.Client.Timeout`.

**Solution**:
- Added `d >= 0` validation to ParseTimeout and parseDuration
- Added safe fallbacks (60s for timeout, 2s for backoff) if defaultVal is somehow negative
- Comprehensive test coverage (3 new tests for negative value handling)

**Impact**: Application now gracefully handles invalid config instead of crashing with runtime panics.

**3. Provider-Specific Token Limits**
**Location**: `internal/usecase/review/prompt.go`
**Severity**: HIGH (HTTP 400 errors from providers)

**Problem**: `defaultMaxTokens = 16384` exceeded limits for some providers:
- Claude Sonnet: max 8k output tokens
- GPT-4-turbo: max 4k-16k depending on variant
- Caused HTTP 400 errors with "invalid request" messages

**Solution**:
- Lowered defaultMaxTokens from 16384 → **8192** (safe across all providers)
- Added extensive documentation explaining:
  - Why 8k is safe across all providers
  - Extended thinking models (o1/o3/o4, Gemini 2.5 Pro) use tokens for reasoning
  - How to handle MAX_TOKENS errors (higher limits, custom config, smaller diffs)
  - Trade-offs and recommendations

**Impact**: Works reliably across all providers while still supporting substantial code reviews. Users can configure higher limits for providers that support them.

**4. OpenAI o3/o4 Reasoning Model Support**
**Location**: `internal/adapter/llm/openai/client.go`
**Severity**: CRITICAL (o3/o4 models failing with HTTP 400)

**Problem**: OpenAI o3 and o4 reasoning models use `max_completion_tokens` instead of `max_tokens` (like o1). Code only detected o1 models, causing o3/o4 to fail with "Unsupported parameter: 'max_tokens'" errors.

**Solution**:
- Updated `isO1Model()` to detect o1, o3, and o4 model families
- Changed implementation from repetitive boolean logic to loop-based approach
- Uses `reasoningModelFamilies := []string{"o1", "o3", "o4"}` for maintainability
- Comprehensive test coverage for all model variants

**Impact**: All OpenAI reasoning models (o1, o3, o4 and their variants like o3-mini, o4-mini) now work correctly.

#### Phase 2: Code Quality Improvements

**5. Import Alias Shadowing**
**Location**: `internal/adapter/llm/http/config_helpers_test.go`
**Severity**: LOW (code clarity)

**Problem**: Import alias `http` shadowed standard library `net/http`, potentially confusing readers.

**Solution**:
- Renamed import from `http` to `llmhttp` for consistency
- Used sed to replace all `http.` references with `llmhttp.` in test file
- Maintains consistency with other test files in codebase

**Impact**: More readable test code, prevents confusion with standard library.

**6. Refactor isO1Model**
**Location**: `internal/adapter/llm/openai/client.go`
**Severity**: LOW (maintainability)

**Problem**: Repetitive boolean logic for o1/o3/o4 checks made it hard to add new model families.

**Solution**:
- Replaced repetitive boolean with loop-based approach
- Uses `reasoningModelFamilies` array for extensibility
- Easier to add new model families (o2, o5, etc.) in the future
- More readable and follows DRY principle

**Impact**: Improved maintainability, easier to extend for future reasoning models.

#### Summary

**Changes**:
- Created logging.go with safe truncation utilities (6 tests)
- Fixed negative duration validation (3 tests)
- Adjusted token limits with comprehensive documentation
- Added o3/o4 reasoning model support
- Fixed import alias shadowing
- Refactored isO1Model for maintainability
- All 180+ tests passing with zero data races

**Commits**:
- 44ee86c: "Fix critical security and performance issues from code review"
- 84849e6: "Improve code quality and maintainability"

**Impact**: Critical security fix prevents data leakage. All OpenAI reasoning models work correctly. Safer configuration handling prevents runtime panics. Better token limit defaults work across all providers. Improved code quality and maintainability.

**Feedback sources**: Comprehensive code reviews from OpenAI o3, Anthropic Claude, and Gemini 2.5 Pro identified these issues and recommendations (Oct 22, 2025).

### ✅ Security & Reliability Improvements (v0.1.6)
**Fixed**: 2025-10-22
**Locations**: `cmd/cr/main.go`, `internal/config/loader.go`, `internal/adapter/llm/http/logging.go`, `internal/adapter/llm/http/logger.go`
**Severity**: HIGH (security + reliability)

This release addresses three critical bugs discovered during manual testing:

#### 1. Graceful Shutdown on SIGINT/SIGTERM
**Location**: `cmd/cr/main.go:45-47`
**Severity**: HIGH (resource leaks, orphaned goroutines)

**Problem**: When users pressed CTRL+C during long-running reviews, the main process exited but goroutines making HTTP requests to LLM providers continued running in the background. This caused:
- Resource leaks (open connections, goroutines)
- Wasted API costs (requests completing after user cancellation)
- Inability to interrupt slow/stuck operations

**Root Cause**: Application used `context.Background()` which never gets cancelled, so goroutines had no signal to abort when the process received SIGINT.

**Solution**:
- Replaced `context.Background()` with `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`
- Context now automatically cancels when SIGINT or SIGTERM received
- All goroutines detect cancellation via `ctx.Done()` and abort promptly
- HTTP clients respect context cancellation and stop in-flight requests
- Comprehensive test coverage (`TestOrchestrator_ContextCancellation`)

**Impact**: Reviews can now be safely interrupted with CTRL+C. All goroutines and HTTP requests abort within 2 seconds of cancellation signal. No more orphaned processes or wasted API calls.

#### 2. Tilde Expansion for Database Paths
**Location**: `internal/config/loader.go:123-141`
**Severity**: MEDIUM (incorrect file locations)

**Problem**: Configuration paths like `~/.config/cr/reviews.db` were interpreted literally as `$REPO_ROOT/~/.config/cr/reviews.db`, creating a directory named `~` in the repository root instead of expanding to the user's home directory.

**Root Cause**: The `expandEnvString()` function only handled `${VAR}` and `$VAR` syntax but didn't implement shell-style tilde expansion.

**Solution**:
- Added tilde expansion to `expandEnvString()` following shell conventions
- Only expands `~` when it appears at the start of the path
- Handles `~` (home dir), `~/` (home dir with slash), and `~/path` (home dir + path)
- Uses `os.UserHomeDir()` for cross-platform compatibility
- Special handling for `~/` to preserve trailing slash
- Comprehensive test coverage (7 new tests for tilde expansion)

**Impact**: Database files and other configured paths now correctly resolve to user's home directory. No more accidental creation of `~` directories in repository roots.

#### 3. API Key Redaction in Error Messages
**Location**: `internal/adapter/llm/http/logging.go:52-92`, `internal/adapter/llm/http/logger.go:149-150`, `cmd/cr/main.go:39`
**Severity**: HIGH (security - API key exposure)

**Problem**: When Gemini API requests failed (e.g., timeout, cancellation), error messages contained full URLs with API keys visible in query parameters like `?key=AIzaSyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`. API keys appeared in TWO locations:
1. Structured logger output (DefaultLogger.LogError)
2. Main error output (main function's log.Println)

**Root Cause**: Go's HTTP client includes full request URLs in error messages. Gemini uses API keys as query parameters instead of headers, causing keys to appear in error text. No sanitization was applied before logging.

**Solution**:
- Created `RedactURLSecrets()` function in `logging.go` with regex-based redaction
- Redacts common secret patterns: `key=`, `apiKey=`, `api_key=`, `token=`, `access_token=`
- Applied redaction in TWO locations to prevent dual exposure:
  - DefaultLogger.LogError: Redacts before structured logging
  - main.go error output: Redacts before terminal output
- Preserves error context (domain, endpoint, error type) while hiding secrets
- Comprehensive test coverage (6 new tests for URL redaction)

**Impact**: API keys can no longer leak through error messages or logs. Both terminal output and structured logs show `key=[REDACTED]` instead of actual keys. Users can still debug errors (domain, endpoint visible) without exposing credentials.

#### Summary

**Changes**:
- Added signal handling for graceful shutdown (1 test)
- Implemented tilde expansion for shell-style paths (7 tests)
- Created URL secret redaction utilities (6 tests)
- Applied redaction at both logger and main error output levels
- All 187+ tests passing with zero data races

**Commits**:
- 4c7dfb5: "Add graceful shutdown on SIGINT/SIGTERM"
- 6551a81: "Fix tilde expansion and add API key redaction to logger"
- 0fd8ca8: "Fix API key exposure in main error output"

**Impact**:
- **Security**: API keys can no longer leak through error messages
- **Reliability**: CTRL+C properly cancels all in-flight operations
- **Correctness**: Configuration paths expand correctly using shell conventions
- **User Experience**: Graceful interruption, no orphaned processes, proper file locations

**Discovery**: All three bugs found during manual testing after v0.1.5 release. User reported CTRL+C not working, database created in wrong location, and API keys visible in Gemini error messages.

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

### v0.1.3 (Released)
- Shared JSON parsing utilities
- Zero code duplication across LLM clients
- ID generation duplication documented as intentional (clean architecture)
- Sync test prevents implementation divergence
- 135+ tests passing with zero data races

### v0.1.4 (Released)
- Complete environment variable expansion
- Support for all config sections (merge, budget, redaction, observability)
- Array/slice env var expansion (expandEnvStringSlice)
- Comprehensive test coverage (6 new tests, 20+ assertions)
- 140+ tests passing with zero data races

### v0.1.5 (Released)
- Configurable HTTP settings (timeout, retries, backoff)
- Global HTTP config with per-provider overrides
- Environment variable expansion for HTTP config
- Module path correction (github.com/bkyoung/code-reviewer)
- OpenAI o3/o4 reasoning model support
- Security fixes (log truncation prevents data leakage)
- Negative duration validation (prevents runtime panics)
- Token limit adjustments (8k default for cross-provider compatibility)
- Code quality improvements (import alias fix, isO1Model refactor)
- 180+ tests passing with zero data races

### v0.1.6 (Current)
- Graceful shutdown on SIGINT/SIGTERM
- Tilde expansion for configuration paths
- API key redaction in error messages and logs
- Context cancellation for in-flight HTTP requests
- 187+ tests passing with zero data races

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
