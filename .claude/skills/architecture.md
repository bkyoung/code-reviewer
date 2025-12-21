# Architecture Skill

Load context for understanding the codebase design and making architectural decisions.

## Instructions

When this skill is invoked:

1. **Load Architecture Doc**: Read `docs/ARCHITECTURE.md`
2. **Review Project Structure**: Understand the clean architecture layers

## Clean Architecture Layers

```
┌─────────────────────────────────────────────────┐
│                  cmd/cr/                        │  Entry point
├─────────────────────────────────────────────────┤
│              internal/adapter/                  │  External integrations
│    cli/ git/ llm/ output/ store/ observability/ │
├─────────────────────────────────────────────────┤
│              internal/usecase/                  │  Business logic
│              review/ merge/                     │
├─────────────────────────────────────────────────┤
│              internal/domain/                   │  Core types
│              (NO external dependencies)         │
└─────────────────────────────────────────────────┘
```

## Key Design Principles

1. **Domain has no dependencies** - Pure Go types only
2. **Adapters depend on domain** - Never the reverse
3. **Use cases orchestrate** - Coordinate adapters and domain logic
4. **Dependency injection** - Interfaces for testability

## Key Components

### Domain (`internal/domain/`)
- `Diff`, `FileDiff` - Git diff representation
- `Finding`, `Review` - Review results
- `Severity`, `Category` - Finding classification

### Use Cases (`internal/usecase/`)
- `review/orchestrator.go` - Main review coordination
- `review/prompt_builder.go` - LLM prompt construction
- `review/context.go` - Context gathering
- `merge/intelligent_merger.go` - Multi-provider consensus

### Adapters (`internal/adapter/`)
- `llm/` - Provider clients (OpenAI, Anthropic, Gemini, Ollama)
- `git/` - Git operations via go-git
- `output/` - Formatters (Markdown, JSON, SARIF)
- `store/` - SQLite persistence

## Design Decisions

Key decisions are documented in the ROADMAP.md under "Known Issues & Technical Debt".

**Intentional duplication:**
- ID generation exists in both `usecase/review/` and `store/` to maintain clean architecture boundaries

**Template-based prompts:**
- Prompts use Go text/template for provider-specific formatting

## After Loading Context

Read `docs/ARCHITECTURE.md` for full details, then assist with design questions or codebase understanding.
