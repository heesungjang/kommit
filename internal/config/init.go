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

// Save writes the given config to the global config file at
// ~/.config/kommit/config.yaml. It creates the directory if necessary.
// Sensitive fields (API keys, tokens) that came from environment variables
// are NOT written to disk — only settings the user explicitly changed via
// the settings dialog should be persisted.
func Save(c *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "kommit")
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return mkErr
	}
	path := filepath.Join(dir, "config.yaml")

	// Build a clean struct without secrets for serialization.
	clean := *c
	clean.AI.APIKey = ""
	clean.Hosting.Token = ""

	data, err := yamlMarshal(clean)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// yamlMarshal uses gopkg.in/yaml.v3 to marshal the config to YAML.
func yamlMarshal(v any) ([]byte, error) {
	// We use a viper round-trip: set all values in a fresh Viper instance
	// and write it out. This preserves mapstructure field names.
	vp := viper.New()
	vp.SetConfigType("yaml")

	// Set each top-level key from the struct.
	cfg := v.(Config)
	vp.Set("theme", cfg.Theme)
	vp.Set("debug", cfg.Debug)
	vp.Set("ai", map[string]any{
		"provider": cfg.AI.Provider,
		"model":    cfg.AI.Model,
		"baseUrl":  cfg.AI.BaseURL,
	})
	vp.Set("hosting", map[string]any{
		"provider": cfg.Hosting.Provider,
	})
	vp.Set("appearance", map[string]any{
		"diffMode":      cfg.Appearance.DiffMode,
		"showGraph":     cfg.Appearance.ShowGraph,
		"compactLog":    cfg.Appearance.CompactLog,
		"syntaxTheme":   cfg.Appearance.SyntaxTheme,
		"nerdFonts":     cfg.Appearance.NerdFonts,
		"sidebarWidth":  cfg.Appearance.SidebarWidth,
		"sidebarMaxPct": cfg.Appearance.SidebarMaxPct,
		"centerPct":     cfg.Appearance.CenterPct,
	})
	// Only write keybinds if there are custom ones.
	if len(cfg.Keybinds.Custom) > 0 {
		vp.Set("keybinds", map[string]any{
			"custom": cfg.Keybinds.Custom,
		})
	}
	// Only write custom commands if there are any.
	if len(cfg.CustomCommands) > 0 {
		cmds := make([]map[string]any, len(cfg.CustomCommands))
		for i, cc := range cfg.CustomCommands {
			m := map[string]any{
				"name":    cc.Name,
				"command": cc.Command,
			}
			if cc.Key != "" {
				m["key"] = cc.Key
			}
			if cc.Context != "" {
				m["context"] = cc.Context
			}
			if cc.Confirm {
				m["confirm"] = cc.Confirm
			}
			if cc.ShowOutput {
				m["showOutput"] = cc.ShowOutput
			}
			if cc.Suspend {
				m["suspend"] = cc.Suspend
			}
			if cc.Description != "" {
				m["description"] = cc.Description
			}
			cmds[i] = m
		}
		vp.Set("customCommands", cmds)
	}

	// Only write workspaces if there are any.
	if len(cfg.Workspaces) > 0 {
		ws := make([]map[string]any, len(cfg.Workspaces))
		for i, w := range cfg.Workspaces {
			m := map[string]any{
				"name":  w.Name,
				"repos": w.Repos,
			}
			if w.Color != "" {
				m["color"] = w.Color
			}
			ws[i] = m
		}
		vp.Set("workspaces", ws)
	}

	// Only write recent repos if there are any.
	if len(cfg.RecentRepos) > 0 {
		vp.Set("recentRepos", cfg.RecentRepos)
	}

	// Write to a temp file and read it back to get the YAML bytes.
	tmpDir, err := os.MkdirTemp("", "kommit-cfg-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "config.yaml")
	vp.SetConfigFile(tmpPath)
	if err := vp.WriteConfig(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmpPath)
}
