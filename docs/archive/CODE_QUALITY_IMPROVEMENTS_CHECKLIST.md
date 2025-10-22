# Code Quality Improvements - Implementation Checklist

**Version**: v0.1.3
**Design**: See CODE_QUALITY_IMPROVEMENTS_DESIGN.md
**Approach**: Test-Driven Development (TDD)

## Phase 1: Create Shared JSON Utilities

### Step 1.1: Write Tests for ExtractJSONFromMarkdown (Red Phase)
- [ ] Create `internal/adapter/llm/http/json_test.go`
- [ ] Test: Extract from ```json code block
- [ ] Test: Extract from ``` code block (no language)
- [ ] Test: Raw JSON (no code block) - should return trimmed text
- [ ] Test: Empty string
- [ ] Test: No JSON (plain text)
- [ ] Test: Multiple code blocks (should take first)
- [ ] Test: Nested/malformed markdown
- [ ] Run tests: `go test ./internal/adapter/llm/http/... -v -run TestExtractJSON` → FAIL

### Step 1.2: Implement ExtractJSONFromMarkdown (Green Phase)
- [ ] Create `internal/adapter/llm/http/json.go`
- [ ] Implement ExtractJSONFromMarkdown function
- [ ] Use compiled regex: `regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")`
- [ ] Handle case where no code block exists (return trimmed input)
- [ ] Run tests → PASS

### Step 1.3: Write Tests for ParseReviewResponse (Red Phase)
- [ ] Test: Valid JSON in ```json code block
- [ ] Test: Valid JSON in ``` code block
- [ ] Test: Raw JSON (no markdown)
- [ ] Test: Invalid JSON - should return error
- [ ] Test: Missing summary field
- [ ] Test: Missing findings field
- [ ] Test: Empty findings array
- [ ] Test: Multiple findings
- [ ] Run tests → FAIL

### Step 1.4: Implement ParseReviewResponse (Green Phase)
- [ ] Implement ParseReviewResponse function
- [ ] Call ExtractJSONFromMarkdown internally
- [ ] Unmarshal to intermediate struct
- [ ] Return summary, findings, error
- [ ] Run tests → PASS

**Checkpoint**: Commit shared utilities
```bash
git add internal/adapter/llm/http/json.go internal/adapter/llm/http/json_test.go
git commit -m "Add shared JSON parsing utilities for LLM clients

Extract common JSON parsing logic to eliminate duplication across
all 4 LLM clients. Provides ExtractJSONFromMarkdown for code block
extraction and ParseReviewResponse for structured parsing.

- ExtractJSONFromMarkdown: Handles ```json and ``` code blocks
- ParseReviewResponse: Parses JSON into summary and findings
- Comprehensive test coverage (15+ tests)
- All tests passing"
```

---

## Phase 2: Update LLM Clients

### Step 2.1: Update OpenAI Client
- [ ] Open `internal/adapter/llm/openai/client.go`
- [ ] Update `parseReviewJSON` to use `http.ParseReviewResponse`
- [ ] Remove `extractJSONFromMarkdown` function (now use shared)
- [ ] Update Response struct creation to use returned values
- [ ] Run OpenAI tests: `go test ./internal/adapter/llm/openai/... -v` → PASS
- [ ] Run all tests to ensure no regressions → PASS

**Checkpoint**: Commit OpenAI update
```bash
git add internal/adapter/llm/openai/client.go
git commit -m "Update OpenAI client to use shared JSON parsing

Replace client-specific parseReviewJSON and extractJSONFromMarkdown
with shared http.ParseReviewResponse. Eliminates code duplication.

- Remove extractJSONFromMarkdown function (72 lines)
- Simplify parseReviewJSON to use shared parsing
- All OpenAI tests passing"
```

### Step 2.2: Update Anthropic Client
- [ ] Open `internal/adapter/llm/anthropic/client.go`
- [ ] Update `parseReviewJSON` to use `http.ParseReviewResponse`
- [ ] Remove regex compilation and extraction logic
- [ ] Run Anthropic tests: `go test ./internal/adapter/llm/anthropic/... -v` → PASS

**Checkpoint**: Commit Anthropic update
```bash
git add internal/adapter/llm/anthropic/client.go
git commit -m "Update Anthropic client to use shared JSON parsing"
```

### Step 2.3: Update Gemini Client
- [ ] Open `internal/adapter/llm/gemini/client.go`
- [ ] Update `parseReviewJSON` to use `http.ParseReviewResponse`
- [ ] Remove extraction logic
- [ ] Run Gemini tests: `go test ./internal/adapter/llm/gemini/... -v` → PASS

**Checkpoint**: Commit Gemini update
```bash
git add internal/adapter/llm/gemini/client.go
git commit -m "Update Gemini client to use shared JSON parsing"
```

