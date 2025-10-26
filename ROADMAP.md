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
**Status: Complete** ✅

- [x] Implement circuit breaker pattern for repeated failures
- [x] Add graceful shutdown handling for in-flight requests ✅ (v0.1.6)
- [x] Improve context propagation and cancellation support ✅ (v0.1.6)

## Enhanced Prompting System

**Status**: Complete (All phases 1-5 complete)

See [ENHANCED_PROMPTING_DESIGN.md](docs/ENHANCED_PROMPTING_DESIGN.md) and [ENHANCED_PROMPTING_CHECKLIST.md](docs/ENHANCED_PROMPTING_CHECKLIST.md) for detailed design and tracking.

### Phase 1: Context Gathering ✅
**Status**: Complete

- ✅ `internal/usecase/review/context.go`: Smart context gathering
- ✅ `internal/usecase/review/context_test.go`: Comprehensive tests (13 tests)
- ✅ Change type detection (auth, database, api, security, etc.)
- ✅ Automatic documentation loading (ARCHITECTURE.md, README.md, design docs)
- ✅ Relevant doc discovery based on change types
- ✅ Custom instructions and context files support

### Phase 2: Enhanced Prompt Building ✅
**Status**: Complete

- ✅ `internal/usecase/review/prompt_builder.go`: Template-based system
- ✅ `internal/usecase/review/prompt_builder_test.go`: Template tests (15 tests)
- ✅ Provider-specific prompt templates
- ✅ Context-rich prompts with architecture, docs, and custom instructions
- ✅ Integration test verifying full workflow

### Phase 3: Intelligent Merge (Rule-Based) ✅
**Status**: Complete

- ✅ `internal/usecase/merge/intelligent_merger.go`: Finding similarity and scoring
- ✅ `internal/usecase/merge/intelligent_merger_test.go`: Merge logic tests (8 tests)
- ✅ Finding grouping by similarity (Jaccard distance)
- ✅ Weighted scoring (agreement 40%, severity 30%, precision 20%, evidence 10%)
- ✅ Precision prior support (from database)
- ⚠️  Summary synthesis uses concatenation (not LLM-based yet)

### Phase 3.5: LLM-Based Summary Synthesis 🔄
**Status**: In Progress
**Priority**: High (quick win, dramatically improves merge quality)

- [ ] Add configuration: `merge.useLLM`, `merge.provider`, `merge.model`
- [ ] Update `IntelligentMerger` to support LLM-based synthesis
- [ ] Synthesis prompt: combine all provider summaries into cohesive narrative
- [ ] Graceful fallback to concatenation on LLM failure
- [ ] Tests for LLM synthesis and fallback
- [ ] Wire synthesis provider in main.go
- [ ] Cost: ~$0.0003 per review (negligible)

**Benefits**:
- Cohesive merged summary instead of concatenated fragments
- Identifies agreement and disagreements across providers
- Highlights key themes and critical findings
- Better user experience with merged reviews

### Phase 4: Planning Agent ✅
**Status**: Complete
**Priority**: Medium (interactive mode only)

- ✅ `internal/usecase/review/planner.go`: Planning agent implementation
- ✅ `internal/usecase/review/tty.go`: TTY detection (disabled in CI/CD)
- ✅ Interactive CLI with LLM-powered questions
- ✅ TTY detection (disabled in CI/CD)
- ✅ Wire `--interactive` flag
- ✅ Configuration: `planning.enabled`, `planning.provider`, `planning.model`, `planning.maxQuestions`, `planning.timeout`
- ✅ Graceful degradation on planning failures
- ✅ Comprehensive test coverage (41 planning tests, all passing)
- ✅ Cost: ~$0.001 per review

### Phase 5: Full CLI Integration ✅
**Status**: Complete

- ✅ All CLI flags added and working
- ✅ Context flows through to prompts
- ✅ `--instructions` and `--context` flags fully wired
- ✅ `--no-architecture` and `--no-auto-context` flags fully wired
- ✅ `--interactive` flag wired to planning agent
- ✅ Planning agent fully integrated
- ✅ User documentation updated

## Known Issues & Technical Debt

This section tracks issues identified through code reviews and technical debt items to be addressed in future releases.

### Optional Code Quality Improvements (Low Priority)

Deferred from v0.2.0 code review feedback - these are nice-to-haves that can be addressed if they become problematic:

**1. Observability Setup Duplication**
- **Location**: `cmd/cr/main.go` - Multiple `SetLogger/SetMetrics/SetPricing` calls
- **Severity**: Low (minor code duplication)
- **Defer Reason**: Only 2-3 instances, extraction would add complexity without significant benefit
- **When to Address**: If observability setup appears in 5+ locations

