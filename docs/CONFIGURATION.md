# Configuration Guide

## Configuration File Location

The code-reviewer tool searches for configuration in the following locations (in order):

1. **Current directory**: `./cr.yaml`
2. **User config directory**: `~/.config/cr/cr.yaml`

Files in the current directory take precedence over files in the user config directory.

## Quick Start

### 1. Copy the starter configuration

```bash
# Option A: Install globally (recommended)
mkdir -p ~/.config/cr
cp cr.yaml ~/.config/cr/cr.yaml

# Option B: Use in current project only
cp cr.yaml ./cr.yaml
```

### 2. Edit the configuration

Open the config file and enable your desired providers:

```bash
# For global config
vim ~/.config/cr/cr.yaml

# For local config
vim ./cr.yaml
```

### 3. Set API keys (if using cloud providers)

```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="..."
```

Or set them directly in the config file (less secure):

```yaml
providers:
  openai:
    enabled: true
    model: "gpt-4o-mini"
    apiKey: "sk-your-actual-key-here"  # Not recommended for shared configs
```

## Configuration Options

### Providers

Control which LLM providers are used for code reviews:

```yaml
providers:
  static:
    enabled: true        # Enable/disable this provider
    model: "static-model"  # Model name to use

  openai:
    enabled: true
    model: "gpt-4o-mini"  # Options: gpt-4o, gpt-4o-mini, gpt-4-turbo, o1-preview, o1-mini
    apiKey: "${OPENAI_API_KEY}"
```

**Available Providers:**
- `static` - Test provider with canned responses (no API key needed)
- `openai` - OpenAI GPT models (requires API key)
  - Standard models: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo` (support temperature, seed, determinism)
  - Reasoning models: `o1-preview`, `o1-mini` (limited parameters, no temperature/seed support)
- `anthropic` - Anthropic Claude models (requires API key)
- `gemini` - Google Gemini models (requires API key)
- `ollama` - Local models via Ollama (no API key, requires Ollama running)

### Store (Review History Persistence)

Configure SQLite database for storing review history:

```yaml
store:
  enabled: true  # Set to false to disable persistence
  path: "~/.config/cr/reviews.db"  # Database file location
```

**Benefits of enabling the store:**
- Track review history over time
- Identify duplicate findings across runs
- Analyze provider precision and accuracy
- Build learning datasets for model improvement

### Output Directory

Control where review files are written:

```yaml
output:
  directory: "./reviews"  # Relative or absolute path
```

Output files include:
- `review-{provider}-{timestamp}.md` - Markdown format
- `review-{provider}-{timestamp}.json` - JSON format
- `review-{provider}-{timestamp}.sarif` - SARIF format (for CI/CD integration)

### Redaction (Secret Protection)

Prevent secrets from being sent to LLM providers:

```yaml
redaction:
  enabled: true
  denyGlobs:
    - "**/*.env"      # Environment files
    - "**/*.pem"      # Private keys
    - "**/*.key"      # Key files
    - "**/secrets.yaml"
  allowGlobs:
    - "src/**/*.go"   # Explicitly allow patterns
```

### Observability (Logging and Metrics)

Monitor LLM API calls with detailed logging and metrics:

```yaml
observability:
  logging:
    enabled: true           # Enable request/response logging
    level: "info"           # Options: debug, info, error
    format: "human"         # Options: human, json
    redactAPIKeys: true     # Redact API keys in logs (show only last 4 chars)
  metrics:
    enabled: true           # Enable performance and cost metrics tracking
