# Code Reviewer (cr)

AI-powered code review tool that uses multiple LLM providers to analyze Git branches and generate detailed review feedback.

## Features

- **Multi-Provider Support**: OpenAI, Anthropic Claude, Google Gemini, and local Ollama models
- **Git Integration**: Review branches, commits, and diffs directly from your repository
- **Multiple Output Formats**: Markdown, JSON, and SARIF for CI/CD integration
- **Cost Tracking**: Automatic token counting and cost calculation per provider
- **Observability**: Comprehensive logging and metrics for monitoring API usage
- **Review History**: SQLite-based storage for tracking reviews over time
- **Secret Protection**: Automatic redaction to prevent secrets from being sent to LLMs
- **Deterministic Reviews**: Reproducible results for CI/CD pipelines
- **Merge Strategies**: Combine insights from multiple providers with configurable weights

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/brandon/code-reviewer
cd code-reviewer

# Build the tool
go build -o cr ./cmd/cr

# Or use the magefile
mage build
```

### Configuration

1. Create a configuration file:

```bash
mkdir -p ~/.config/cr
cat > ~/.config/cr/cr.yaml << 'EOF'
providers:
  openai:
    enabled: true
    model: "gpt-4o-mini"
    apiKey: "${OPENAI_API_KEY}"

output:
  directory: "./reviews"

observability:
  logging:
    enabled: true
    level: "info"
    format: "human"
    redactAPIKeys: true
  metrics:
    enabled: true
EOF
```

2. Set your API key:

```bash
export OPENAI_API_KEY="sk-your-api-key-here"
```

### Basic Usage

```bash
# Review the current branch against main
./cr review branch main --target HEAD

# Review specific commits
./cr review branch HEAD~3 --target HEAD

# Review with multiple providers
./cr review branch main --target HEAD
```

## Observability and Cost Tracking

The tool provides comprehensive observability features to monitor API usage, track costs, and debug issues.

### Enabling Logging

```yaml
# cr.yaml
observability:
  logging:
    enabled: true
    level: "info"        # Options: debug, info, error
    format: "human"      # Options: human, json
    redactAPIKeys: true  # Always redact API keys (recommended)
```

### Log Output Examples

**Human-readable format (default):**
```
[INFO] openai/gpt-4o-mini: Response received (duration=2.3s, tokens=150/75, cost=$0.0012)
[INFO] anthropic/claude-3-5-sonnet-20241022: Response received (duration=3.1s, tokens=200/120, cost=$0.0028)
```

**JSON format (for log aggregation):**
```json
{"level":"info","type":"response","provider":"openai","model":"gpt-4o-mini","timestamp":"2025-10-21T10:30:00Z","duration_ms":2300,"tokens_in":150,"tokens_out":75,"cost":0.0012,"status_code":200,"finish_reason":"stop"}
```

### Cost Tracking

Costs are automatically calculated and displayed in all output formats:

**Terminal output:**
```
Review complete!
- OpenAI (gpt-4o-mini): $0.0012 (150 tokens in, 75 tokens out)
- Anthropic (claude-3-5-sonnet): $0.0028 (200 tokens in, 120 tokens out)
Total cost: $0.0040
```

**Review files include cost data:**
```markdown
## Review Summary
- **Provider**: OpenAI (gpt-4o-mini)
- **Cost**: $0.0012
- **Tokens**: 150 in, 75 out
- **Duration**: 2.3s
```

### Debug Mode

Enable debug logging to see detailed request information:

```bash
# Via config file
observability:
  logging:
    level: "debug"

# Via environment variable
export CR_OBSERVABILITY_LOGGING_LEVEL=debug
./cr review branch main --target HEAD
```

Debug output includes:
- Prompt character counts
- Redacted API keys (last 4 characters only)
- Request timestamps
- Model and provider details

**Example debug output:**
```
[DEBUG] openai/gpt-4o-mini: Request sent (prompt=1543 chars, key=****cdef)
[INFO] openai/gpt-4o-mini: Response received (duration=2.3s, tokens=150/75, cost=$0.0012)
```

### Metrics Tracking

Enable metrics to track performance and usage statistics:

```yaml
observability:
  metrics:
    enabled: true
```

Metrics include:
- Request/response duration
- Token counts (input and output)
- Cost per request and cumulative cost
- Error rates by provider and type
- Success rates

### Production Monitoring

For production use, enable JSON logging for integration with log aggregation tools:

```yaml
observability:
  logging:
    enabled: true
    level: "info"
    format: "json"
    redactAPIKeys: true
  metrics:
    enabled: true
