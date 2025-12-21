# Code Reviewer Project Reset Plan

**Date:** 2025-12-21
**Purpose:** Reset project infrastructure to align with mature development practices

---

## 1. MVP Definition

### Vision Statement

**code-reviewer** is an AI-powered code review tool with two primary modes:

1. **GitHub PR Integration (Primary)** - First-class automated reviewer
2. **Local CLI (Secondary)** - Standalone review to local directory

### MVP Scope

#### Mode 1: GitHub PR Integration

| Feature | Description | Priority |
|---------|-------------|----------|
| Automated Review on Push | Trigger on push unless `[skip code-review]` present | Must Have |
| Inline Annotations | Comments on specific source lines, not just PR comments | Must Have |
| Summary Comment | High-level review summary as PR comment | Must Have |
| First-Class Reviewer | Bot initiates actual GitHub code review (not just comments) | Must Have |
| Request Changes | Block merges for configurable high-priority issues | Must Have |
| Incremental Reviews | Only review new changes since last review, not entire PR | Must Have |
| Finding Deduplication | Track findings across PR updates, don't re-flag same issues | Must Have |
| PR Size Guards | Warn/handle gracefully when PR exceeds context limits | Must Have |
| Configurable Rules | Security vulns, failing builds/tests, custom repo rules | Should Have |

#### Mode 2: Local CLI

| Feature | Description | Priority |
|---------|-------------|----------|
| Local Review | Run against local repo | Must Have |
| File Output | Write review to local directory | Must Have |
| Multiple Formats | Markdown, JSON, SARIF | Must Have |

### What's NOT in MVP (Defer to Later Phases)

**Defer to v0.4.x (Phase 3):**
- Feedback learning from accept/dismiss actions
- Auto-fix suggestions (GitHub suggestion format)
- Review templates (feature/bugfix/security)
- Cost visibility / pre-review estimates
- Comment threading (reply to existing)
- Enhanced secret detection (entropy-based)

**Defer to v1.0+ (Phase 4):**
- Multi-platform support (GitLab, Bitbucket, Azure DevOps)
- Org-wide learning / Postgres sync
- Custom rules engine
- Review metrics dashboard
- Reviewer personas (security-focused, performance-focused)
- Streaming responses

---

## 2. Current State Assessment

### What Exists (v0.2.3)

| Component | Status | Notes |
|-----------|--------|-------|
| Multi-provider LLM support | ✅ Complete | OpenAI, Anthropic, Gemini, Ollama |
| Local CLI review | ✅ Complete | `cr review branch` works |
| Output formats | ✅ Complete | Markdown, JSON, SARIF |
| GitHub workflow | ⚠️ Partial | Posts PR comments, uploads SARIF, but no inline annotations |
| Inline annotations | ❌ Missing | Key MVP gap |
| Request changes | ❌ Missing | Key MVP gap |
| Code review initiation | ❌ Missing | Currently posts comments, not reviews |

### Gap Analysis

| Gap | Current | Target | Effort Est. |
|-----|---------|--------|-------------|
| Inline annotations | PR comment only | Line-specific annotations | Medium |
| Code review API | Uses `gh pr comment` | Use `gh api` for reviews | Medium |
| Request changes | Never blocks | Configurable blocking | Medium |
| Skip trigger | None | `[skip code-review]` | Low |
| Diff position mapping | Not implemented | Map findings to diff positions | High |
| Incremental reviews | Full PR every time | Only new changes since last review | High |
| Finding deduplication | Re-flags same issues | Track + skip duplicates | Medium |
| PR size guards | May fail on large PRs | Warn, truncate, or split | Medium |
| Comment threading | Orphan comments | Reply to existing threads | Medium |

---

## 3. Proposed Phase Structure

### Phase 1: Foundation (Current - v0.2.x)
**Status:** ✅ Complete

- Multi-provider LLM support
- Local CLI functionality
- Basic GitHub workflow integration
- Security testing validation

### Phase 2: GitHub Native (v0.3.x)
**Status:** Next Phase

**Goal:** Make the bot a first-class GitHub reviewer that's usable at scale

| Milestone | Features |
|-----------|----------|
| 2.1: Inline Annotations | Diff position mapping, line-specific comments |
| 2.2: Review API | Use GitHub review API instead of comments |
| 2.3: Request Changes | Configurable blocking behavior |
| 2.4: Skip Trigger | `[skip code-review]` support |
| 2.5: Incremental Reviews | Only review new changes since last review |
| 2.6: Finding Deduplication | Track findings, don't re-flag same issues |
| 2.7: PR Size Guards | Warn/truncate/split large PRs |

