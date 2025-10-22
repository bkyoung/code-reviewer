# Structured Logging Fix - Implementation Checklist

**Version**: v0.1.2
**Design**: See STRUCTURED_LOGGING_FIX_DESIGN.md
**Approach**: Test-Driven Development (TDD)

## Phase 1: Extend Interface & Implement JSON Format

### Step 1.1: Extend Logger Interface (Red Phase)
- [ ] Open `internal/adapter/llm/http/logger.go`
- [ ] Add `LogWarning(ctx context.Context, message string, fields map[string]interface{})` to Logger interface
- [ ] Add `LogInfo(ctx context.Context, message string, fields map[string]interface{})` to Logger interface
- [ ] Verify code fails to compile (DefaultLogger doesn't implement new methods)

### Step 1.2: Add Stub Implementations (Fixes Compilation)
- [ ] Add stub `LogWarning` method to DefaultLogger (just empty for now)
- [ ] Add stub `LogInfo` method to DefaultLogger (just empty for now)
- [ ] Verify code compiles: `go build ./...`

### Step 1.3: Write Tests for JSON Format (Red Phase)
- [ ] Create test `TestDefaultLogger_LogWarning_JSON` in `internal/adapter/llm/http/logger_test.go`
- [ ] Test should capture log output and verify JSON structure
- [ ] Test should verify level="warning", message field, custom fields
- [ ] Create test `TestDefaultLogger_LogInfo_JSON`
- [ ] Run tests: `go test ./internal/adapter/llm/http/... -v` → FAIL (expected)

### Step 1.4: Implement JSON Logging (Green Phase)
- [ ] Implement `LogWarning` in DefaultLogger with JSON format support
- [ ] Implement `LogInfo` in DefaultLogger with JSON format support
- [ ] Extract helper functions: `logWarningJSON`, `logInfoJSON`
- [ ] Add imports: `encoding/json`, `strings`, `time`
- [ ] Run tests: `go test ./internal/adapter/llm/http/... -v` → PASS

### Step 1.5: Test Log Level Filtering
- [ ] Add test `TestDefaultLogger_LogWarning_RespectLogLevel`
- [ ] Verify LogLevelError skips warnings
- [ ] Verify LogLevelInfo includes warnings
- [ ] Run tests → PASS

**Checkpoint**: Commit if all tests pass
```bash
git add internal/adapter/llm/http/logger.go internal/adapter/llm/http/logger_test.go
git commit -m "Add LogWarning/LogInfo to Logger interface with JSON format"
```

---

## Phase 2: Implement Human Format

### Step 2.1: Write Tests for Human Format (Red Phase)
- [ ] Create test `TestDefaultLogger_LogWarning_Human` in logger_test.go
- [ ] Test should verify "[WARN]" prefix, timestamp, message, key=value pairs
- [ ] Create test `TestDefaultLogger_LogInfo_Human`
- [ ] Run tests → FAIL (expected)

### Step 2.2: Implement Human Logging (Green Phase)
- [ ] Implement `logWarningHuman` helper function
- [ ] Implement `logInfoHuman` helper function
- [ ] Format as: `[WARN] timestamp message key1=value1 key2=value2`
- [ ] Run tests → PASS

### Step 2.3: Test Edge Cases
- [ ] Add test `TestDefaultLogger_LogWarning_EmptyFields`
- [ ] Add test `TestDefaultLogger_LogWarning_MultipleFields`
- [ ] Add test `TestDefaultLogger_LogWarning_SpecialCharacters`
- [ ] Run tests → PASS

**Checkpoint**: Commit if all tests pass
```bash
git add internal/adapter/llm/http/logger.go internal/adapter/llm/http/logger_test.go
git commit -m "Add human-readable format for LogWarning/LogInfo"
```

---

## Phase 3: Update ReviewLogger

### Step 3.1: Update ReviewLogger Implementation
- [ ] Open `internal/adapter/observability/logger.go`
- [ ] Replace `LogWarning` implementation to delegate: `l.logger.LogWarning(ctx, message, fields)`
- [ ] Replace `LogInfo` implementation to delegate: `l.logger.LogInfo(ctx, message, fields)`
- [ ] Remove `log` import if no longer needed
- [ ] Update comments to reflect delegation

### Step 3.2: Update ReviewLogger Tests
- [ ] Open `internal/adapter/observability/logger_test.go`
- [ ] Tests should now verify structured output from underlying logger
- [ ] Run tests: `go test ./internal/adapter/observability/... -v` → PASS

**Checkpoint**: Commit if all tests pass
```bash
git add internal/adapter/observability/logger.go internal/adapter/observability/logger_test.go
git commit -m "Update ReviewLogger to delegate to injected structured logger"
```

---

## Phase 4: Integration Testing & Verification

### Step 4.1: Run Full Test Suite
- [ ] Run all tests: `mage test`
- [ ] All tests should pass (120+ tests)
- [ ] Verify no regressions

### Step 4.2: Run Race Detector
- [ ] Run: `go test -race ./...`
- [ ] Verify zero data races

### Step 4.3: Format Code
- [ ] Run: `mage format`
- [ ] Verify no formatting changes needed

### Step 4.4: Build Project
- [ ] Run: `mage build`
- [ ] Verify successful build

### Step 4.5: Manual Testing - JSON Format
- [ ] Enable logging in config: `observability.logging.enabled = true`
- [ ] Set format: `observability.logging.format = "json"`
- [ ] Run a review that triggers store warnings (e.g., with store disabled after run created)
- [ ] Verify JSON logs appear: `{"level":"warning","message":"...",...}`
- [ ] Verify JSON is valid and parseable

### Step 4.6: Manual Testing - Human Format
- [ ] Set format: `observability.logging.format = "human"`
- [ ] Run same review
- [ ] Verify human logs appear: `[WARN] 2025-10-22T... message key=value`
- [ ] Verify readable and consistent

**Checkpoint**: Commit if all verification passes
```bash
git add -A
git commit -m "Final integration testing and verification"
```

---

## Phase 5: Documentation

### Step 5.1: Update ROADMAP.md
- [ ] Move "Structured Logging Throughout" from Known Issues to Recently Fixed
- [ ] Add entry to Recently Fixed Issues section:
  ```markdown
  ### ✅ Structured Logging Fix (v0.1.2)
  **Fixed**: 2025-10-22
  **Locations**: internal/adapter/llm/http/logger.go, internal/adapter/observability/logger.go

  Extended llmhttp.Logger interface with generic LogWarning/LogInfo methods.
  ReviewLogger now properly delegates to injected structured logger instead
  of falling back to unstructured log.Printf.

  **Impact**: Consistent structured logging throughout application, better
  log aggregation and filtering in production.
  ```
- [ ] Update current version to v0.1.2

### Step 5.2: Archive Design Documents
- [ ] Move STRUCTURED_LOGGING_FIX_DESIGN.md to docs/archive/
- [ ] Move STRUCTURED_LOGGING_FIX_CHECKLIST.md to docs/archive/
- [ ] Update docs/archive/README.md with new entries

### Step 5.3: Commit Documentation
```bash
git add ROADMAP.md docs/archive/
git commit -m "Update documentation for v0.1.2 structured logging fix"
```

---

## Completion Criteria

### Code Quality
- [ ] All existing tests pass
- [ ] New tests provide comprehensive coverage
- [ ] Zero data races (verified with -race flag)
- [ ] Code formatted (gofmt)
- [ ] Project builds successfully

### Functionality
- [ ] JSON format produces valid, parseable JSON
- [ ] Human format is readable and consistent
- [ ] Log level filtering works correctly
- [ ] ReviewLogger delegates to injected logger
- [ ] Nil logger fallback still works

### Documentation
- [ ] ROADMAP.md updated
- [ ] Design documents archived
- [ ] Commits have clear messages

### Testing
- [ ] Unit tests for JSON format
- [ ] Unit tests for human format
- [ ] Unit tests for log level filtering
- [ ] Unit tests for edge cases
- [ ] Integration tests (manual verification)
- [ ] Race detector clean

---

## Time Tracking

| Phase | Estimated | Actual | Notes |
|-------|-----------|--------|-------|
| Setup & Design | 15 min | | Create design doc, checklist |
| Phase 1: Interface & JSON | 30 min | | Extend interface, implement JSON |
| Phase 2: Human Format | 30 min | | Implement human-readable format |
| Phase 3: ReviewLogger | 15 min | | Update delegation |
| Phase 4: Integration | 45 min | | Testing and verification |
| Phase 5: Documentation | 15 min | | Update docs |
| **Total** | **2h 30m** | | |

---

## Notes

- Follow TDD strictly: Red → Green → Refactor
- Commit after each major phase
- Run tests frequently to catch regressions early
- Keep commits small and focused
- Test both JSON and human formats thoroughly
