# Enhanced Prompting System - Implementation Checklist

**Status**: v0.1.7 In Progress
**Last Updated**: 2025-10-26
**Design Document**: [ENHANCED_PROMPTING_DESIGN.md](ENHANCED_PROMPTING_DESIGN.md)

This checklist tracks the implementation of the Enhanced Prompting System across 5 phases.

## Phase 1: Context Gathering ✅ COMPLETE

**Status**: ✅ Complete (implemented 2025-10-26)

### Implementation
- [x] Create `internal/usecase/review/context.go`
  - [x] `ProjectContext` struct with all context fields
  - [x] `ContextGatherer` for rule-based context loading
  - [x] `detectChangeTypes()` - map-based keyword detection
  - [x] `loadFile()` - read documentation files
  - [x] `loadDesignDocs()` - glob pattern matching
  - [x] `findRelevantDocs()` - map change types to docs
- [x] Create `internal/usecase/review/context_test.go`
  - [x] 13 comprehensive test functions
  - [x] Test data directory with sample docs
  - [x] Integration tests with real repository

### Configuration
- [x] Add context gathering config to `internal/config/config.go`
- [x] Support for custom context files
- [x] Support for custom instructions

### Integration
- [x] Wire into `orchestrator.go:ReviewBranch()`
- [x] Add `RepoDir` to `OrchestratorDeps`
- [x] Context gathering before LLM calls

### Verification
- [x] All tests pass (13 context tests + integration)
- [x] `mage format` passes
- [x] `mage ci` passes
- [x] Can gather context from code-reviewer repo itself

---

## Phase 2: Enhanced Prompt Building ✅ COMPLETE

**Status**: ✅ Complete (implemented 2025-10-26)

### Implementation
- [x] Create `internal/usecase/review/prompt_builder.go`
  - [x] `EnhancedPromptBuilder` struct
  - [x] `Build()` method with provider-specific templates
  - [x] `renderTemplate()` using Go text/template
  - [x] `formatDiff()` for diff formatting
  - [x] Provider-specific default templates (OpenAI, Anthropic, Gemini, Ollama)
- [x] Create `internal/usecase/review/prompt_builder_test.go`
  - [x] 15 test functions
  - [x] Template rendering tests
  - [x] Provider-specific format tests
  - [x] Integration test with context gathering

### Template System
- [x] Default template with all context sections
- [x] Provider-specific templates (OpenAI style, Anthropic XML style)
- [x] Template variables: Architecture, README, DesignDocs, RelevantDocs, CustomInstructions, CustomContext, ChangeTypes, ChangedPaths, Diff, BaseRef, TargetRef
- [x] Token budgeting (basic - not fully implemented)

### Integration
- [x] Update `PromptBuilder` signature in `orchestrator.go`
- [x] Wire `EnhancedPromptBuilder` into main.go
- [x] Provider-specific prompts flow to each LLM

### Verification
- [x] All tests pass (15 prompt builder tests)
- [x] Templates render correctly
- [x] Provider-specific formatting works
- [x] `mage ci` passes

---

## Phase 3: Intelligent Merge (Rule-Based) ✅ COMPLETE

**Status**: ✅ Complete (implemented 2025-10-26)

### Implementation
- [x] Create `internal/usecase/merge/intelligent_merger.go`
  - [x] `IntelligentMerger` struct with scoring weights
  - [x] `Merge()` - orchestrates grouping, scoring, ranking
  - [x] `groupSimilarFindings()` - Jaccard similarity
  - [x] `areSimilar()` - file + line overlap + description similarity
  - [x] `scoreGroup()` - weighted scoring algorithm
  - [x] `selectRepresentative()` - choose best finding from group
  - [x] `synthesizeSummary()` - concatenate summaries (rule-based)
- [x] Create `internal/usecase/merge/intelligent_merger_test.go`
  - [x] 8 test functions
  - [x] Finding grouping tests
  - [x] Similarity detection tests
  - [x] Scoring algorithm tests
  - [x] Summary synthesis tests

### Scoring Algorithm
- [x] Agreement weight: 0.4 (how many providers found it)
- [x] Severity weight: 0.3 (average severity score)
- [x] Precision weight: 0.2 (historical precision from store)
- [x] Evidence weight: 0.1 (ratio with evidence)

### Store Integration
- [x] `PrecisionStore` interface
- [x] `GetPrecisionPriors()` method
- [x] Wire store into `IntelligentMerger`
- [x] Add `GetPrecisionPriors()` to `review.Store` interface
- [x] Implement in `store/bridge.go`
- [x] Implement in test mocks