**2. Template-Based Prompt Construction**
- **Location**: `internal/usecase/review/planner.go:234-276`
- **Severity**: Low (maintainability concern)
- **Defer Reason**: Current string concatenation is clear and working well
- **When to Address**: If prompts become significantly more complex or need versioning

All medium and high priority technical debt has been addressed.

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

## Dropped Features

### TUI (Terminal User Interface)
**Status: DROPPED - Pivoted to GitHub-native PR integration**

Originally planned as Phase 3, the TUI feature has been dropped in favor of GitHub PR integration. The decision was made after recognizing that:
- Most code review happens in GitHub/GitLab UI, not the terminal
- SARIF + Code Scanning provides native GitHub UI integration
- GitHub PR comments are more natural for review workflows
- TUI would have limited adoption compared to GitHub integration

**What was planned**:
- Bubble Tea-based interactive review browser
- Finding list/detail views with severity filtering
- Feedback capture ('a' accept, 'r' reject)
- Statistics and precision tracking views

**What we're doing instead**:
- GitHub PR inline comments (v0.3.0)
- Native GitHub Code Scanning integration (Phase 0)
- Feedback via GitHub PR comment reactions
- Statistics via database queries and reports

## Future Features (Deferred)

### Enhanced Redaction

**Status: Deferred - Current regex-based approach working well**

- [ ] Implement entropy-based secret detection
- [ ] Add Shannon entropy calculation
- [ ] Integrate entropy detector into redaction engine
- [ ] Add config options for entropy threshold
- [ ] Combine regex + entropy detection for better coverage

**When to revisit**: If users report secrets leaking through reviews

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

### v0.1.6 (Released)
- Graceful shutdown on SIGINT/SIGTERM
- Tilde expansion for configuration paths
- API key redaction in error messages and logs
- Context cancellation for in-flight HTTP requests
- 187+ tests passing with zero data races

### v0.1.7 (Released)
**Focus: Enhanced Prompting System (Phases 1-3.5)**

- ✅ Phase 1: Context Gathering (complete)
  - Smart document loading (ARCHITECTURE, README, design docs)
  - Change type detection and relevant doc discovery
  - Custom instructions and context files support
- ✅ Phase 2: Enhanced Prompt Building (complete)
  - Template-based prompting system
  - Provider-specific prompt templates
  - Context-rich prompts with all gathered documentation
- ✅ Phase 3: Intelligent Merge - Rule-Based (complete)
  - Finding similarity detection and grouping
  - Weighted scoring algorithm
  - Precision prior support
- ✅ Phase 3.5: LLM-Based Summary Synthesis (complete)
  - Configurable synthesis provider/model
  - Cohesive narrative instead of concatenation
  - Graceful fallback on LLM failure
  - Cost: ~$0.0003 per review

### v0.2.0 (Released)
**Focus: Interactive Mode & Planning Agent**

- ✅ Phase 4: Planning Agent (commit: b8c60e8)
  - Interactive CLI with LLM-powered planning
  - Context analysis and clarifying questions
  - TTY detection (disabled in CI/CD)
  - Wire `--interactive` flag
  - Graceful degradation on failures
  - 41 planning tests, all passing
- ✅ Phase 5: Full CLI Integration
  - All CLI flags implemented and working
  - Planning agent fully integrated
  - User documentation updated
- ✅ Bug Fixes (commits: 08f1105)
  - Fixed planning provider not respecting model configuration
  - Fixed JSON format incompatibility with provider expectations
  - Planning now works correctly with all providers
- ✅ Code Review Feedback Implementation (commits: a51ab90, 30a1ec2)
  - **Phase 1: Refactoring & Tests**
    - Extracted `createPlanningProvider()` function for maintainability
    - Extended multi-provider support (OpenAI, Anthropic, Gemini, Ollama)
    - Added 6 comprehensive unit tests with table-driven approach
    - Reduced code complexity and improved testability
  - **Phase 2: Documentation & Polish**
    - Added detailed workflow comments for planning provider paths
    - Improved error messages with actionable guidance
    - Provider-specific hints (environment variables, configuration)
    - Clarified JSON embedding documentation in planner.go
    - Streamlined prompt documentation to reduce redundancy

**Release Notes**:
- New `--interactive` flag enables LLM-powered clarifying questions
- Planning agent asks 1-5 targeted questions before review
- TTY detection ensures planning only runs in interactive environments
- Multi-provider support for planning (OpenAI, Anthropic, Gemini, Ollama)
- Comprehensive test coverage (234+ total tests, all passing)
- Zero data races verified
- Improved code quality and documentation based on multi-provider code reviews

