package config

// Config represents the full application configuration.
type Config struct {
	Providers   map[string]ProviderConfig `yaml:"providers"`
	Merge       MergeConfig               `yaml:"merge"`
	Git         GitConfig                 `yaml:"git"`
	Output      OutputConfig              `yaml:"output"`
	Budget      BudgetConfig              `yaml:"budget"`
	Redaction   RedactionConfig           `yaml:"redaction"`
	Determinism DeterminismConfig         `yaml:"determinism"`
	Store       StoreConfig               `yaml:"store"`
}

// ProviderConfig configures a single LLM provider.
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`
	APIKey  string `yaml:"apiKey"`
}

type MergeConfig struct {
	Enabled  bool               `yaml:"enabled"`
	Provider string             `yaml:"provider"`
	Model    string             `yaml:"model"`
	Strategy string             `yaml:"strategy"`
	Weights  map[string]float64 `yaml:"weights"`
}

type GitConfig struct {
	RepositoryDir string `yaml:"repositoryDir"`
}

type OutputConfig struct {
	Directory string `yaml:"directory"`
}

type BudgetConfig struct {
	HardCapUSD        float64  `yaml:"hardCapUSD"`
	DegradationPolicy []string `yaml:"degradationPolicy"`
}

type RedactionConfig struct {
	Enabled    bool     `yaml:"enabled"`
	DenyGlobs  []string `yaml:"denyGlobs"`
	AllowGlobs []string `yaml:"allowGlobs"`
}

type DeterminismConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Temperature float64 `yaml:"temperature"`
	UseSeed     bool    `yaml:"useSeed"`
}

// StoreConfig configures the persistence layer.
type StoreConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Merge combines multiple configuration instances, prioritising the latter ones.
func Merge(configs ...Config) Config {
	result := Config{}
	for _, cfg := range configs {
		result = merge(result, cfg)
	}
	return result
}

func merge(base, overlay Config) Config {
	result := base

	result.Output = chooseOutput(base.Output, overlay.Output)
	result.Git = chooseGit(base.Git, overlay.Git)
	result.Budget = chooseBudget(base.Budget, overlay.Budget)
	result.Redaction = chooseRedaction(base.Redaction, overlay.Redaction)
	result.Determinism = chooseDeterminism(base.Determinism, overlay.Determinism)
	result.Merge = chooseMerge(base.Merge, overlay.Merge)
	result.Store = chooseStore(base.Store, overlay.Store)
	result.Providers = mergeProviders(base.Providers, overlay.Providers)

	return result
}

func mergeProviders(base, overlay map[string]ProviderConfig) map[string]ProviderConfig {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	result := make(map[string]ProviderConfig, len(base)+len(overlay))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		result[key] = value
	}
	return result
}

func chooseOutput(base, overlay OutputConfig) OutputConfig {
	if overlay.Directory != "" {
		return overlay
	}
	return base
}

func chooseGit(base, overlay GitConfig) GitConfig {
	if overlay.RepositoryDir != "" {
		return overlay
	}
	return base
}

func chooseBudget(base, overlay BudgetConfig) BudgetConfig {
	if overlay.HardCapUSD != 0 || len(overlay.DegradationPolicy) > 0 {
		return overlay
	}
	return base
}

func chooseRedaction(base, overlay RedactionConfig) RedactionConfig {
	if overlay.Enabled || len(overlay.DenyGlobs) > 0 || len(overlay.AllowGlobs) > 0 {
		return overlay
	}
	return base
}

func chooseDeterminism(base, overlay DeterminismConfig) DeterminismConfig {
	if overlay.Enabled || overlay.Temperature != 0 || overlay.UseSeed {
		return overlay
	}
	return base
}

func chooseMerge(base, overlay MergeConfig) MergeConfig {
	if overlay.Enabled || overlay.Provider != "" || overlay.Model != "" || overlay.Strategy != "" || len(overlay.Weights) > 0 {
		return overlay
	}
	return base
}

func chooseStore(base, overlay StoreConfig) StoreConfig {
	if overlay.Enabled || overlay.Path != "" {
		return overlay
	}
	return base
}
