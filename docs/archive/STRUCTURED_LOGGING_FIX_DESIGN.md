# Structured Logging Fix - Technical Design

**Version**: v0.1.2
**Date**: 2025-10-22
**Status**: Implementation

## Problem Statement

During v0.1.1 production hardening, we added structured logging infrastructure but didn't fully implement it. The `ReviewLogger` adapter receives an `llmhttp.Logger` but never uses it, instead falling back to unstructured `log.Printf` calls.

**Current behavior:**
```go
func (l *ReviewLogger) LogWarning(ctx context.Context, message string, fields map[string]interface{}) {
    log.Printf("warning: %s %v", message, fields)  // Ignores l.logger!
}
```

**Root cause**: The `llmhttp.Logger` interface only has API-specific methods (LogRequest, LogResponse, LogError), not generic LogWarning/LogInfo methods needed by the orchestrator.

**Impact**:
- Loses structured logging benefits (JSON format, consistent fields)
- Inconsistent log output between LLM clients and orchestrator
- Makes log aggregation and filtering harder in production

**Feedback sources**:
- OpenAI o4-mini review (Oct 22): "You lose structured logging capabilities"
- Anthropic Claude review (Oct 22): "Consider leveraging structured logging more fully"

## Goals

1. ✅ Extend `llmhttp.Logger` interface with generic LogWarning/LogInfo methods
2. ✅ Implement these methods in `DefaultLogger` with JSON and human formats
3. ✅ Update `ReviewLogger` to delegate to the injected logger
4. ✅ Maintain backward compatibility (nil logger still works)
5. ✅ Comprehensive test coverage for both formats
6. ✅ Zero breaking changes to existing code

## Design

### 1. Interface Extension

**File**: `internal/adapter/llm/http/logger.go`

Extend the `Logger` interface:
```go
type Logger interface {
    // API-specific methods (existing)
    LogRequest(ctx context.Context, req RequestLog)
    LogResponse(ctx context.Context, resp ResponseLog)
    LogError(ctx context.Context, err ErrorLog)

    // Generic methods (NEW)
    LogWarning(ctx context.Context, message string, fields map[string]interface{})
    LogInfo(ctx context.Context, message string, fields map[string]interface{})
}
```

### 2. DefaultLogger Implementation

**Format**: Both JSON and human-readable

**JSON format:**
```json
{"level":"warning","timestamp":"2025-10-22T10:30:45Z","message":"failed to save review","runID":"run-123","provider":"openai","error":"database error"}
```

**Human format:**
```
[WARN] 2025-10-22T10:30:45Z failed to save review runID=run-123 provider=openai error=database error
```

**Implementation:**
```go
func (l *DefaultLogger) LogWarning(ctx context.Context, message string, fields map[string]interface{}) {
    if l.level > LogLevelInfo {
        return  // Skip if log level too high
    }

    timestamp := time.Now().UTC()

    if l.format == LogFormatJSON {
        l.logWarningJSON(timestamp, message, fields)
    } else {
        l.logWarningHuman(timestamp, message, fields)
    }
}

func (l *DefaultLogger) LogInfo(ctx context.Context, message string, fields map[string]interface{}) {
    if l.level > LogLevelInfo {
        return
    }

    timestamp := time.Now().UTC()

    if l.format == LogFormatJSON {
        l.logInfoJSON(timestamp, message, fields)
    } else {
        l.logInfoHuman(timestamp, message, fields)
    }
}
```

**Helper functions:**
```go
func (l *DefaultLogger) logWarningJSON(timestamp time.Time, message string, fields map[string]interface{}) {
    // Build JSON object
    logEntry := map[string]interface{}{
        "level":     "warning",
        "timestamp": timestamp.Format(time.RFC3339),
        "message":   message,
    }

    // Merge fields
    for k, v := range fields {
        logEntry[k] = v
    }

    // Marshal and log
    if jsonBytes, err := json.Marshal(logEntry); err == nil {
        log.Println(string(jsonBytes))
    }
}

func (l *DefaultLogger) logWarningHuman(timestamp time.Time, message string, fields map[string]interface{}) {
    // Build key=value pairs
    var pairs []string
    for k, v := range fields {
        pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
    }

    if len(pairs) > 0 {
        log.Printf("[WARN] %s %s %s", timestamp.Format(time.RFC3339), message, strings.Join(pairs, " "))
    } else {
        log.Printf("[WARN] %s %s", timestamp.Format(time.RFC3339), message)
    }
}

// Similar implementations for logInfoJSON and logInfoHuman
```