### Integration
- [x] Update `Merger` interface to accept context
- [x] Wire `IntelligentMerger` into main.go
- [x] Replace simple deduplication with intelligent merge

### Verification
- [x] All tests pass (8 merge tests)
- [x] Findings group correctly
- [x] Scoring weights work as expected
- [x] Precision priors influence results
- [x] `mage ci` passes

### Known Limitation
- ⚠️  Summary synthesis uses concatenation, not LLM-based (addressed in Phase 3.5)

---

## Phase 3.5: LLM-Based Summary Synthesis 🔄 IN PROGRESS

**Status**: 🔄 In Progress (started 2025-10-26)

### Configuration
- [ ] Add `merge.useLLM` config field (bool, default: true)
- [ ] Add `merge.provider` config field (string, default: "openai")
- [ ] Add `merge.model` config field (string, default: "gpt-4o-mini")
- [ ] Add config to `internal/config/config.go`
- [ ] Environment variable expansion support

### Implementation
- [ ] Update `internal/usecase/merge/intelligent_merger.go`
  - [ ] Add `synthProvider` field to `IntelligentMerger`
  - [ ] Add `useLLM` config field
  - [ ] Create `buildSynthesisPrompt()` method
  - [ ] Update `synthesizeSummary()` to support LLM calls
  - [ ] Implement graceful fallback to concatenation on error
- [ ] Update `internal/usecase/merge/intelligent_merger_test.go`
  - [ ] Test synthesis prompt generation
  - [ ] Test LLM synthesis with mock provider
  - [ ] Test fallback on LLM failure
  - [ ] Test with useLLM: false (concatenation)

### Synthesis Prompt Design
- [ ] Input: All provider summaries + finding counts
- [ ] Output: Cohesive narrative (200-300 words)
- [ ] Identify: Key themes, agreement, disagreements, critical findings
- [ ] Format: Markdown with sections

### Integration
- [ ] Update main.go to wire synthesis provider
- [ ] Build synthesis provider from config
- [ ] Pass to `IntelligentMerger` constructor
- [ ] Add logging for synthesis operations

### Verification
- [ ] Unit tests pass (synthesis prompt, mock LLM)
- [ ] Integration test with real provider
- [ ] Fallback works correctly
- [ ] Cost is ~$0.0003 per review
- [ ] Merged summary is cohesive narrative
- [ ] `mage format` passes
- [ ] `mage ci` passes

### Documentation
- [ ] Update ENHANCED_PROMPTING_DESIGN.md (mark Phase 3.5 complete)
- [ ] Update ROADMAP.md (mark Phase 3.5 complete)
- [ ] Update this checklist
- [ ] Update CONFIGURATION.md with new merge config fields

---

## Phase 4: Planning Agent ❌ NOT STARTED

**Status**: ❌ Not Started
**Priority**: Medium (interactive mode only)

### Implementation
- [ ] Create `internal/usecase/review/planner.go`
  - [ ] `PlanningAgent` struct
  - [ ] `Plan()` method - analyze context and generate questions
  - [ ] `buildPlanningPrompt()` - context analysis prompt
  - [ ] `parseQuestions()` - extract questions from LLM response
  - [ ] `presentQuestions()` - CLI interaction
  - [ ] `incorporateAnswers()` - update context with user responses
- [ ] Create `internal/usecase/review/planner_test.go`
  - [ ] Prompt generation tests
  - [ ] Question parsing tests
  - [ ] User interaction tests (mock IO)
  - [ ] Integration tests

### Configuration
- [ ] Add `planning.enabled` config field (bool, default: false)
- [ ] Add `planning.provider` config field (string, default: "openai")
- [ ] Add `planning.model` config field (string, default: "gpt-4o-mini")
- [ ] Add `planning.maxQuestions` config field (int, default: 5)
- [ ] Add `planning.timeout` config field (duration, default: "30s")

### CLI Integration
- [ ] TTY detection (check if stdin is terminal)
- [ ] Wire `--interactive` flag to trigger planning
- [ ] Wire `--no-planning` flag to skip planning
- [ ] Remove warning messages from CLI
- [ ] Add planning phase to orchestrator workflow

### Planning Prompt Design
- [ ] Input: ProjectContext + Diff summary
- [ ] Output: JSON with questions array
- [ ] Question types: yes/no, multiple choice, free text
- [ ] Context gaps identification

### Verification
- [ ] Planning agent asks relevant questions
- [ ] User responses update context correctly
- [ ] Works only in TTY mode (disabled in CI/CD)
- [ ] Can be skipped with `--no-planning`
- [ ] Cost is ~$0.001 per review
- [ ] All tests pass
- [ ] `mage ci` passes

