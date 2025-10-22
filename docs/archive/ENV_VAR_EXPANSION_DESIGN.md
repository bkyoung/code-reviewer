# Environment Variable Expansion - Technical Design

**Version**: v0.1.4
**Date**: 2025-10-22
**Status**: Implementation

## Problem Statement

Environment variable expansion (`${VAR}` and `$VAR` syntax) is not consistently applied across all configuration sections. Currently only providers, git, output, and store have env var expansion. The merge, redaction, budget, and observability sections are missing this feature.

## Current State

**File**: `internal/config/loader.go:65-79`

```go
func expandEnvVars(cfg Config) Config {
	// Expand provider API keys
	for name, provider := range cfg.Providers {
		provider.APIKey = expandEnvString(provider.APIKey)
		provider.Model = expandEnvString(provider.Model)
		cfg.Providers[name] = provider
	}

	// Expand other string fields
	cfg.Git.RepositoryDir = expandEnvString(cfg.Git.RepositoryDir)
	cfg.Output.Directory = expandEnvString(cfg.Output.Directory)
	cfg.Store.Path = expandEnvString(cfg.Store.Path)

	return cfg
}
```

### Fields Currently Expanded ✅
- `Providers[].APIKey`
- `Providers[].Model`
- `Git.RepositoryDir`
- `Output.Directory`
- `Store.Path`

### Fields Missing Expansion ❌
- `MergeConfig.Provider` (string)
- `MergeConfig.Model` (string)
- `MergeConfig.Strategy` (string)
- `BudgetConfig.DegradationPolicy` ([]string)
- `RedactionConfig.DenyGlobs` ([]string)
- `RedactionConfig.AllowGlobs` ([]string)
- `ObservabilityConfig.Logging.Level` (string)
- `ObservabilityConfig.Logging.Format` (string)

## Design

### Solution: Comprehensive Env Var Expansion

Update `expandEnvVars` to handle all string and []string fields in the config.

**Updated function**:
```go
func expandEnvVars(cfg Config) Config {
	// Expand provider API keys and models
	for name, provider := range cfg.Providers {
		provider.APIKey = expandEnvString(provider.APIKey)
		provider.Model = expandEnvString(provider.Model)
		cfg.Providers[name] = provider
	}

	// Expand merge config
	cfg.Merge.Provider = expandEnvString(cfg.Merge.Provider)
	cfg.Merge.Model = expandEnvString(cfg.Merge.Model)
	cfg.Merge.Strategy = expandEnvString(cfg.Merge.Strategy)

	// Expand git config
	cfg.Git.RepositoryDir = expandEnvString(cfg.Git.RepositoryDir)

	// Expand output config
	cfg.Output.Directory = expandEnvString(cfg.Output.Directory)

	// Expand budget config
	cfg.Budget.DegradationPolicy = expandEnvStringSlice(cfg.Budget.DegradationPolicy)

	// Expand redaction config
	cfg.Redaction.DenyGlobs = expandEnvStringSlice(cfg.Redaction.DenyGlobs)
	cfg.Redaction.AllowGlobs = expandEnvStringSlice(cfg.Redaction.AllowGlobs)

	// Expand store config
	cfg.Store.Path = expandEnvString(cfg.Store.Path)

	// Expand observability config
	cfg.Observability.Logging.Level = expandEnvString(cfg.Observability.Logging.Level)
	cfg.Observability.Logging.Format = expandEnvString(cfg.Observability.Logging.Format)

	return cfg
}

// expandEnvStringSlice expands environment variables in a slice of strings.
func expandEnvStringSlice(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = expandEnvString(s)
	}
	return result
}
```

## Testing Strategy

### Unit Tests (TDD Approach)

**Test file**: `internal/config/loader_test.go`

1. **Test merge config expansion**
   - Set env vars: `TEST_PROVIDER=openai`, `TEST_MODEL=gpt-4`, `TEST_STRATEGY=consensus`
   - Config: `{merge: {provider: "${TEST_PROVIDER}", model: "${TEST_MODEL}", strategy: "${TEST_STRATEGY}"}}`
   - Verify: Fields are expanded correctly

2. **Test budget degradation policy expansion**
   - Set env vars: `POLICY_1=reduce-providers`, `POLICY_2=reduce-context`
   - Config: `{budget: {degradationPolicy: ["${POLICY_1}", "${POLICY_2}"]}}`
   - Verify: Array elements are expanded

3. **Test redaction globs expansion**
   - Set env vars: `DENY_PATTERN=*.secret`, `ALLOW_PATTERN=public/*`
   - Config: `{redaction: {denyGlobs: ["${DENY_PATTERN}"], allowGlobs: ["${ALLOW_PATTERN}"]}}`
   - Verify: Glob patterns are expanded

4. **Test observability logging expansion**
   - Set env vars: `LOG_LEVEL=debug`, `LOG_FORMAT=json`
   - Config: `{observability: {logging: {level: "${LOG_LEVEL}", format: "${LOG_FORMAT}"}}}`
   - Verify: Logging config is expanded

5. **Test empty slices**
   - Config: `{redaction: {denyGlobs: []}}`
   - Verify: Empty slices are handled correctly (no panic)

6. **Test mixed expansion**
   - Some fields with ${VAR}, some with $VAR, some with plain strings
   - Verify: All syntax variations work correctly

## Implementation Plan (TDD)

### Phase 1: Write Tests (Red Phase)
1. Add `expandEnvStringSlice` tests
2. Add integration tests for each missing config section
3. Run tests → FAIL (expected)

### Phase 2: Implement (Green Phase)
1. Add `expandEnvStringSlice` function
2. Update `expandEnvVars` to handle all missing fields
3. Run tests → PASS

### Phase 3: Refactor (if needed)
1. Consider helper functions if code becomes repetitive
2. Ensure consistency with existing patterns
3. Run tests → PASS

### Phase 4: Verification
1. Run full test suite: `mage test`
2. Run race detector: `go test -race ./...`
3. Format code: `mage format`
4. Build: `mage build`
5. CI: `mage ci`

## Benefits

✅ **Consistency** - All config sections support env var expansion
✅ **Security** - Sensitive values (API keys, paths) can be externalized
✅ **Flexibility** - Users can override any config value via environment
✅ **CI/CD Friendly** - Easy to configure different environments
✅ **Completeness** - No more "missing" sections

## Non-Goals

- NOT changing the expansion syntax (keep ${VAR} and $VAR)
- NOT adding new expansion features (case-insensitive, defaults, etc.)
- NOT breaking existing configurations

## Risks

**Very low risk**:
- Pure addition (no changes to existing expansion logic)
- Backwards compatible (fields that don't use env vars unchanged)
- Comprehensive test coverage will catch regressions

## Rollback Plan

If issues arise:
1. Revert the commit
2. Existing configurations will continue to work as before
3. Only new env var usages would be affected

## Time Estimate

- Write tests: 30 minutes
- Implement expansion: 15 minutes
- Verification: 15 minutes
- Documentation: 15 minutes
- **Total**: ~1-1.5 hours

## Success Criteria

✅ All existing tests pass
✅ New tests for all missing config sections pass
✅ Environment variables expand in merge, budget, redaction, observability
✅ Array/slice expansion works correctly
✅ Zero regressions
✅ CI passes
