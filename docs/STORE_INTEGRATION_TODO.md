# Store Integration Implementation Checklist

Status: In Progress
Started: 2025-10-21

## Goal

Integrate the SQLite persistence layer with the review orchestrator so that all review runs, provider reviews, and findings are automatically persisted to the database.

## Phase 1: Utility Functions (TDD)

### 1.1 Run ID Generation
- [ ] Write tests for GenerateRunID
  - [ ] Test ID format (run-TIMESTAMP-HASH)
  - [ ] Test uniqueness for same refs at different times
  - [ ] Test sortability by timestamp
  - [ ] Test hash determinism
- [ ] Implement GenerateRunID

### 1.2 Finding Hash Generation
- [ ] Write tests for GenerateFindingHash
  - [ ] Test same finding produces same hash
  - [ ] Test case insensitivity in description
  - [ ] Test whitespace normalization
  - [ ] Test different findings produce different hashes
- [ ] Implement GenerateFindingHash

### 1.3 Review ID Generation
- [ ] Write tests for GenerateReviewID
  - [ ] Test format (review-RUNID-PROVIDER)
  - [ ] Test uniqueness per run+provider
- [ ] Implement GenerateReviewID

### 1.4 Finding ID Generation
- [ ] Write tests for GenerateFindingID
  - [ ] Test format (finding-REVIEWID-INDEX)
  - [ ] Test index padding
- [ ] Implement GenerateFindingID

### 1.5 Config Hash Generation
- [ ] Write tests for CalculateConfigHash
  - [ ] Test determinism (same input = same hash)
  - [ ] Test sensitivity (different input = different hash)
  - [ ] Test all fields included
- [ ] Implement CalculateConfigHash

## Phase 2: Configuration Updates (TDD)

### 2.1 Config Structure
- [ ] Add StoreConfig struct to internal/config
- [ ] Add Store field to Config struct
- [ ] Write tests for config loading with store options

### 2.2 Default Configuration
- [ ] Implement defaultStorePath() for ~/.config/cr/reviews.db
- [ ] Add store defaults to setDefaults()
- [ ] Write tests for defaults

### 2.3 Environment Variables
- [ ] Test CR_STORE_ENABLED override
- [ ] Test CR_STORE_PATH override

## Phase 3: Orchestrator Integration (TDD)

### 3.1 Dependencies Update
- [ ] Add Store to OrchestratorDeps
- [ ] Update validateDependencies (store is optional)
- [ ] Write test for orchestrator with nil store

### 3.2 Helper Methods
- [ ] Write tests for saveReviewToStore
  - [ ] Test review record creation
  - [ ] Test finding records creation
  - [ ] Test finding hash generation
  - [ ] Test error handling
- [ ] Implement saveReviewToStore

### 3.3 Integration Points
- [ ] Write test for run creation before review
  - [ ] Test run ID generation
  - [ ] Test config hash calculation
  - [ ] Test timestamp recording
- [ ] Implement run creation in ReviewBranch()
- [ ] Write test for review saving after provider completes
- [ ] Integrate saveReviewToStore in provider goroutine
- [ ] Write test for merged review saving
- [ ] Integrate saveReviewToStore for merged review

### 3.4 Error Handling
- [ ] Write test for store creation failure (graceful degradation)
- [ ] Write test for save failure (log but continue)
- [ ] Implement warning logs for store failures

## Phase 4: Main Function Updates

### 4.1 Store Initialization
- [ ] Add store initialization in cmd/cr/main.go
- [ ] Add directory creation with error handling
- [ ] Add defer reviewStore.Close()
- [ ] Wire store into orchestrator deps

### 4.2 Config Loading
- [ ] Load store config from file
- [ ] Handle missing config gracefully
- [ ] Test with store enabled
- [ ] Test with store disabled

## Phase 5: Integration Testing

### 5.1 End-to-End Tests
- [ ] Write test: review with store enabled
  - [ ] Verify run created
  - [ ] Verify all provider reviews saved
  - [ ] Verify all findings saved
  - [ ] Verify merged review saved
  - [ ] Verify finding hashes correct
  - [ ] Verify timestamps recorded
- [ ] Write test: review without store (backward compat)
- [ ] Write test: review with store disabled in config
- [ ] Write test: review with store initialization failure

### 5.2 Concurrent Review Tests
- [ ] Write test: multiple concurrent reviews
- [ ] Verify no database locks
- [ ] Verify all runs persisted correctly

## Phase 6: Documentation & Verification

### 6.1 Documentation
- [ ] Update PHASE3_TODO.md with store integration completion
- [ ] Update IMPLEMENTATION_PLAN.md if needed
- [ ] Add example .cr.yaml with store config
- [ ] Document environment variable overrides

### 6.2 Verification
- [ ] Run full test suite (mage ci)
- [ ] Verify all existing tests still pass
- [ ] Verify new tests pass
- [ ] Test with real review (create DB file)
- [ ] Inspect DB with sqlite3 CLI to verify data
- [ ] Test with store disabled (ensure backward compat)

## Acceptance Criteria

- [ ] All utility functions implemented and tested
- [ ] Store configuration working (file + env vars)
- [ ] Reviews automatically persisted when store enabled
- [ ] Reviews work without store (backward compatible)
- [ ] Store failures log warnings but don't break reviews
- [ ] Finding hashes enable de-duplication
- [ ] Run IDs are sortable and unique
- [ ] All tests passing (existing + new)
- [ ] Database created at default path automatically
- [ ] Documentation updated

## Testing Commands

```bash
# Run all tests
mage ci

# Run specific tests
go test ./internal/store/... -v
go test ./internal/usecase/review/... -v
go test ./cmd/cr/... -v

# Test with real review
./cr review branch main --target feature --output ./reviews

# Inspect database
sqlite3 ~/.config/cr/reviews.db
> .tables
> SELECT * FROM runs;
> SELECT * FROM reviews;
> SELECT * FROM findings;
> .quit
```

## Notes

- Store is optional - orchestrator must work without it
- All store operations should fail gracefully with warnings
- Finding hashes use normalized descriptions for better de-duplication
- Run IDs include timestamp for sorting
- Config hash helps track which settings produced which results
- Use in-memory SQLite (:memory:) for all tests