### Phase 0: Self-Dogfooding via GitHub Actions (In Progress)
**Status: In Progress**
**Priority: Critical - Real-world usage drives v0.3.0 development**

This phase enables immediate self-dogfooding by integrating the code reviewer into this repository's CI/CD:

- ✅ Create `.github/workflows/code-review.yml` workflow
- ✅ Configure workflow to run on every PR to main
- ✅ Generate SARIF output and upload to GitHub Code Scanning
- ✅ Post review summaries as PR comments for Claude Code integration
- ✅ Setup documentation (GITHUB_ACTION_SETUP.md)
- [ ] Test workflow on a practice PR
- [ ] Iterate based on real-world feedback
- [ ] Document learnings and pain points
- [ ] **Optimize**: Generate both formats (SARIF + Markdown) from single review run (50% cost reduction)

**Benefits**:
- Immediate real-world testing of SARIF output quality
- Validates inline annotations work correctly in GitHub UI
- Identifies missing features and UX issues
- Provides cost data for budget planning
- Informs v0.3.0 development priorities

**Setup Requirements**:
- Add `OPENAI_API_KEY` to GitHub repository secrets
- Enable GitHub Code Scanning (free for public repos)
- See [GITHUB_ACTION_SETUP.md](docs/GITHUB_ACTION_SETUP.md) for full instructions

### v0.3.0 (Future - Weeks 2-4)
**Focus: GitHub PR Integration with Inline Comments**

This release transforms the tool from a CLI-first code reviewer into a GitHub-native PR review assistant:

**Core GitHub Integration**:
- [ ] Research GitHub review comments API (create, update, delete)
- [ ] Design findings-to-diff-position mapper algorithm
- [ ] Implement GitHub adapter for inline PR comments
- [ ] Add diff position calculation for multi-line findings
- [ ] Handle edge cases (file renames, binary files, large diffs)

**Deduplication & Persistence**:
- [ ] Implement SQLite + GitHub Actions Cache strategy
- [ ] Design cache key structure (branch, commit, config hash)
- [ ] Add finding deduplication across PR updates
- [ ] Track finding lifecycle (new, updated, resolved, dismissed)
- [ ] Prevent duplicate comments on unchanged code

**Cost Reporting**:
- [ ] Add per-PR cost summary in review comment
- [ ] Track cumulative costs across PR lifecycle
- [ ] Show cost breakdown by provider and operation
- [ ] Add cost estimation before running review

**Documentation & Polish**:
- [ ] Create comprehensive GitHub integration docs
- [ ] Update workflow templates with cache configuration
- [ ] Add troubleshooting guide for common issues
- [ ] Document cost optimization strategies

**Success Criteria**:
- Reviews appear as inline PR comments on specific lines
- Findings deduplicate correctly across PR updates
- Cache persists between PR synchronize events
- Total cost per PR is reasonable ($0.05-$0.50)
- No rate limiting issues with GitHub API

### v0.4.0+ (Long-Term Vision)
**Focus: Org-Wide Learning & Multi-Platform Support**

**Database Evolution**:
- [ ] Add optional Postgres sync for org-wide learning
- [ ] Design hybrid architecture (SQLite + Postgres sync)
- [ ] Implement precision prior aggregation across repos
- [ ] Add org-wide statistics and trending

**Advanced GitHub Features**:
- [ ] Suggested fixes (GitHub's suggestion format)
- [ ] Review thread management (resolve, dismiss, acknowledge)
- [ ] Multi-commit reviews (compare PR branch to base)
- [ ] Differential reviews (only review new changes since last push)

**Multi-Platform Support**:
- [ ] GitLab integration (merge request comments)
- [ ] Bitbucket support (PR comments)
- [ ] Azure DevOps integration
- [ ] Self-hosted Git platforms

**Budget & Cost Control** (from original v0.3.0 plan):
- [ ] Budget enforcement and cost controls
- [ ] Pre-flight cost estimation
- [ ] Degradation policies (reduce providers, reduce context)
- [ ] Hard cap support with graceful rejection

### v1.0.0 (Future)
- Production-ready with battle-tested GitHub integration
- Comprehensive CI/CD integrations (GitHub, GitLab, Bitbucket)
- Org-wide learning with Postgres backend
- Advanced comment management and suggested fixes
- Multi-repository support with shared learning
- Performance optimized for large diffs and monorepos
- Complete documentation and best practices