---

## Phase 5: Full CLI Integration 🟡 PARTIALLY COMPLETE

**Status**: 🟡 Partially Complete (7/7 flags added, 5/7 fully wired)

### CLI Flags
- [x] `--instructions` - Custom review instructions (fully wired)
- [x] `--context` - Additional context files (fully wired)
- [x] `--interactive` - Interactive mode (stub, warns "not yet implemented")
- [x] `--no-planning` - Skip planning (stub, warns "not yet implemented")
- [x] `--plan-only` - Dry-run mode (stub, warns "not yet implemented")
- [x] `--no-architecture` - Skip ARCHITECTURE.md (fully wired)
- [x] `--no-auto-context` - Disable auto context gathering (fully wired)

### Orchestrator Wiring
- [x] Context gathering respects flags
- [x] Custom instructions flow through
- [x] Context files loaded and included
- [x] `NoArchitecture` flag implemented
- [x] `NoAutoContext` flag implemented
- [ ] Planning agent integration (blocked on Phase 4)
- [ ] `--interactive` flag triggers planning
- [ ] `--no-planning` flag skips planning
- [ ] `--plan-only` shows gathered context without review

### Main.go Wiring
- [x] `EnhancedPromptBuilder` instantiated
- [x] `IntelligentMerger` instantiated
- [ ] Synthesis provider wired (Phase 3.5)
- [ ] Planning provider wired (Phase 4)
- [x] `RepoDir` passed to orchestrator

### Documentation
- [ ] Update user documentation with new flags
- [ ] Add examples for each flag combination
- [ ] Document interactive mode workflow
- [ ] Document planning agent behavior
- [ ] Update CONFIGURATION.md with all new config

### End-to-End Testing
- [ ] Test with all 4 providers in parallel
- [ ] Test with custom instructions
- [ ] Test with context files
- [ ] Test with `--no-architecture`
- [ ] Test with `--no-auto-context`
- [ ] Test interactive mode (after Phase 4)
- [ ] Test in CI/CD environment (non-TTY)

### Verification
- [x] All 7 flags appear in help
- [x] Flags parse correctly
- [x] Context gathering works with flags
- [ ] Planning agent works (Phase 4)
- [ ] End-to-end tests pass
- [x] `mage ci` passes

---

## Overall Progress Summary

### Completed ✅
1. **Phase 1**: Context Gathering (13 tests, all passing)
2. **Phase 2**: Enhanced Prompt Building (15 tests, all passing)
3. **Phase 3**: Intelligent Merge - Rule-Based (8 tests, all passing)
4. **Phase 5**: CLI Integration - Partial (7 flags added, 5 fully wired)

### In Progress 🔄
- **Phase 3.5**: LLM-Based Summary Synthesis (0% complete)

### Not Started ❌
- **Phase 4**: Planning Agent (blocked)
- **Phase 5**: Full CLI Integration (waiting on Phase 4 for interactive flags)

### Test Coverage
- **Current**: 187+ tests passing, zero data races
- **After Phase 3.5**: ~195 tests (estimate: +8 synthesis tests)
- **After Phase 4**: ~210 tests (estimate: +15 planning tests)
- **After Phase 5**: ~220 tests (estimate: +10 integration tests)

### Cost Impact (Estimated)
- **Context Gathering**: Free (rule-based, no LLM calls)
- **Enhanced Prompts**: $0 extra (same review calls, richer context)
- **Intelligent Merge**: Free (rule-based scoring)
- **Summary Synthesis**: ~$0.0003 per review (gpt-4o-mini)
- **Planning Agent**: ~$0.001 per review (interactive mode only)
- **Total Extra Cost**: < $0.002 per review (~1% of typical review cost)

---

## Next Steps (Priority Order)

1. **Phase 3.5: LLM-Based Summary Synthesis** (HIGH PRIORITY - Quick Win)
   - Write tests for synthesis prompt generation
   - Implement LLM call in `synthesizeSummary()`
   - Add configuration fields
   - Wire synthesis provider in main.go
   - Verify cost and quality improvement

2. **Phase 5: Documentation** (MEDIUM PRIORITY)
   - Update user documentation with new flags
   - Add configuration examples
   - Document model selection strategy

3. **Phase 4: Planning Agent** (MEDIUM PRIORITY - Interactive Only)
   - Design planning prompt
   - Implement question parsing
   - Add TTY detection
   - Wire interactive flags

4. **Phase 5: Full Integration** (LOW PRIORITY - Blocked on Phase 4)
   - Complete interactive mode wiring
   - End-to-end testing
   - Performance optimization
