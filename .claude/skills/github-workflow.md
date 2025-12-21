# GitHub Workflow Skill

Load context for GitHub operations: commits, PRs, issues, and releases.

## Instructions

When this skill is invoked, assist with GitHub workflows.

## Commit Messages

Format: `<type>: <description>`

Types:
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring
- `test`: Adding/updating tests
- `docs`: Documentation
- `chore`: Maintenance tasks

**Important:** Do NOT include Claude attribution footer.

## Pull Requests

**Structure:**
```markdown
## Summary
<1-3 bullet points describing what this PR does>

## Changes
- <Specific change 1>
- <Specific change 2>

## Test Plan
- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test -race ./...`)
- [ ] Manual testing completed

## Checklist
- [ ] Code formatted (`gofmt -w .`)
- [ ] Tests added/updated
- [ ] Documentation updated (if needed)
```

## Issue Management

**Creating issues:**
```bash
gh issue create --title "Title" --body "Description" --label "type:feature"
```

**Labels:**
- `type:feature`, `type:bug`, `type:tech-debt`
- `priority:high`, `priority:medium`, `priority:low`
- `phase:2`, `phase:3`, `phase:4`
- `epic` - Parent issue for feature area

**Milestones:**
- `v0.3.0 - GitHub Native`
- `v0.4.0 - Production`
- `v1.0.0 - MVP Complete`

## Releases

**Tagging a release:**
```bash
git tag -a v0.X.Y -m "Release message"
git push origin v0.X.Y
```

**Creating GitHub release:**
```bash
gh release create v0.X.Y --title "v0.X.Y - Title" --notes "Release notes"
```

## Branch Strategy

- `main` - Stable, tagged releases
- `feature/*` - Feature development
- `fix/*` - Bug fixes
- `test/*` - Testing/experiments

## After Loading Context

Assist with commits, PRs, issues, releases, or other GitHub operations.