**Why these are MVP:** Without incremental reviews and deduplication, every push triggers a full re-review with duplicate findings. The tool becomes unusable noise.

### Phase 3: Production Hardening (v0.4.x)
**Status:** Future

**Goal:** Production-ready with feedback loops and cost visibility

| Feature | Description |
|---------|-------------|
| Input Token Budgeting | Prevent context overflow, smart truncation |
| Feedback Learning | Capture accept/dismiss to improve future reviews |
| Auto-Fix Suggestions | GitHub `suggestion` format for one-click fixes |
| Review Templates | Different prompts for feature/bugfix/security PRs |
| Smart Context Selection | Only load relevant docs based on changed files |
| Comment Threading | Reply to existing threads instead of new comments |
| Cost Visibility | Show estimated cost before running review |
| Enhanced Secret Detection | Entropy-based detection for encoded secrets |

### Phase 4: Enterprise Features (v1.0+)
**Status:** Future

- Multi-platform (GitLab, Bitbucket)
- Org-wide learning
- Advanced analytics
- Custom rules engine

---

## 4. Document Structure

### What to Create

| Document | Purpose | Location |
|----------|---------|----------|
| CLAUDE.md | Quick start, current status, skills | `/CLAUDE.md` |
| MVP_ROADMAP.md | High-level phases only | `/docs/MVP_ROADMAP.md` |
| ARCHITECTURE.md | System design | `/docs/ARCHITECTURE.md` (exists, update) |

### What to Archive

| Document | Action |
|----------|--------|
| ROADMAP.md | Archive to `docs/archive/`, replace with slim version |
| Session summaries | Keep as historical record |

### Skills to Create

| Skill | Purpose |
|-------|---------|
| `github-workflow` | Commits, PRs, issues |
| `development` | Building, testing, debugging |
| `architecture` | Design decisions, codebase understanding |
| `review` | Using the code reviewer itself |

### Commands to Create

| Command | Purpose |
|---------|---------|
| `resume` | Continue work on current task |
| `review-pr` | Run code review on a PR |

---

## 5. GitHub Issue Structure

### Milestones

| Milestone | Description | Target |
|-----------|-------------|--------|
| v0.3.0 - GitHub Native | First-class reviewer | Phase 2 |
| v0.4.0 - Production | Hardening & reliability | Phase 3 |
| v1.0.0 - MVP Complete | Feature complete | Phase 3/4 |

### Epics (as GitHub Issues with `epic` label)

| Epic | Phase | Key Issues |
|------|-------|------------|
| Inline Annotations | 2 | Diff position mapping, annotation API |
| Review API Integration | 2 | GitHub review creation, request changes |
| Incremental Reviews | 2 | Change tracking, commit-range diffing |
| Finding Deduplication | 2 | Finding fingerprinting, state persistence |
| PR Size Management | 2 | Size detection, truncation, warnings |
| Security Hardening | 3 | Anti-injection, token budgeting, entropy detection |
| Feedback Loop | 3 | Accept/dismiss capture, precision learning |
| Cost Management | 3 | Pre-review estimates, cost reporting |

### Issue Labels

| Label | Purpose |
|-------|---------|
| `epic` | Parent issue for feature area |
| `phase:2` / `phase:3` | Phase assignment |
| `priority:high/medium/low` | Prioritization |
| `type:feature/bug/tech-debt` | Issue type |

---

## 6. Execution Order

### Immediate (This Session)

1. [ ] Tag v0.2.3 release
2. [ ] Create CLAUDE.md
3. [ ] Create `.claude/skills/` directory with initial skills
4. [ ] Archive current ROADMAP.md
5. [ ] Create slim MVP_ROADMAP.md

### Next Session

1. [ ] Create GitHub milestones (v0.3.0, v0.4.0, v1.0.0)
2. [ ] Create GitHub issues from ROADMAP items
3. [ ] Begin Phase 2 work (inline annotations)

---

## 7. Key Decisions

| Decision | Rationale |
|----------|-----------|
| First-class reviewer, not advisory | User requirement - bot can request changes |
| Defer multi-platform to v2.0+ | Focus on GitHub excellence first |
| Phase 2 = GitHub Native | Biggest value-add, closes main gap |
| Skills over docs | Load context on-demand, reduce always-on overhead |
| Slim ROADMAP | Track work in GitHub Issues, not markdown |

---

## References

- Intraspect CLAUDE.md: `/Users/brandon/Development/knomixio/intraspect/CLAUDE.md`
- Intraspect skills: `/Users/brandon/Development/knomixio/intraspect/.claude/skills/`
- Current ROADMAP: `/Users/brandon/Development/personal/code-reviewer/ROADMAP.md`