### Step 2.4: Update Ollama Client
- [ ] Open `internal/adapter/llm/ollama/client.go`
- [ ] Update `parseReviewJSON` to use `http.ParseReviewResponse`
- [ ] Remove extraction logic
- [ ] Run Ollama tests: `go test ./internal/adapter/llm/ollama/... -v` → PASS

**Checkpoint**: Commit Ollama update
```bash
git add internal/adapter/llm/ollama/client.go
git commit -m "Update Ollama client to use shared JSON parsing"
```

---

## Phase 3: ID Generation Documentation

### Step 3.1: Add Sync Test (Red Phase)
- [ ] Open `internal/usecase/review/store_helpers_test.go`
- [ ] Add import for `internal/store` package
- [ ] Write `TestIDGenerationMatchesStorePackage` test
- [ ] Test generateRunID matches store.GenerateRunID
- [ ] Test generateReviewID matches store.GenerateReviewID
- [ ] Test generateFindingID matches store.GenerateFindingID
- [ ] Add comment explaining why duplication is intentional
- [ ] Run test → SHOULD PASS (implementations are already identical)

### Step 3.2: Document Intentional Duplication
- [ ] Update `internal/usecase/review/store_helpers.go` comments
- [ ] Add detailed comment above generateRunID explaining circular dependency
- [ ] Reference clean architecture principles

**Checkpoint**: Commit ID generation documentation
```bash
git add internal/usecase/review/store_helpers.go internal/usecase/review/store_helpers_test.go
git commit -m "Document intentional ID generation duplication

Add test to ensure ID generation functions in review package stay in sync
with store package implementations. Duplication is intentional to avoid
circular dependencies while maintaining clean architecture.

- Add TestIDGenerationMatchesStorePackage sync test
- Document why duplication exists (clean architecture, no circular deps)
- Test ensures implementations don't accidentally diverge"
```

---

## Phase 4: Final Verification

### Step 4.1: Run Full Test Suite
- [ ] Run all tests: `mage test`
- [ ] All tests should pass (135+ tests)
- [ ] No new test failures

### Step 4.2: Run Race Detector
- [ ] Run: `go test -race ./...`
- [ ] Zero data races

### Step 4.3: Format Code
- [ ] Run: `mage format`
- [ ] Verify formatting

### Step 4.4: Build Project
- [ ] Run: `mage build`
- [ ] Successful build

### Step 4.5: Run CI Suite
- [ ] Run: `mage ci`
- [ ] All checks pass

---

## Phase 5: Documentation

### Step 5.1: Update ROADMAP.md
- [ ] Update current version to v0.1.3
- [ ] Move "Extract Shared JSON Parsing Logic" from Known Issues to Recently Fixed
- [ ] Update "Deduplicate ID Generation" to document it's intentional, not a problem
- [ ] Add entry to Recently Fixed Issues section

### Step 5.2: Archive Design Documents
- [ ] Move CODE_QUALITY_IMPROVEMENTS_DESIGN.md to docs/archive/
- [ ] Move CODE_QUALITY_IMPROVEMENTS_CHECKLIST.md to docs/archive/
- [ ] Update docs/archive/README.md with new entries

### Step 5.3: Commit Documentation
```bash
git add ROADMAP.md docs/archive/
git commit -m "Update documentation for v0.1.3 code quality improvements"
```

---

## Completion Criteria

### Code Quality
- [ ] Zero JSON parsing duplication (shared utilities used by all clients)
- [ ] ID generation duplication documented as intentional
- [ ] Sync test prevents ID generation divergence
- [ ] All tests passing (135+ tests)
- [ ] Zero data races
- [ ] Code formatted
- [ ] Project builds successfully

### Testing
- [ ] 15+ new tests for shared JSON parsing
- [ ] Existing client tests still pass
- [ ] New sync test for ID generation

### Documentation
- [ ] Design document archived
- [ ] Checklist archived
- [ ] ROADMAP.md updated
- [ ] Commits have clear messages

---

## Time Tracking

| Phase | Estimated | Actual | Notes |
|-------|-----------|--------|-------|
| Setup & Design | 30 min | | Create design doc, checklist |
| Phase 1: Shared Utilities | 1 hour | | Tests + implementation |
| Phase 2: Update Clients | 1.5 hours | | 4 clients, one at a time |
| Phase 3: ID Documentation | 30 min | | Test + comments |
| Phase 4: Verification | 15 min | | CI suite |
| Phase 5: Documentation | 15 min | | Update docs |
| **Total** | **3h 30m** | | |

---

## Notes

- Follow TDD strictly: Red → Green → Refactor
- Update one client at a time to minimize risk
- Run tests after each client update
- Commit frequently (after each phase)
- ID generation "duplication" is NOT a bug - it's correct design
