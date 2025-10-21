# Documentation

This directory contains all documentation for the Code Reviewer project.

## Quick Start

- **[../README.md](../README.md)** - Project overview, installation, and quick start guide
- **[../ROADMAP.md](../ROADMAP.md)** - Current status and future feature plans
- **[CONFIGURATION.md](CONFIGURATION.md)** - Complete configuration reference

## User Guides

### Core Features
- **[CONFIGURATION.md](CONFIGURATION.md)** - Configuration options, examples, and environment variables
- **[OBSERVABILITY.md](OBSERVABILITY.md)** - Logging, metrics, and monitoring guide
- **[COST_TRACKING.md](COST_TRACKING.md)** - Cost analysis, pricing, and optimization strategies
- **[DEVELOPER_WORKFLOW.md](DEVELOPER_WORKFLOW.md)** - Contributing and development workflow

## Technical Documentation

### Architecture & Design
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture and design principles
- **[TECHNICAL_DESIGN_SPEC.md](TECHNICAL_DESIGN_SPEC.md)** - Overall technical specifications

### Component Design Documents
- **[HTTP_CLIENT_DESIGN.md](HTTP_CLIENT_DESIGN.md)** - HTTP client architecture and API integration
- **[OBSERVABILITY_COST_DESIGN.md](OBSERVABILITY_COST_DESIGN.md)** - Observability and cost tracking design decisions
- **[STORE_INTEGRATION_DESIGN.md](STORE_INTEGRATION_DESIGN.md)** - SQLite store architecture and integration patterns
- **[PHASE3_TECHNICAL_DESIGN.md](PHASE3_TECHNICAL_DESIGN.md)** - Phase 3 technical design (store, TUI, feedback)
- **[OLLAMA_GEMINI_DESIGN.md](OLLAMA_GEMINI_DESIGN.md)** - Ollama and Gemini client design

## Implementation History

See **[archive/](archive/)** for completed implementation checklists and historical planning documents.

## Documentation Organization

```
docs/
├── README.md                          # This file
├── ARCHITECTURE.md                    # System architecture
├── CONFIGURATION.md                   # User configuration guide
├── OBSERVABILITY.md                   # Logging and metrics guide
├── COST_TRACKING.md                   # Cost analysis guide
├── DEVELOPER_WORKFLOW.md              # Development guide
├── HTTP_CLIENT_DESIGN.md              # HTTP client design
├── OBSERVABILITY_COST_DESIGN.md       # Observability design
├── STORE_INTEGRATION_DESIGN.md        # Store design
├── PHASE3_TECHNICAL_DESIGN.md         # Phase 3 design
├── TECHNICAL_DESIGN_SPEC.md           # Technical specifications
├── OLLAMA_GEMINI_DESIGN.md            # Ollama/Gemini design
└── archive/                           # Completed checklists
    ├── README.md                      # Archive index
    ├── PHASE1_TODO.md                 # ✅ Complete
    ├── HTTP_CLIENT_TODO.md            # ✅ Complete
    ├── OBSERVABILITY_COST_TODO.md     # ✅ Complete
    ├── STORE_INTEGRATION_TODO.md      # ✅ Complete
    ├── MAIN_INTEGRATION_CHECKLIST.md  # ✅ Complete
    ├── ORCHESTRATOR_INTEGRATION_CHECKLIST.md  # ✅ Complete
    ├── PHASE3_TODO.md                 # ✅ Week 1 Complete
    ├── IMPEMENTATION_PLAN.md          # Original plan
    └── CODE_REVIEW_FIXES.md           # Historical fixes
```

## For Contributors

1. **Getting Started**: Read [DEVELOPER_WORKFLOW.md](DEVELOPER_WORKFLOW.md)
2. **Architecture**: Understand [ARCHITECTURE.md](ARCHITECTURE.md)
3. **Roadmap**: Check [../ROADMAP.md](../ROADMAP.md) for planned features
4. **Testing**: Run `mage test` or `mage ci`
5. **Documentation**: Update relevant guides when adding features

## For Users

1. **Installation**: See [../README.md](../README.md)
2. **Configuration**: See [CONFIGURATION.md](CONFIGURATION.md)
3. **Monitoring**: See [OBSERVABILITY.md](OBSERVABILITY.md)
4. **Costs**: See [COST_TRACKING.md](COST_TRACKING.md)

## Updating Documentation

When adding new features:
- Update user guides (CONFIGURATION.md, OBSERVABILITY.md, etc.)
- Add design docs if introducing new components
- Update ARCHITECTURE.md if changing system design
- Update ../ROADMAP.md to reflect progress
- Move completed checklists to archive/
