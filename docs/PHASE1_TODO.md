# Phase 1 TODOs

- [x] Establish the Go module and clean architecture directory scaffolding (`cmd/`, `internal/config`, `internal/domain`, `internal/usecase`, `internal/adapter`).
- [x] Define core domain models (review, finding, diff) and configuration structures.
- [x] Create failing tests for the review orchestrator use case that exercise the Git engine, LLM provider port, and Markdown output adapter via dependency injection.
- [x] Create failing tests for the Git engine that verify cumulative diffs between branches using an in-memory go-git repository.
- [x] Create failing tests for the Markdown output manager to ensure deterministic report generation.
- [x] Create failing tests for the CLI layer that validate wiring of the `review branch` command into the orchestrator.
- [x] Implement production code to satisfy the tests while keeping the design functional, SOLID, and clean.
- [x] Run formatting, linting, tests, and build pipelines; then refresh the documentation to reflect Phase 1 status.
