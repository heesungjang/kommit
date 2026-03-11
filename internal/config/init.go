package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var cfg Config

// Load reads configuration from multiple paths, merging them.
// Supports both YAML (.opengit.yml / .opengit.yaml) and JSON (.opengit.json)
// formats. Later paths override earlier ones.
func Load() (Config, error) {
	cfg = DefaultConfig()

	// Build search paths (lowest to highest priority).
	var searchPaths []string
	if home, err := os.UserHomeDir(); err == nil {
		searchPaths = append(searchPaths, filepath.Join(home, ".config", "opengit"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		searchPaths = append(searchPaths, filepath.Join(xdg, "opengit"))
	}
	searchPaths = append(searchPaths, ".")

	// Try config file names in priority order: YAML first, then JSON.
	// Viper resolves the first file it finds across all search paths.
	configNames := []struct {
		name string
		typ  string
	}{
		{"opengit", "yaml"},  // opengit.yaml / opengit.yml
		{".opengit", "yaml"}, // .opengit.yaml / .opengit.yml
		{".opengit", "json"}, // .opengit.json (legacy)
	}

	var loaded bool
	for _, cn := range configNames {
		v := viper.New()
		v.SetConfigName(cn.name)
		v.SetConfigType(cn.typ)
		for _, p := range searchPaths {
			v.AddConfigPath(p)
		}
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				continue // try next name
			}
			return cfg, err
		}
		if err := v.Unmarshal(&cfg); err != nil {
			return cfg, err
		}
		loaded = true
		break
	}

	// If no file found, use defaults — also set up env overrides.
	if !loaded {
		v := viper.New()
		v.SetEnvPrefix("OPENGIT")
		v.AutomaticEnv()
	}

	// Override AI API key from environment
	if key := os.Getenv("OPENGIT_AI_API_KEY"); key != "" {
		cfg.AI.APIKey = key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" && cfg.AI.Provider == "anthropic" {
		cfg.AI.APIKey = key
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" && cfg.AI.Provider == "openai" {
		cfg.AI.APIKey = key
	}

	// Override hosting token from environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cfg.Hosting.Token = token
	}
	if token := os.Getenv("GITLAB_TOKEN"); token != "" && cfg.Hosting.Provider == "gitlab" {
		cfg.Hosting.Token = token
	}

	return cfg, nil
}

// Get returns the current loaded configuration.
func Get() Config {
	return cfg
}
