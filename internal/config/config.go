package config

// Config is the root configuration structure.
type Config struct {
	Theme      string           `json:"theme" mapstructure:"theme"`
	Debug      bool             `json:"debug" mapstructure:"debug"`
	Editor     string           `json:"editor" mapstructure:"editor"`
	AI         AIConfig         `json:"ai" mapstructure:"ai"`
	Hosting    HostingConfig    `json:"hosting" mapstructure:"hosting"`
	Keybinds   KeybindConfig    `json:"keybinds" mapstructure:"keybinds"`
	Appearance AppearanceConfig `json:"appearance" mapstructure:"appearance"`
}

// AIConfig holds AI provider settings.
type AIConfig struct {
	Provider string `json:"provider" mapstructure:"provider"` // anthropic, openai, ollama
	Model    string `json:"model" mapstructure:"model"`
	APIKey   string `json:"apiKey" mapstructure:"apiKey"`
	BaseURL  string `json:"baseUrl" mapstructure:"baseUrl"`
}

// HostingConfig holds git hosting provider settings.
type HostingConfig struct {
	Provider string `json:"provider" mapstructure:"provider"` // github, gitlab, auto
	Token    string `json:"token" mapstructure:"token"`
}

// KeybindConfig allows customizing keybindings.
type KeybindConfig struct {
	// Override defaults; map of action name -> key
	Custom map[string]string `json:"custom" mapstructure:"custom"`
}

// AppearanceConfig controls visual settings.
type AppearanceConfig struct {
	DiffMode    string `json:"diffMode" mapstructure:"diffMode"`       // inline, side-by-side
	ShowGraph   bool   `json:"showGraph" mapstructure:"showGraph"`     // show commit graph
	CompactLog  bool   `json:"compactLog" mapstructure:"compactLog"`   // compact commit log
	SyntaxTheme string `json:"syntaxTheme" mapstructure:"syntaxTheme"` // chroma theme name
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Theme:  "catppuccin-mocha",
		Debug:  false,
		Editor: "",
		AI: AIConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
		},
		Hosting: HostingConfig{
			Provider: "auto",
		},
		Keybinds: KeybindConfig{
			Custom: make(map[string]string),
		},
		Appearance: AppearanceConfig{
			DiffMode:    "inline",
			ShowGraph:   true,
			CompactLog:  false,
			SyntaxTheme: "catppuccin-mocha",
		},
	}
}
