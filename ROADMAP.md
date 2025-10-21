# Code Reviewer Roadmap

## Current Status

**v0.1.0 - Core Functionality Complete** ✅

The code reviewer now has:
- ✅ Multi-provider LLM support (OpenAI, Anthropic, Gemini, Ollama)
- ✅ Full HTTP client implementation with retry logic and error handling
- ✅ Comprehensive observability (logging, metrics, cost tracking)
- ✅ SQLite-based review persistence
- ✅ Multiple output formats (Markdown, JSON, SARIF)
- ✅ Configuration system with environment variable support
- ✅ Secret redaction
- ✅ Deterministic reviews for CI/CD
- ✅ All unit and integration tests passing (120+ tests)

## Near-Term Enhancements

### 1. Manual Testing & Verification (Optional)
**Priority: Low**

- [ ] Manual testing with real API keys for all 4 providers
- [ ] Verify cost calculations match actual provider billing
- [ ] Test database persistence with real reviews
- [ ] Inspect SQLite database schema and data
- [ ] Performance testing with large diffs

### 2. Configuration Enhancements
**Priority: Low**

- [ ] Add `http.timeout` config option (currently hardcoded to 60s)
- [ ] Add `http.maxRetries` config option (currently hardcoded to 5)
- [ ] Add `http.retryBackoff` config option for customizing backoff strategy
- [ ] Add provider-specific timeout overrides

### 3. Resilience Features
**Priority: Low**

- [ ] Implement circuit breaker pattern for repeated failures
- [ ] Add graceful shutdown handling for in-flight requests
- [ ] Improve context propagation and cancellation support

## Future Features (Deferred)

### Phase 3 Continuation: TUI & Intelligence (Weeks 2-4)

**Status: Deferred - Store infrastructure complete, TUI not yet implemented**

#### TUI Implementation (Week 2)
- [ ] Add Bubble Tea, Bubbles, and Lipgloss dependencies
- [ ] Create `internal/adapter/tui/` package
- [ ] Implement review list view (load runs from store)
- [ ] Implement finding list view (show findings by severity)
- [ ] Implement finding detail view (scrollable viewport)
- [ ] Add navigation and key bindings
- [ ] Add `tui` command to CLI

#### Feedback & Intelligence (Week 3)
- [ ] Add feedback capture in TUI ('a' accept, 'r' reject)
- [ ] Implement feedback processor use case
- [ ] Create statistics view showing precision by provider
- [ ] Implement intelligent merger v2 (uses precision priors)
- [ ] Update merger configuration (`strategy: "intelligent"`)
- [ ] Wire precision priors into scoring algorithm

#### Enhanced Redaction (Week 4)
- [ ] Implement entropy-based secret detection
- [ ] Add Shannon entropy calculation
- [ ] Integrate entropy detector into redaction engine
- [ ] Add config options for entropy threshold
- [ ] Combine regex + entropy detection for better coverage

### Phase 4: Budget Enforcement & Cost Control

**Status: Not Started - Cost tracking infrastructure complete**

- [ ] Add budget.hardCapUSD config option
- [ ] Implement pre-flight cost estimation
- [ ] Add degradation policies (reduce providers, reduce context)
- [ ] Create budget tracking middleware
- [ ] Add warnings when approaching budget limits
- [ ] Reject reviews that would exceed hard cap

### Phase 5: Multi-Repository & CI/CD

**Status: Not Started**

- [ ] Support reviewing multiple repositories
- [ ] Add PR/MR integration (GitHub, GitLab)
- [ ] Implement GitHub Actions workflow
- [ ] Add GitLab CI template
- [ ] Create Docker image for containerized reviews
- [ ] Add webhook support for automatic reviews

### Advanced Features (Backlog)

#### Model Experimentation
- [ ] Add model comparison mode (compare outputs side-by-side)
- [ ] Implement A/B testing framework
- [ ] Add custom prompt templates
- [ ] Support for fine-tuned models

#### Collaboration
- [ ] Export/import review history
- [ ] Share precision priors across teams
- [ ] Generate team-wide statistics
- [ ] Create learning datasets from feedback

#### Performance
- [ ] Implement request batching for large diffs
- [ ] Add streaming support for faster feedback
- [ ] Optimize token usage with smart chunking
- [ ] Add caching for repeated diff analysis

#### Integration
- [ ] Prometheus metrics export
- [ ] OpenTelemetry tracing support
- [ ] Slack/Discord notifications
- [ ] Email digest of review summaries

## Completed Work (Archive)

See `docs/archive/` for detailed implementation checklists:

- **Phase 1**: Core architecture and domain model
- **Phase 2**: Git integration and basic review workflow
- **HTTP Clients**: All 4 providers (OpenAI, Anthropic, Gemini, Ollama)
- **Observability**: Logging, metrics, and cost tracking
- **Store Integration**: SQLite persistence with full orchestrator integration
- **Configuration**: Complete config system with environment variable expansion

## Contributing

When adding new features:

1. Follow TDD (test-driven development)
2. Maintain clean architecture principles
3. Update documentation
4. Ensure all tests pass (`mage ci`)
5. Update this roadmap

## Release Planning

### v0.1.0 (Current)
- Core review functionality
- Multi-provider support
- Observability and cost tracking
- Review persistence

### v0.2.0 (Future)
- TUI for review history
- Feedback and intelligent merging
- Enhanced secret detection

### v0.3.0 (Future)
- Budget enforcement
- Advanced cost controls

### v1.0.0 (Future)
- Production-ready
- CI/CD integrations
- Comprehensive documentation
- Performance optimized