```

**Logging Levels:**
- `debug` - Log requests (with prompt size and redacted API key) and responses
- `info` - Log responses only (default, recommended for production)
- `error` - Log errors only

**Log Formats:**
- `human` - Human-readable format for terminal output (default)
  ```
  [INFO] openai/gpt-4o-mini: Response received (duration=2.3s, tokens=150/75, cost=$0.0012)
  ```
- `json` - Structured JSON for log aggregation and analysis
  ```json
  {"level":"info","type":"response","provider":"openai","model":"gpt-4o-mini","timestamp":"2025-10-21T10:30:00Z","duration_ms":2300,"tokens_in":150,"tokens_out":75,"cost":0.0012,"status_code":200,"finish_reason":"stop"}
  ```

**API Key Redaction:**
- When enabled (default), API keys are redacted to show only the last 4 characters
- Example: `sk-1234567890abcdef` becomes `****cdef`
- Disable only for local development or debugging: `redactAPIKeys: false`

**Metrics Tracked:**
- Request/response duration
- Token counts (input and output)
- Cost per request and total cost
- Error rates and types
- Per-provider statistics

**Environment Variable Overrides:**
```bash
export CR_OBSERVABILITY_LOGGING_ENABLED=true
export CR_OBSERVABILITY_LOGGING_LEVEL=debug
export CR_OBSERVABILITY_LOGGING_FORMAT=json
export CR_OBSERVABILITY_LOGGING_REDACTAPIKEYS=true
export CR_OBSERVABILITY_METRICS_ENABLED=true
```

**When to enable logging:**
- Debugging API issues or rate limits
- Monitoring performance in production
- Analyzing cost patterns
- Troubleshooting timeout or connectivity issues

**When to use JSON format:**
- Production environments with log aggregation (e.g., ELK, Splunk)
- Automated log analysis and alerting
- Cost tracking and budget monitoring

See [OBSERVABILITY.md](./OBSERVABILITY.md) for detailed logging examples and [COST_TRACKING.md](./COST_TRACKING.md) for cost analysis.

### Determinism (Reproducible Reviews)

Control review consistency:

```yaml
determinism:
  enabled: true
  temperature: 0.0  # 0.0 = consistent, 1.0 = creative
  useSeed: true     # Use deterministic seeds per branch comparison
```

**When to enable:**
- CI/CD pipelines (consistent results)
- Testing and validation
- Comparing provider outputs

**When to disable:**
- Exploring different review perspectives
- Generating creative suggestions

**Note:** OpenAI o1-series reasoning models (`o1-preview`, `o1-mini`) do not support temperature or seed parameters. Determinism settings are automatically ignored for these models.

### Merge Configuration

Combine multiple provider reviews into consensus:

```yaml
merge:
  enabled: true
  strategy: "weighted"  # Options: weighted, unanimous, majority
  weights:
    openai: 1.0
    anthropic: 1.5      # Give Anthropic higher weight
    gemini: 0.8
```

**Strategies:**
- `weighted` - Combine findings with provider weights
- `unanimous` - Only include findings all providers agree on
- `majority` - Include findings most providers agree on

### Budget (Cost Control)

Prevent runaway costs:

```yaml
budget:
  hardCapUSD: 10.0  # Maximum spend per review
  degradationPolicy:
    - "reduce-providers"  # Drop lower-priority providers first
    - "reduce-context"    # Then reduce context size
```

## Environment Variables

### Using Environment Variables in Config Files

You can reference environment variables directly in your YAML configuration using `${VAR}` or `$VAR` syntax:

```yaml
providers:
  openai:
    enabled: true
    model: "gpt-4o-mini"
    apiKey: "${OPENAI_API_KEY}"  # Expands to value of OPENAI_API_KEY env var

  anthropic:
    enabled: true
    model: "claude-3-5-sonnet-20241022"
    apiKey: "$ANTHROPIC_API_KEY"  # Also works without braces

output:
  directory: "${CR_OUTPUT_DIR}"  # Can be used for any string field

store:
  path: "${HOME}/.config/cr/reviews.db"  # Useful for home directory references
