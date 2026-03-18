package config

// Config is the root configuration structure.
type Config struct {
	Theme          string           `json:"theme" mapstructure:"theme"`
	Debug          bool             `json:"debug" mapstructure:"debug"`
	AI             AIConfig         `json:"ai" mapstructure:"ai"`
	Hosting        HostingConfig    `json:"hosting" mapstructure:"hosting"`
	Keybinds       KeybindConfig    `json:"keybinds" mapstructure:"keybinds"`
	Appearance     AppearanceConfig `json:"appearance" mapstructure:"appearance"`
	CustomCommands []CustomCommand  `json:"customCommands" mapstructure:"customCommands"`
	Workspaces     []WorkspaceEntry `json:"workspaces" mapstructure:"workspaces"`
	RecentRepos    []string         `json:"recentRepos" mapstructure:"recentRepos"`
}

// WorkspaceEntry defines a named group of git repositories.
type WorkspaceEntry struct {
	Name  string   `json:"name" mapstructure:"name"`   // display name (e.g. "My Microservices")
	Repos []string `json:"repos" mapstructure:"repos"` // absolute paths to repo roots
	Color string   `json:"color" mapstructure:"color"` // optional theme color name (e.g. "blue", "green")
}

// MaxRecentRepos is the maximum number of recent repos to track.
const MaxRecentRepos = 10

// AddRecentRepo prepends a repo path to the recent repos list, deduplicating
// and capping at MaxRecentRepos.
func (c *Config) AddRecentRepo(path string) {
	filtered := make([]string, 0, len(c.RecentRepos))
	for _, p := range c.RecentRepos {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	c.RecentRepos = append([]string{path}, filtered...)
	if len(c.RecentRepos) > MaxRecentRepos {
		c.RecentRepos = c.RecentRepos[:MaxRecentRepos]
	}
}

// CustomCommand defines a user-configured shell command that can be triggered
// from the TUI. Commands can reference template variables like {{.Hash}},
// {{.Branch}}, {{.Path}} which are substituted at runtime.
type CustomCommand struct {
	Name        string `json:"name" mapstructure:"name"`               // display name in menu
	Command     string `json:"command" mapstructure:"command"`         // shell command to execute
	Key         string `json:"key" mapstructure:"key"`                 // optional shortcut key (e.g. "ctrl+x")
	Context     string `json:"context" mapstructure:"context"`         // when to show: "commit", "file", "branch", "global" (default: "global")
	Confirm     bool   `json:"confirm" mapstructure:"confirm"`         // show confirm dialog before executing
	ShowOutput  bool   `json:"showOutput" mapstructure:"showOutput"`   // show command output in a toast
	Suspend     bool   `json:"suspend" mapstructure:"suspend"`         // suspend TUI while command runs (for interactive commands)
	Description string `json:"description" mapstructure:"description"` // optional description shown in menu
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
	DiffMode      string         `json:"diffMode" mapstructure:"diffMode"`           // inline, side-by-side
	ShowGraph     bool           `json:"showGraph" mapstructure:"showGraph"`         // show commit graph
	CompactLog    bool           `json:"compactLog" mapstructure:"compactLog"`       // compact commit log
	SyntaxTheme   string         `json:"syntaxTheme" mapstructure:"syntaxTheme"`     // chroma theme name
	SidebarWidth  int            `json:"sidebarWidth" mapstructure:"sidebarWidth"`   // sidebar width in columns (0 = auto)
	SidebarMaxPct int            `json:"sidebarMaxPct" mapstructure:"sidebarMaxPct"` // max sidebar width as % of terminal (0 = default 15%)
	CenterPct     int            `json:"centerPct" mapstructure:"centerPct"`         // center panel width as % of remaining (0 = default 70%)
	NerdFonts     bool           `json:"nerdFonts" mapstructure:"nerdFonts"`         // use Nerd Font glyphs for icons
	ThemeColors   ThemeOverrides `json:"themeColors" mapstructure:"themeColors"`     // override individual theme colors
}

// ThemeOverrides allows overriding individual colors in the active theme.
// Each field is a hex color string (e.g. "#ff0000"). Empty means use theme default.
type ThemeOverrides struct {
	Base     string `json:"base" mapstructure:"base"`
	Mantle   string `json:"mantle" mapstructure:"mantle"`
	Crust    string `json:"crust" mapstructure:"crust"`
	Surface0 string `json:"surface0" mapstructure:"surface0"`
	Surface1 string `json:"surface1" mapstructure:"surface1"`
	Surface2 string `json:"surface2" mapstructure:"surface2"`
	Overlay0 string `json:"overlay0" mapstructure:"overlay0"`
	Overlay1 string `json:"overlay1" mapstructure:"overlay1"`
	Text     string `json:"text" mapstructure:"text"`
	Subtext0 string `json:"subtext0" mapstructure:"subtext0"`
	Subtext1 string `json:"subtext1" mapstructure:"subtext1"`

	Red       string `json:"red" mapstructure:"red"`
	Green     string `json:"green" mapstructure:"green"`
	Yellow    string `json:"yellow" mapstructure:"yellow"`
	Blue      string `json:"blue" mapstructure:"blue"`
	Mauve     string `json:"mauve" mapstructure:"mauve"`
	Pink      string `json:"pink" mapstructure:"pink"`
	Teal      string `json:"teal" mapstructure:"teal"`
	Sky       string `json:"sky" mapstructure:"sky"`
	Peach     string `json:"peach" mapstructure:"peach"`
	Maroon    string `json:"maroon" mapstructure:"maroon"`
	Lavender  string `json:"lavender" mapstructure:"lavender"`
	Flamingo  string `json:"flamingo" mapstructure:"flamingo"`
	Rosewater string `json:"rosewater" mapstructure:"rosewater"`
	Sapphire  string `json:"sapphire" mapstructure:"sapphire"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Theme: "catppuccin-mocha",
		Debug: false,
		AI: AIConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-6",
		},
		Hosting: HostingConfig{
			Provider: "auto",
		},
		Keybinds: KeybindConfig{
			Custom: make(map[string]string),
		},
		Appearance: AppearanceConfig{
			DiffMode:      "inline",
			ShowGraph:     true,
			CompactLog:    false,
			SyntaxTheme:   "catppuccin-mocha",
			SidebarWidth:  0, // auto
			SidebarMaxPct: 0, // default 15%
			CenterPct:     0, // default 70%
		},
	}
}
