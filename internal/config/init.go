package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var cfg Config

// Load reads configuration from multiple paths, merging them.
// Supports YAML (.kommit.yaml / .kommit.yml) and JSON (.kommit.json).
//
// Search order (lowest to highest priority):
//
//	~/.config/kommit/config.yaml    (global)
//	$XDG_CONFIG_HOME/kommit/config.yaml
//	./.kommit.yaml                  (local, dotfile to avoid binary collision)
func Load() (Config, error) {
	cfg = DefaultConfig()

	// --- Pass 1: global config dirs ---
	var globalPaths []string
	if home, err := os.UserHomeDir(); err == nil {
		globalPaths = append(globalPaths, filepath.Join(home, ".config", "kommit"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		globalPaths = append(globalPaths, filepath.Join(xdg, "kommit"))
	}

	var loaded bool
	if len(globalPaths) > 0 {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		for _, p := range globalPaths {
			v.AddConfigPath(p)
		}
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return cfg, err
			}
		} else {
			if err := v.Unmarshal(&cfg); err != nil {
				return cfg, err
			}
			loaded = true
		}
	}

	// --- Pass 2: local dotfile in current directory ---
	// Uses ".kommit" (with dot prefix) to avoid collision with the kommit binary.
	localNames := []struct {
		name string
		typ  string
	}{
		{".kommit", "yaml"}, // .kommit.yaml / .kommit.yml
		{".kommit", "json"}, // .kommit.json
	}
	for _, cn := range localNames {
		v := viper.New()
		v.SetConfigName(cn.name)
		v.SetConfigType(cn.typ)
		v.AddConfigPath(".")
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				continue
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
		v.SetEnvPrefix("KOMMIT")
		v.AutomaticEnv()
	}

	// Override AI API key from environment
	if key := os.Getenv("KOMMIT_AI_API_KEY"); key != "" {
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