```

Then pipe logs to your monitoring system:
```bash
./cr review branch main --target HEAD 2>&1 | tee -a /var/log/cr/reviews.log
```

### Environment Variable Configuration

All observability settings can be configured via environment variables:

```bash
# Enable logging
export CR_OBSERVABILITY_LOGGING_ENABLED=true
export CR_OBSERVABILITY_LOGGING_LEVEL=debug
export CR_OBSERVABILITY_LOGGING_FORMAT=json
export CR_OBSERVABILITY_LOGGING_REDACTAPIKEYS=true

# Enable metrics
export CR_OBSERVABILITY_METRICS_ENABLED=true

# Run review
./cr review branch main --target HEAD
```

## Documentation

- [Configuration Guide](docs/CONFIGURATION.md) - Complete configuration reference
- [Observability Guide](docs/OBSERVABILITY.md) - Detailed logging and metrics documentation
- [Cost Tracking Guide](docs/COST_TRACKING.md) - Cost analysis and optimization strategies
- [Architecture](docs/ARCHITECTURE.md) - System architecture and design decisions
- [Development Workflow](docs/DEVELOPER_WORKFLOW.md) - Contributing and development guide

## Example Configurations

### Minimal (Testing)

```yaml
providers:
  static:
    enabled: true
    model: "static-model"

output:
  directory: "./reviews"
```

### Production (Multi-provider with observability)

```yaml
providers:
  openai:
    enabled: true
    model: "gpt-4o-mini"
    apiKey: "${OPENAI_API_KEY}"

  anthropic:
    enabled: true
    model: "claude-3-5-sonnet-20241022"
    apiKey: "${ANTHROPIC_API_KEY}"

store:
  enabled: true
  path: "~/.config/cr/reviews.db"

output:
  directory: "./reviews"

observability:
  logging:
    enabled: true
    level: "info"
    format: "json"
    redactAPIKeys: true
  metrics:
    enabled: true

redaction:
  enabled: true
  denyGlobs:
    - "**/*.env"
    - "**/*.pem"
    - "**/*.key"

determinism:
  enabled: true
  temperature: 0.0
  useSeed: true
```

### Local Only (Ollama)

```yaml
providers:
  ollama:
    enabled: true
    model: "codellama"

output:
  directory: "./reviews"

observability:
  logging:
    enabled: true
    level: "debug"
    format: "human"
```

## Supported LLM Providers

| Provider | Models | API Key Required | Cost |
|----------|--------|------------------|------|
| OpenAI | gpt-4o, gpt-4o-mini, o1-preview, o1-mini | Yes | Paid |
| Anthropic | claude-3-5-sonnet-20241022, claude-3-5-haiku-20241022 | Yes | Paid |
| Google Gemini | gemini-1.5-pro, gemini-1.5-flash | Yes | Paid |
| Ollama | Any local model | No | Free |

See [COST_TRACKING.md](docs/COST_TRACKING.md) for detailed pricing information.

## Output Formats

The tool generates reviews in multiple formats:

1. **Markdown** (`.md`) - Human-readable review with findings and suggestions
2. **JSON** (`.json`) - Structured data for programmatic analysis
3. **SARIF** (`.sarif`) - Static Analysis Results Interchange Format for CI/CD integration

All formats include:
- Review findings with severity levels
- File locations and line numbers
- Cost and token usage data
- Provider and model information
- Timestamps and duration

## CI/CD Integration

Use the SARIF output format for integration with GitHub, GitLab, or other CI/CD platforms:

```bash
# Generate SARIF output
./cr review branch origin/main --target HEAD

# Upload to GitHub Code Scanning
gh api repos/$REPO/code-scanning/sarifs \
  -F sarif=@reviews/review-openai-*.sarif \
  -F ref=$GITHUB_REF \
  -F sha=$GITHUB_SHA
```

## Security

- **API Key Redaction**: All logs redact API keys to show only the last 4 characters
- **Secret Protection**: Configure deny globs to prevent sensitive files from being sent to LLMs
- **Local Models**: Use Ollama for completely local, private code reviews

## Building

```bash
# Build binary
mage build

# Run tests
mage test

# Run linter
mage maintain:gofmt

# Full CI pipeline
mage ci
```

## Contributing

See [DEVELOPER_WORKFLOW.md](docs/DEVELOPER_WORKFLOW.md) for development setup and contribution guidelines.

## License

[Add your license here]

## Support

For issues, questions, or feature requests, please open an issue on GitHub.