### 3. ReviewLogger Delegation

**File**: `internal/adapter/observability/logger.go`

Simplify to just delegate:
```go
func (l *ReviewLogger) LogWarning(ctx context.Context, message string, fields map[string]interface{}) {
    l.logger.LogWarning(ctx, message, fields)
}

func (l *ReviewLogger) LogInfo(ctx context.Context, message string, fields map[string]interface{}) {
    l.logger.LogInfo(ctx, message, fields)
}
```

### 4. Backward Compatibility

The orchestrator already has nil checks:
```go
if o.deps.Logger != nil {
    o.deps.Logger.LogWarning(ctx, "failed to save review", map[string]interface{}{
        "runID":    runID,
        "provider": name,
        "error":    err.Error(),
    })
} else {
    log.Printf("warning: failed to save review to store: %v\n", err)
}
```

No changes needed - nil logger fallback remains.

## Testing Strategy

### Unit Tests

**File**: `internal/adapter/llm/http/logger_test.go`

1. **Test LogWarning JSON format**
   - Verify JSON structure
   - Verify all fields present
   - Verify timestamp format

2. **Test LogWarning human format**
   - Verify human-readable output
   - Verify key=value pairs
   - Verify message and timestamp

3. **Test LogInfo JSON format**
4. **Test LogInfo human format**

5. **Test log level filtering**
   - LogLevelError should skip warnings/info
   - LogLevelInfo should log warnings/info
   - LogLevelDebug should log everything

6. **Test with empty fields**
7. **Test with multiple fields**
8. **Test with special characters in values**

**File**: `internal/adapter/observability/logger_test.go`

1. **Test ReviewLogger delegates to injected logger**
   - Create mock logger
   - Verify LogWarning called on mock
   - Verify LogInfo called on mock
   - Verify correct parameters passed

### Integration Tests

**Manual verification**:
1. Run review with `--log-format json` → verify JSON output
2. Run review with `--log-format human` → verify human output
3. Trigger store warnings → verify structured logs appear

## Implementation Plan

### Phase 1: Extend Interface (Red → Green)
**Time**: 30 minutes

1. Add LogWarning/LogInfo to Logger interface (breaks compilation)
2. Add stub implementations to DefaultLogger (fixes compilation)
3. Write failing tests for JSON format
4. Implement logWarningJSON and logInfoJSON
5. Tests pass ✅

### Phase 2: Human Format (Red → Green)
**Time**: 30 minutes

1. Write failing tests for human format
2. Implement logWarningHuman and logInfoHuman
3. Tests pass ✅

### Phase 3: Update ReviewLogger (Green → Green)
**Time**: 15 minutes

1. Update ReviewLogger to delegate
2. Update ReviewLogger tests
3. Tests pass ✅

### Phase 4: Integration Testing (Green → Green)
**Time**: 45 minutes

1. Run full test suite
2. Manual testing with real reviews
3. Verify JSON and human formats
4. Format code
5. Build and verify

### Phase 5: Documentation
**Time**: 15 minutes

1. Update ROADMAP.md
2. Commit changes

**Total estimated time**: 2 hours 15 minutes

## Success Criteria

1. ✅ All existing tests pass
2. ✅ New tests provide comprehensive coverage
3. ✅ JSON format produces valid, parseable JSON
4. ✅ Human format is readable and consistent
5. ✅ Log level filtering works correctly
6. ✅ ReviewLogger delegates to injected logger
7. ✅ Zero breaking changes
8. ✅ Code formatted and linted
9. ✅ Documentation updated

## Non-Goals

- Advanced features (structured context, log sampling, etc.)
- Integration with external logging systems (that's future work)
- Performance optimization (current approach is simple and fast enough)

## Risk Assessment

**Low risk** - This is an enhancement to existing functionality, not a breaking change. The nil logger fallback ensures backward compatibility.

## Rollback Plan

If issues arise, we can:
1. Revert commits
2. Return to v0.1.1 where logging worked (just not optimally)

The change is isolated to logger implementation, minimal blast radius.
