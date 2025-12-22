# Continue Work

Figure out where we left off and resume, with awareness of the Phase 2 plan.

## 1. Check Planning State First

**Read the planning documents to understand current priorities:**

```
# Quick check of project reset plan status
head -100 docs/PROJECT_RESET_PLAN.md
```

## 2. Gather Git Context

1. **Check current branch:**
   ```
   git branch --show-current
   git status
   ```

2. **Look for issue context in branch name:**
   - Branch names like `feature/42-...` or `fix/123-...` indicate an issue number
   - If found, fetch that issue: `gh issue view <number>`

3. **Check for uncommitted work:**
   ```
   git status
   git stash list
   ```

4. **Check recent commits on this branch:**
   ```
   git log --oneline -10
   ```

5. **Check for open PRs I might be working on:**
   ```
   gh pr list --state open --author @me
   gh pr list --state open
   ```

## 3. Check Phase 2 Issue Status

```
# See high-priority Phase 2 issues
gh issue list --state open --label "phase:2" --label "priority:high" --limit 10

# See medium-priority Phase 2 issues
gh issue list --state open --label "phase:2" --label "priority:medium" --limit 10

# See low-priority Phase 2 issues
gh issue list --state open --label "phase:2" --label "priority:low" --limit 10
```

**Phase 2 Priority Order (from PROJECT_RESET_PLAN.md):**

1. **Milestone 2.1:** Inline Annotations (diff position mapping, line-specific comments)
2. **Milestone 2.2:** Review API (use GitHub review API instead of comments)
3. **Milestone 2.3:** Request Changes (configurable blocking behavior)
4. **Milestone 2.4:** Skip Trigger (`[skip code-review]` support)
5. **Milestone 2.5:** Incremental Reviews (only review new changes)
6. **Milestone 2.6:** Finding Deduplication (track findings, don't re-flag)
7. **Milestone 2.7:** PR Size Guards (warn/truncate/split large PRs)

## 4. Present Findings

Summarize:
- Current branch and its likely associated issue
- Any uncommitted or stashed changes
- Open PRs that might need attention
- Best guess at what was being worked on
- **Current Phase 2 priorities** based on issue labels
- **Any recently closed issues** that may need follow-up

## 5. Suggest Next Steps

Based on the context:

1. **If on a feature branch with uncommitted work:**
   "It looks like you were working on #XXX. Should I continue?"

2. **If on main with clean state:**
   "You're on main with no pending work. Based on Phase 2 priorities, I suggest:
   - High priority: [next uncompleted high-priority issue]
   - Medium priority: [next uncompleted medium-priority issue]
   Which would you like to work on?"

3. **If there's an open PR:**
   "There's an open PR for #XXX. Should I check its status and address any feedback?"

## 6. Before Starting Work

Do NOT start implementation until the user confirms.

When they confirm:
1. If starting a new issue, create feature branch: `git checkout -b feature/<issue-number>-short-description`
2. Read the issue details: `gh issue view <number>`
3. Begin implementation following TDD workflow

## 7. Remember the Definition of Done

Before marking any work as complete:

- [ ] Tests written (TDD)
- [ ] Code formatted (`gofmt -w .`)
- [ ] All tests pass (`go test ./...`)
- [ ] Build succeeds (`go build -o cr ./cmd/cr`)
- [ ] No race conditions (`go test -race ./...`)
