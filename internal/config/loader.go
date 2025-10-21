package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// LoaderOptions describes how configuration should be discovered.
type LoaderOptions struct {
	ConfigPaths []string
	FileName    string
	EnvPrefix   string
}

// Load returns the merged configuration from files and environment variables.
func Load(opts LoaderOptions) (Config, error) {
	v := viper.New()

	name := opts.FileName
	if name == "" {
		name = "cr"
	}

	configFile := locateConfigFile(name, opts.ConfigPaths)
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName(name)
	}

	prefix := opts.EnvPrefix
	if prefix == "" {
		prefix = "CR"
	}
	v.SetEnvPrefix(prefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AllowEmptyEnv(true)

	setDefaults(v)

	if configFile != "" {
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", configFile, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

func locateConfigFile(name string, paths []string) string {
	searchPaths := append([]string{}, paths...)
	searchPaths = append(searchPaths, ".")
	for _, dir := range searchPaths {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name+".yaml")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("output.directory", "out")
	v.SetDefault("determinism.enabled", true)
	v.SetDefault("determinism.temperature", 0.0)
}