```

**How it works:**
- When the config is loaded, `${VARIABLE_NAME}` is replaced with the value from `os.Getenv("VARIABLE_NAME")`
- Both `${VAR}` and `$VAR` syntax are supported
- If the environment variable is not set, the original syntax is kept unchanged (e.g., `${UNDEFINED}` stays as `${UNDEFINED}`)
- Only uppercase variable names are matched: `[A-Z_][A-Z0-9_]*`
- Multiple variables can be used in a single string: `"${PATH1}:${PATH2}"`

**Recommended approach:**
1. Use `${VAR}` syntax in config files (portable, keeps secrets out of version control)
2. Set actual values in your shell environment
3. Never commit files with hardcoded API keys

### Overriding with Environment Variables

All configuration can also be overridden completely using environment variables (without modifying the config file):

```bash
# Store configuration
export CR_STORE_ENABLED=true
export CR_STORE_PATH="/custom/path/reviews.db"

# Output directory
export CR_OUTPUT_DIRECTORY="./custom-reviews"

# Provider API keys
export CR_PROVIDERS_OPENAI_APIKEY="sk-..."
export CR_PROVIDERS_ANTHROPIC_APIKEY="sk-ant-..."

# Redaction
export CR_REDACTION_ENABLED=true

# Determinism
export CR_DETERMINISM_ENABLED=true
export CR_DETERMINISM_TEMPERATURE=0.0
```

Environment variables use the format: `CR_<SECTION>_<KEY>`

**Note:** Direct environment variables (e.g., `CR_PROVIDERS_OPENAI_APIKEY`) take precedence over config file values, even those using `${VAR}` expansion.

## Example Configurations

### Minimal (Testing)

```yaml
providers:
  static:
    enabled: true
    model: "static-model"

store:
  enabled: false

output:
  directory: "./reviews"
```

### Production (Multi-provider with persistence)

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

merge:
  enabled: true
  strategy: "weighted"
  weights:
    openai: 1.0
    anthropic: 1.2

redaction:
  enabled: true
  denyGlobs:
    - "**/*.env"
    - "**/*.pem"
    - "**/*.key"

observability:
  logging:
    enabled: true
    level: "info"
    format: "json"  # JSON for production log aggregation
    redactAPIKeys: true
  metrics:
    enabled: true

determinism:
  enabled: true
  temperature: 0.0
  useSeed: true

budget:
  hardCapUSD: 5.0
```

### Local Only (Ollama)

```yaml
providers:
  ollama:
    enabled: true
    model: "codellama"

store:
  enabled: true
  path: "./local-reviews.db"

output:
  directory: "./reviews"

redaction:
  enabled: false
```

## Verifying Configuration

After creating your config file, test it:

```bash
# Build the application
mage build

# Run a test review (uses static provider if no API keys set)
./cr review branch main --target HEAD
```

## Configuration Troubleshooting

### Config file not found

```
Error: config load failed: no configuration file found
```

**Solution:** Ensure config file exists at `~/.config/cr/cr.yaml` or `./cr.yaml`

### Provider API key missing

```
Error: provider initialization failed
```

**Solution:** Set environment variables or add API keys to config file

### Store directory creation failed

```
Warning: failed to create store directory: permission denied
```

**Solution:** Check directory permissions or use a different store path

### Invalid YAML syntax

```
Error: config load failed: yaml: unmarshal errors
```

**Solution:** Validate YAML syntax at https://www.yamllint.com/

### Environment variable not expanding

```
OpenAI: No API key provided, using static client
```

If you have `apiKey: "${OPENAI_API_KEY}"` in your config but it's not working:

**Solution:**
1. Check if the environment variable is actually set:
   ```bash
   echo $OPENAI_API_KEY
   ```
2. If empty, export it before running the tool:
   ```bash
   export OPENAI_API_KEY="sk-your-key-here"
   ./cr review branch main --target HEAD
   ```
3. Ensure the variable name is uppercase (lowercase variables won't be expanded)
4. Rebuild the application after updating config code: `go build ./cmd/cr`

## Next Steps

- See [USAGE.md](./USAGE.md) for running reviews
- See [OBSERVABILITY.md](./OBSERVABILITY.md) for logging and metrics details
- See [COST_TRACKING.md](./COST_TRACKING.md) for cost analysis and optimization
- See [MAIN_INTEGRATION_CHECKLIST.md](./MAIN_INTEGRATION_CHECKLIST.md) for store features
- See [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) for roadmap
