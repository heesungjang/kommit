package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var cfg Config

// Load reads configuration from multiple paths, merging them.
// Later paths override earlier ones.
func Load() (Config, error) {
	cfg = DefaultConfig()

	v := viper.New()
	v.SetConfigName(".opengit")
	v.SetConfigType("json")

	// Search paths (lowest to highest priority):
	// 1. $HOME/.config/opengit/
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "opengit"))
	}
	// 2. XDG_CONFIG_HOME/opengit/
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		v.AddConfigPath(filepath.Join(xdg, "opengit"))
	}
	// 3. Current directory
	v.AddConfigPath(".")

	// Environment variable overrides
	v.SetEnvPrefix("OPENGIT")
	v.AutomaticEnv()

	// Read config file (not an error if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return cfg, err
		}
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
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
