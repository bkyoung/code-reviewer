# Orchestrator Store Integration Checklist

Status: In Progress
Date: 2025-10-21

## Goal
Wire the SQLite store into the review orchestrator so all reviews, findings, and run metadata are automatically persisted when the store is enabled.

## Implementation Steps

### Phase 1: Dependencies & Helpers (TDD)

#### 1. Update OrchestratorDeps
- [x] Add Store field to OrchestratorDeps
- [ ] Update validateDependencies to allow nil Store (optional)
- [ ] Write test verifying orchestrator works with nil Store

#### 2. Helper Method: saveReviewToStore
- [ ] Write test: saves review record correctly
- [ ] Write test: saves all findings with correct IDs
- [ ] Write test: generates finding hashes correctly
- [ ] Write test: handles empty findings list
- [ ] Write test: returns error on save failure
- [ ] Implement saveReviewToStore method

#### 3. Helper Method: calculateConfigHash
- [ ] Write test: deterministic hash from BranchRequest
- [ ] Write test: different requests produce different hashes
- [ ] Implement calculateConfigHash helper in orchestrator

### Phase 2: Integration Points

#### 4. Run Creation
- [ ] Write test: creates run record before review starts
- [ ] Write test: run has correct metadata (refs, repo, timestamp)
- [ ] Write test: run ID is generated correctly
- [ ] Write test: continues if store is nil
- [ ] Implement run creation in ReviewBranch

#### 5. Review Persistence
- [ ] Write test: saves each provider review after completion
- [ ] Write test: saves merged review at end
- [ ] Write test: logs warning on save failure but continues
- [ ] Integrate saveReviewToStore calls in provider goroutines
- [ ] Integrate saveReviewToStore for merged review

#### 6. Error Handling
- [ ] Write test: store creation failure logs warning
- [ ] Write test: save failure doesn't break review
- [ ] Write test: nil store doesn't cause panics
- [ ] Implement graceful degradation

### Phase 3: Main Function Integration

#### 7. Store Initialization
- [ ] Check store enabled in config
- [ ] Create store directory if needed
- [ ] Initialize SQLite store with config path
- [ ] Handle initialization errors gracefully
- [ ] Add defer store.Close()

#### 8. Wire Store into Orchestrator
- [ ] Pass store to OrchestratorDeps
- [ ] Test with store enabled
- [ ] Test with store disabled

### Phase 4: Integration Testing

#### 9. End-to-End Tests
- [ ] Test: run review, verify run saved to DB
- [ ] Test: verify all provider reviews saved
- [ ] Test: verify all findings saved with correct hashes
- [ ] Test: verify merged review saved
- [ ] Test: verify timestamps recorded correctly
- [ ] Test: verify config hash calculated

#### 10. Backward Compatibility
- [ ] Test: review works with store disabled
- [ ] Test: review works with nil store
- [ ] Test: existing tests still pass

### Phase 5: Documentation & Verification

#### 11. Documentation
- [ ] Update STORE_INTEGRATION_TODO.md progress
- [ ] Add example configuration with store settings
- [ ] Document environment variables

#### 12. Manual Testing
- [ ] Run real review with store enabled
- [ ] Inspect database with sqlite3
- [ ] Verify data structure
- [ ] Test with store disabled

#### 13. CI/CD
- [ ] Run mage ci
- [ ] All tests passing
- [ ] No regressions

## Acceptance Criteria

- [ ] Store integrated into orchestrator
- [ ] Reviews automatically persisted when enabled
- [ ] Store failures don't break reviews
- [ ] Backward compatible (works without store)
- [ ] All tests passing
- [ ] Database created at default path
- [ ] Data structure correct in DB
- [ ] Documentation updated
- [ ] Code formatted and linted

## Notes

- Store is optional - must work without it
- All store operations should fail gracefully
- Use in-memory SQLite for tests
- Finding hashes enable de-duplication
- Run IDs are sortable by timestamp
