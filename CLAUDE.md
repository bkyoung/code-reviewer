# Code Reviewer - Claude Context

**Project:** AI-Powered Code Review Tool
**Status:** Phase 2 In Progress
**Version:** v0.3.0
**Last Updated:** 2025-12-26

---

## Quick Start

1. **First:** Read `docs/PROJECT_RESET_PLAN.md` for MVP scope and phase structure
2. **Build:** `go build -o cr ./cmd/cr`
3. **Test:** `go test ./...`
4. **Run:** `./cr review branch main` (reviews current branch against main)

---

## Current Phase: Phase 2 - GitHub Native

### Phase Status

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1: Foundation | ✅ Complete | Multi-provider LLM, local CLI, basic GitHub workflow |
| **Phase 2: GitHub Native** | 🚧 In Progress | First-class reviewer with inline annotations |
| Phase 3: Production | Planned | Feedback loops, cost visibility, hardening |
| Phase 4: Enterprise | Planned | Multi-platform, org-wide learning |

### Phase 2 Progress

| Milestone | Status |
|-----------|--------|
| 2.1 Inline Annotations | ✅ Complete |
| 2.2 Review API | ✅ Complete |
| 2.3 Request Changes | ✅ Complete |
| 2.4 Skip Trigger | Not Started |
| 2.5 Incremental Reviews | ⏸️ Deferred |
| 2.6 Finding Deduplication | 🚧 Epic #106 |
| 2.7 PR Size Guards | 🚧 PR #113 |
| 2.8 Finding Verification | ✅ Complete |

### v0.3.0 Features

- **Finding Verification:** Agent-based verification with confidence thresholds filters out false positives
- **Fingerprinting:** Stable finding identifiers embedded in GitHub comments (infrastructure for future dedup)

### Current Focus: Deduplication v2 (Epic #106)

**Comment-Based State** - Using GitHub PR comments as the state store instead of separate tracking:

- **#107** Simplified deduplication - check existing comments before posting
- **#108** Status-aware deduplication - detect replies, accurate review status

Key insight: Fingerprints are already embedded in posted comments. We can read PR state instead of maintaining separate tracking.

### Key Documents

- **MVP Scope:** `docs/PROJECT_RESET_PLAN.md`
- **Architecture:** `docs/ARCHITECTURE.md`
- **Security:** `docs/SECURITY.md`

---

## Technology Stack

- **Language:** Go 1.21+
- **Architecture:** Clean Architecture (domain → usecase → adapter)
- **LLM Providers:** OpenAI, Anthropic, Gemini, Ollama
- **Output Formats:** Markdown, JSON, SARIF
- **Persistence:** SQLite
- **Build:** `go build` (Mage available but optional)

---

## Development Essentials

### Build & Test

```bash
# Build
go build -o cr ./cmd/cr

# Test all
go test ./...

# Test with race detector
go test -race ./...

# Format
gofmt -w .

# Lint (if golangci-lint installed)
golangci-lint run
```

### Running Reviews

```bash
# Review current branch against main
./cr review branch main

# Review with output directory
./cr review branch main --output ./review-output

# Review with custom context
./cr review branch main --instructions "Focus on security"
```

### Core Rules

1. **TDD Mandatory:** Write tests first
2. **Clean Architecture:** Domain has no external dependencies
3. **Functional Style:** Prefer immutability, SOLID principles
4. **Format Before Commit:** `gofmt -w .`
5. **All Tests Pass:** `go test ./...` must succeed

### Definition of Done

- [ ] Tests written (TDD)
- [ ] Code formatted (`gofmt`)
- [ ] All tests pass
- [ ] Build succeeds
- [ ] No race conditions (`go test -race`)

---

## Skills (Load Context On-Demand)

Use these skills for targeted context instead of reading docs manually:

| Skill | Use When | Invoke |
|-------|----------|--------|
| **development** | Building, testing, debugging | `/skill development` |
| **github-workflow** | Commits, PRs, issues | `/skill github-workflow` |
| **architecture** | Design questions, understanding codebase | `/skill architecture` |
| **review** | Using the code reviewer itself | `/skill review` |
| **triage-pr-review** | Triage PR code review feedback & SARIF alerts | `/skill triage-pr-review` |

---

## Project Structure

```
cmd/cr/              # CLI entry point
internal/
  adapter/           # External integrations
    cli/             # Command-line interface
    git/             # Git operations
    llm/             # LLM provider clients
      anthropic/
      gemini/
      ollama/
      openai/
    output/          # Output formatters (markdown, json, sarif)
    store/           # SQLite persistence
  config/            # Configuration loading
  domain/            # Core domain types (no dependencies)
  redaction/         # Secret redaction
  usecase/           # Business logic
    merge/           # Multi-provider merge
    review/          # Review orchestration
docs/                # Documentation
security-tests/      # Security test cases
```

---

## Common Pitfalls

1. **Don't** skip tests - TDD is mandatory
2. **Don't** import domain from adapters - clean architecture violation
3. **Don't** commit secrets - redaction exists but prevention is better
4. **Don't** ignore race detector - `go test -race` must pass
5. **Don't** forget to format - `gofmt -w .` before committing

---

## When You're Stuck

**Current work:**
- Project Reset Plan: `docs/PROJECT_RESET_PLAN.md`
- Known Issues: See "Security Testing Findings" in `ROADMAP.md`

**Reference documentation:**
- Architecture: `docs/ARCHITECTURE.md`
- Security: `docs/SECURITY.md`
- GitHub Setup: `docs/GITHUB_ACTION_SETUP.md`

**Historical context:**
- Original roadmap: `ROADMAP.md` (detailed, being archived)
- Session summaries: `docs/session-summaries/`

---

**Remember:** This file provides minimal always-on context. Use GitHub Issues for work tracking. Use skills for deeper, task-specific context.
