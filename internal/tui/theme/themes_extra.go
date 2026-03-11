package theme

import "github.com/charmbracelet/lipgloss"

// TokyoNight returns the Tokyo Night (Storm) theme — a dark blue-purple palette
// inspired by the lights of downtown Tokyo.
func TokyoNight() Theme {
	return Theme{
		Name:     "tokyo-night",
		Base:     lipgloss.Color("#222436"),
		Mantle:   lipgloss.Color("#1e2030"),
		Crust:    lipgloss.Color("#191b28"),
		Surface0: lipgloss.Color("#2f334d"),
		Surface1: lipgloss.Color("#383c5a"),
		Surface2: lipgloss.Color("#414566"),
		Overlay0: lipgloss.Color("#545c7e"),
		Overlay1: lipgloss.Color("#636da6"),
		Text:     lipgloss.Color("#c8d3f5"),
		Subtext0: lipgloss.Color("#a9b1d6"),
		Subtext1: lipgloss.Color("#b4bcd0"),

		Red:       lipgloss.Color("#ff757f"),
		Green:     lipgloss.Color("#c3e88d"),
		Yellow:    lipgloss.Color("#ffc777"),
		Blue:      lipgloss.Color("#82aaff"),
		Mauve:     lipgloss.Color("#c099ff"),
		Pink:      lipgloss.Color("#fca7ea"),
		Teal:      lipgloss.Color("#4fd6be"),
		Sky:       lipgloss.Color("#86e1fc"),
		Peach:     lipgloss.Color("#ff966c"),
		Maroon:    lipgloss.Color("#c53b53"),
		Lavender:  lipgloss.Color("#b4befe"),
		Flamingo:  lipgloss.Color("#f7768e"),
		Rosewater: lipgloss.Color("#ffc0b9"),
		Sapphire:  lipgloss.Color("#7dcfff"),

		// Diff backgrounds: ~20% accent blended with base #222436
		DiffAddedLineBg:   lipgloss.Color("#2b3a3a"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#3b2b3a"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#2f334d"), // Surface0
	}
}

// Dracula returns the Dracula theme — a dark purple palette known for its
// distinctive pink, purple, and green accent colors.
func Dracula() Theme {
	return Theme{
		Name:     "dracula",
		Base:     lipgloss.Color("#282a36"),
		Mantle:   lipgloss.Color("#21222c"),
		Crust:    lipgloss.Color("#191a21"),
		Surface0: lipgloss.Color("#343746"),
		Surface1: lipgloss.Color("#3e4254"),
		Surface2: lipgloss.Color("#4d5168"),
		Overlay0: lipgloss.Color("#6272a4"),
		Overlay1: lipgloss.Color("#7082b4"),
		Text:     lipgloss.Color("#f8f8f2"),
		Subtext0: lipgloss.Color("#c0c0b8"),
		Subtext1: lipgloss.Color("#d8d8d2"),

		Red:       lipgloss.Color("#ff5555"),
		Green:     lipgloss.Color("#50fa7b"),
		Yellow:    lipgloss.Color("#f1fa8c"),
		Blue:      lipgloss.Color("#6272a4"),
		Mauve:     lipgloss.Color("#bd93f9"),
		Pink:      lipgloss.Color("#ff79c6"),
		Teal:      lipgloss.Color("#8be9fd"),
		Sky:       lipgloss.Color("#8be9fd"),
		Peach:     lipgloss.Color("#ffb86c"),
		Maroon:    lipgloss.Color("#ff6e6e"),
		Lavender:  lipgloss.Color("#d6acff"),
		Flamingo:  lipgloss.Color("#ff79c6"),
		Rosewater: lipgloss.Color("#ffcce6"),
		Sapphire:  lipgloss.Color("#69b4f0"),

		// Diff backgrounds: ~20% accent blended with base #282a36
		DiffAddedLineBg:   lipgloss.Color("#2d3e3c"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#3e2a36"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#343746"), // Surface0
	}
}

// Nord returns the Nord theme — an arctic, north-bluish color palette
// with cool, muted tones.
func Nord() Theme {
	return Theme{
		Name:     "nord",
		Base:     lipgloss.Color("#2e3440"),
		Mantle:   lipgloss.Color("#272c36"),
		Crust:    lipgloss.Color("#21262e"),
		Surface0: lipgloss.Color("#3b4252"),
		Surface1: lipgloss.Color("#434c5e"),
		Surface2: lipgloss.Color("#4c566a"),
		Overlay0: lipgloss.Color("#616e88"),
		Overlay1: lipgloss.Color("#6e7d99"),
		Text:     lipgloss.Color("#d8dee9"),
		Subtext0: lipgloss.Color("#b0b8c8"),
		Subtext1: lipgloss.Color("#c4ccd8"),

		Red:       lipgloss.Color("#bf616a"),
		Green:     lipgloss.Color("#a3be8c"),
		Yellow:    lipgloss.Color("#ebcb8b"),
		Blue:      lipgloss.Color("#88c0d0"),
		Mauve:     lipgloss.Color("#b48ead"),
		Pink:      lipgloss.Color("#c2a1bd"),
		Teal:      lipgloss.Color("#8fbcbb"),
		Sky:       lipgloss.Color("#88c0d0"),
		Peach:     lipgloss.Color("#d08770"),
		Maroon:    lipgloss.Color("#bf616a"),
		Lavender:  lipgloss.Color("#b8c5db"),
		Flamingo:  lipgloss.Color("#d08770"),
		Rosewater: lipgloss.Color("#e5c7a0"),
		Sapphire:  lipgloss.Color("#81a1c1"),

		// Diff backgrounds: ~20% accent blended with base #2e3440
		DiffAddedLineBg:   lipgloss.Color("#384249"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#3d3740"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#3b4252"), // Surface0
	}
}

// GruvboxDark returns the Gruvbox Dark theme — a warm retro palette with
// earthy tones and high contrast accents.
func GruvboxDark() Theme {
	return Theme{
		Name:     "gruvbox-dark",
		Base:     lipgloss.Color("#282828"),
		Mantle:   lipgloss.Color("#1d2021"),
		Crust:    lipgloss.Color("#171717"),
		Surface0: lipgloss.Color("#3c3836"),
		Surface1: lipgloss.Color("#504945"),
		Surface2: lipgloss.Color("#665c54"),
		Overlay0: lipgloss.Color("#7c6f64"),
		Overlay1: lipgloss.Color("#928374"),
		Text:     lipgloss.Color("#ebdbb2"),
		Subtext0: lipgloss.Color("#bdae93"),
		Subtext1: lipgloss.Color("#d5c4a1"),

		Red:       lipgloss.Color("#fb4934"),
		Green:     lipgloss.Color("#b8bb26"),
		Yellow:    lipgloss.Color("#fabd2f"),
		Blue:      lipgloss.Color("#83a598"),
		Mauve:     lipgloss.Color("#d3869b"),
		Pink:      lipgloss.Color("#d3869b"),
		Teal:      lipgloss.Color("#8ec07c"),
		Sky:       lipgloss.Color("#83a598"),
		Peach:     lipgloss.Color("#fe8019"),
		Maroon:    lipgloss.Color("#cc241d"),
		Lavender:  lipgloss.Color("#b8a5c4"),
		Flamingo:  lipgloss.Color("#fe8019"),
		Rosewater: lipgloss.Color("#d5c4a1"),
		Sapphire:  lipgloss.Color("#689d6a"),

		// Diff backgrounds: ~20% accent blended with base #282828
		DiffAddedLineBg:   lipgloss.Color("#343a28"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#3e2828"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#3c3836"), // Surface0
	}
}

// RosePine returns the Rosé Pine theme — a soft, muted dark theme with warm
// rose and gold accents evoking natural, understated elegance.
func RosePine() Theme {
	return Theme{
		Name:     "rose-pine",
		Base:     lipgloss.Color("#191724"),
		Mantle:   lipgloss.Color("#15121f"),
		Crust:    lipgloss.Color("#100e1a"),
		Surface0: lipgloss.Color("#1f1d2e"),
		Surface1: lipgloss.Color("#26233a"),
		Surface2: lipgloss.Color("#2e2b42"),
		Overlay0: lipgloss.Color("#6e6a86"),
		Overlay1: lipgloss.Color("#908caa"),
		Text:     lipgloss.Color("#e0def4"),
		Subtext0: lipgloss.Color("#b0aecb"),
		Subtext1: lipgloss.Color("#c8c6e0"),

		Red:       lipgloss.Color("#eb6f92"),
		Green:     lipgloss.Color("#9ccfd8"),
		Yellow:    lipgloss.Color("#f6c177"),
		Blue:      lipgloss.Color("#31748f"),
		Mauve:     lipgloss.Color("#c4a7e7"),
		Pink:      lipgloss.Color("#ebbcba"),
		Teal:      lipgloss.Color("#9ccfd8"),
		Sky:       lipgloss.Color("#9ccfd8"),
		Peach:     lipgloss.Color("#f6c177"),
		Maroon:    lipgloss.Color("#eb6f92"),
		Lavender:  lipgloss.Color("#c4a7e7"),
		Flamingo:  lipgloss.Color("#ebbcba"),
		Rosewater: lipgloss.Color("#f2dede"),
		Sapphire:  lipgloss.Color("#31748f"),

		// Diff backgrounds: ~20% accent blended with base #191724
		DiffAddedLineBg:   lipgloss.Color("#232d35"), // 20% foam/green
		DiffRemovedLineBg: lipgloss.Color("#301e30"), // 20% love/red
		DiffHunkHeaderBg:  lipgloss.Color("#1f1d2e"), // Surface0
	}
}

// KanagawaWave returns the Kanagawa Wave theme — a dark palette inspired by
// Katsushika Hokusai's "The Great Wave off Kanagawa", with ink-wash tones
// and warm, muted accent colors.
func KanagawaWave() Theme {
	return Theme{
		Name:     "kanagawa-wave",
		Base:     lipgloss.Color("#1f1f28"),
		Mantle:   lipgloss.Color("#1a1a22"),
		Crust:    lipgloss.Color("#16161d"),
		Surface0: lipgloss.Color("#2a2a37"),
		Surface1: lipgloss.Color("#363646"),
		Surface2: lipgloss.Color("#414154"),
		Overlay0: lipgloss.Color("#54546d"),
		Overlay1: lipgloss.Color("#727169"),
		Text:     lipgloss.Color("#dcd7ba"),
		Subtext0: lipgloss.Color("#a6a18a"),
		Subtext1: lipgloss.Color("#c0bca0"),

		Red:       lipgloss.Color("#e46876"),
		Green:     lipgloss.Color("#98bb6c"),
		Yellow:    lipgloss.Color("#e6c384"),
		Blue:      lipgloss.Color("#7e9cd8"),
		Mauve:     lipgloss.Color("#957fb8"),
		Pink:      lipgloss.Color("#d27e99"),
		Teal:      lipgloss.Color("#7fb4ca"),
		Sky:       lipgloss.Color("#7fb4ca"),
		Peach:     lipgloss.Color("#ffa066"),
		Maroon:    lipgloss.Color("#c34043"),
		Lavender:  lipgloss.Color("#9cabca"),
		Flamingo:  lipgloss.Color("#e46876"),
		Rosewater: lipgloss.Color("#e8cfc0"),
		Sapphire:  lipgloss.Color("#658594"),

		// Diff backgrounds: ~20% accent blended with base #1f1f28
		DiffAddedLineBg:   lipgloss.Color("#2d3536"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#362430"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#2a2a37"), // Surface0
	}
}

func init() {
	Themes["tokyo-night"] = TokyoNight
	Themes["dracula"] = Dracula
	Themes["nord"] = Nord
	Themes["gruvbox-dark"] = GruvboxDark
	Themes["rose-pine"] = RosePine
	Themes["kanagawa-wave"] = KanagawaWave
}
