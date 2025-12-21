# Code Reviewer Roadmap

**Last Updated:** 2025-12-21
**Current Version:** v0.2.3
**Current Phase:** Phase 2 Planning

---

## Vision

An AI-powered code review tool that acts as a **first-class GitHub reviewer** - providing inline annotations, requesting changes for critical issues, and learning from feedback.

---

## Phase Overview

| Phase | Version | Status | Focus |
|-------|---------|--------|-------|
| 1: Foundation | v0.1.x - v0.2.x | ✅ Complete | Multi-provider LLM, local CLI, basic GitHub |
| **2: GitHub Native** | v0.3.x | 🚧 Next | First-class reviewer, inline annotations |
| 3: Production | v0.4.x | Planned | Feedback loops, cost visibility |
| 4: Enterprise | v1.0+ | Planned | Multi-platform, org-wide learning |

---

## Phase 2: GitHub Native (v0.3.x)

**Goal:** Make the bot a first-class GitHub reviewer that's usable at scale

| Milestone | Feature | Status |
|-----------|---------|--------|
| 2.1 | Inline Annotations | Not Started |
| 2.2 | Review API Integration | Not Started |
| 2.3 | Request Changes | Not Started |
| 2.4 | Skip Trigger (`[skip code-review]`) | Not Started |
| 2.5 | Incremental Reviews | Not Started |
| 2.6 | Finding Deduplication | Not Started |
| 2.7 | PR Size Guards | Not Started |

**Exit Criteria:**
- [ ] Bot initiates GitHub code reviews (not just comments)
- [ ] Findings appear as inline annotations on specific lines
- [ ] Configurable rules trigger "Request Changes"
- [ ] Only new changes reviewed after each push
- [ ] No duplicate findings across PR updates

---

## Phase 3: Production Hardening (v0.4.x)

**Goal:** Production-ready with feedback loops and cost visibility

| Feature | Description |
|---------|-------------|
| Feedback Learning | Capture accept/dismiss to improve future reviews |
| Auto-Fix Suggestions | GitHub `suggestion` format for one-click fixes |
| Review Templates | Different prompts for feature/bugfix/security PRs |
| Smart Context Selection | Only load relevant docs based on changed files |
| Comment Threading | Reply to existing threads instead of new comments |
| Cost Visibility | Show estimated cost before running review |
| Token Budgeting | Prevent context overflow, smart truncation |
| Enhanced Secret Detection | Entropy-based detection for encoded secrets |

---

## Phase 4: Enterprise Features (v1.0+)

**Goal:** Enterprise-ready with multi-platform support

| Feature | Description |
|---------|-------------|
| Multi-Platform | GitLab, Bitbucket, Azure DevOps |
| Org-Wide Learning | Postgres sync for shared precision priors |
| Custom Rules Engine | Repo-specific patterns and checks |
| Review Metrics | Precision tracking, noise analysis |
| Reviewer Personas | Security-focused, performance-focused modes |

---

## References

- **Detailed Planning:** `docs/PROJECT_RESET_PLAN.md`
- **Architecture:** `docs/ARCHITECTURE.md`
- **Security:** `docs/SECURITY.md`
- **Archived Roadmap:** `docs/archive/ROADMAP_v0.2.3.md`

---

## Work Tracking

**Track work in GitHub Issues, not this document.**

- [GitHub Issues](https://github.com/bkyoung/code-reviewer/issues)
- [Milestones](https://github.com/bkyoung/code-reviewer/milestones)
