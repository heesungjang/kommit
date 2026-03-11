package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// CatppuccinMocha returns the Catppuccin Mocha (dark) theme.
func CatppuccinMocha() Theme {
	return Theme{
		Name:      "catppuccin-mocha",
		Base:      lipgloss.Color("#1e1e2e"),
		Mantle:    lipgloss.Color("#181825"),
		Crust:     lipgloss.Color("#11111b"),
		Surface0:  lipgloss.Color("#313244"),
		Surface1:  lipgloss.Color("#45475a"),
		Surface2:  lipgloss.Color("#585b70"),
		Overlay0:  lipgloss.Color("#6c7086"),
		Overlay1:  lipgloss.Color("#7f849c"),
		Text:      lipgloss.Color("#cdd6f4"),
		Subtext0:  lipgloss.Color("#a6adc8"),
		Subtext1:  lipgloss.Color("#bac2de"),
		Red:       lipgloss.Color("#f38ba8"),
		Green:     lipgloss.Color("#a6e3a1"),
		Yellow:    lipgloss.Color("#f9e2af"),
		Blue:      lipgloss.Color("#89b4fa"),
		Mauve:     lipgloss.Color("#cba6f7"),
		Pink:      lipgloss.Color("#f5c2e7"),
		Teal:      lipgloss.Color("#94e2d5"),
		Sky:       lipgloss.Color("#89dceb"),
		Peach:     lipgloss.Color("#fab387"),
		Maroon:    lipgloss.Color("#eba0ac"),
		Lavender:  lipgloss.Color("#b4befe"),
		Flamingo:  lipgloss.Color("#f2cdcd"),
		Rosewater: lipgloss.Color("#f5e0dc"),
		Sapphire:  lipgloss.Color("#74c7ec"),
		// Diff backgrounds: 20% accent blended with base #1e1e2e
		DiffAddedLineBg:   lipgloss.Color("#394545"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#493447"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#313244"), // Surface0
	}
}

// CatppuccinLatte returns the Catppuccin Latte (light) theme.
func CatppuccinLatte() Theme {
	return Theme{
		Name:      "catppuccin-latte",
		Base:      lipgloss.Color("#eff1f5"),
		Mantle:    lipgloss.Color("#e6e9ef"),
		Crust:     lipgloss.Color("#dce0e8"),
		Surface0:  lipgloss.Color("#ccd0da"),
		Surface1:  lipgloss.Color("#bcc0cc"),
		Surface2:  lipgloss.Color("#acb0be"),
		Overlay0:  lipgloss.Color("#9ca0b0"),
		Overlay1:  lipgloss.Color("#8c8fa1"),
		Text:      lipgloss.Color("#4c4f69"),
		Subtext0:  lipgloss.Color("#6c6f85"),
		Subtext1:  lipgloss.Color("#5c5f77"),
		Red:       lipgloss.Color("#d20f39"),
		Green:     lipgloss.Color("#40a02b"),
		Yellow:    lipgloss.Color("#df8e1d"),
		Blue:      lipgloss.Color("#1e66f5"),
		Mauve:     lipgloss.Color("#8839ef"),
		Pink:      lipgloss.Color("#ea76cb"),
		Teal:      lipgloss.Color("#179299"),
		Sky:       lipgloss.Color("#04a5e5"),
		Peach:     lipgloss.Color("#fe640b"),
		Maroon:    lipgloss.Color("#e64553"),
		Lavender:  lipgloss.Color("#7287fd"),
		Flamingo:  lipgloss.Color("#dd7878"),
		Rosewater: lipgloss.Color("#dc8a78"),
		Sapphire:  lipgloss.Color("#209fb5"),
		// Diff backgrounds: 20% accent blended with base #eff1f5
		DiffAddedLineBg:   lipgloss.Color("#d8ead7"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#ead4d8"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#ccd0da"), // Surface0
	}
}

// CatppuccinMacchiato returns the Catppuccin Macchiato theme.
func CatppuccinMacchiato() Theme {
	return Theme{
		Name:      "catppuccin-macchiato",
		Base:      lipgloss.Color("#24273a"),
		Mantle:    lipgloss.Color("#1e2030"),
		Crust:     lipgloss.Color("#181926"),
		Surface0:  lipgloss.Color("#363a4f"),
		Surface1:  lipgloss.Color("#494d64"),
		Surface2:  lipgloss.Color("#5b6078"),
		Overlay0:  lipgloss.Color("#6e738d"),
		Overlay1:  lipgloss.Color("#8087a2"),
		Text:      lipgloss.Color("#cad3f5"),
		Subtext0:  lipgloss.Color("#a5adcb"),
		Subtext1:  lipgloss.Color("#b8c0e0"),
		Red:       lipgloss.Color("#ed8796"),
		Green:     lipgloss.Color("#a6da95"),
		Yellow:    lipgloss.Color("#eed49f"),
		Blue:      lipgloss.Color("#8aadf4"),
		Mauve:     lipgloss.Color("#c6a0f6"),
		Pink:      lipgloss.Color("#f5bde6"),
		Teal:      lipgloss.Color("#8bd5ca"),
		Sky:       lipgloss.Color("#91d7e3"),
		Peach:     lipgloss.Color("#f5a97f"),
		Maroon:    lipgloss.Color("#ee99a0"),
		Lavender:  lipgloss.Color("#b7bdf8"),
		Flamingo:  lipgloss.Color("#f0c6c6"),
		Rosewater: lipgloss.Color("#f4dbd6"),
		Sapphire:  lipgloss.Color("#7dc4e4"),
		// Diff backgrounds: 20% accent blended with base #24273a
		DiffAddedLineBg:   lipgloss.Color("#3e4b4b"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#4e3a4d"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#363a4f"), // Surface0
	}
}

// Themes is a map of available themes.
var Themes = map[string]func() Theme{
	"catppuccin-mocha":     CatppuccinMocha,
	"catppuccin-latte":     CatppuccinLatte,
	"catppuccin-macchiato": CatppuccinMacchiato,
}

// Get returns a theme by name, defaulting to Mocha.
// The special name "auto" detects the terminal background color and selects
// catppuccin-latte (light) or catppuccin-mocha (dark) accordingly.
func Get(name string) Theme {
	if name == "auto" {
		if termenv.HasDarkBackground() {
			return CatppuccinMocha()
		}
		return CatppuccinLatte()
	}
	if fn, ok := Themes[name]; ok {
		return fn()
	}
	return CatppuccinMocha()
}
