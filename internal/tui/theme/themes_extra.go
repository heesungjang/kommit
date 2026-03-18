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

// SolarizedDark returns the Solarized Dark theme — Ethan Schoonover's precision
// color palette with a blue-green base and carefully chosen accent colors.
func SolarizedDark() Theme {
	return Theme{
		Name:     "solarized-dark",
		Base:     lipgloss.Color("#002b36"),
		Mantle:   lipgloss.Color("#00252f"),
		Crust:    lipgloss.Color("#001f27"),
		Surface0: lipgloss.Color("#073642"),
		Surface1: lipgloss.Color("#0d4150"),
		Surface2: lipgloss.Color("#586e75"),
		Overlay0: lipgloss.Color("#657b83"),
		Overlay1: lipgloss.Color("#839496"),
		Text:     lipgloss.Color("#fdf6e3"),
		Subtext0: lipgloss.Color("#93a1a1"),
		Subtext1: lipgloss.Color("#eee8d5"),

		Red:       lipgloss.Color("#dc322f"),
		Green:     lipgloss.Color("#859900"),
		Yellow:    lipgloss.Color("#b58900"),
		Blue:      lipgloss.Color("#268bd2"),
		Mauve:     lipgloss.Color("#6c71c4"),
		Pink:      lipgloss.Color("#d33682"),
		Teal:      lipgloss.Color("#2aa198"),
		Sky:       lipgloss.Color("#2aa198"),
		Peach:     lipgloss.Color("#cb4b16"),
		Maroon:    lipgloss.Color("#dc322f"),
		Lavender:  lipgloss.Color("#8894c7"),
		Flamingo:  lipgloss.Color("#d33682"),
		Rosewater: lipgloss.Color("#eee8d5"),
		Sapphire:  lipgloss.Color("#268bd2"),

		DiffAddedLineBg:   lipgloss.Color("#0a3a2a"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#2a1a2a"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#073642"), // Surface0
	}
}

// SolarizedLight returns the Solarized Light theme — the light variant of
// Ethan Schoonover's precision palette with a warm ivory base.
func SolarizedLight() Theme {
	return Theme{
		Name:     "solarized-light",
		Base:     lipgloss.Color("#fdf6e3"),
		Mantle:   lipgloss.Color("#eee8d5"),
		Crust:    lipgloss.Color("#e0dcc7"),
		Surface0: lipgloss.Color("#eee8d5"),
		Surface1: lipgloss.Color("#ddd6c1"),
		Surface2: lipgloss.Color("#93a1a1"),
		Overlay0: lipgloss.Color("#839496"),
		Overlay1: lipgloss.Color("#657b83"),
		Text:     lipgloss.Color("#073642"),
		Subtext0: lipgloss.Color("#586e75"),
		Subtext1: lipgloss.Color("#002b36"),

		Red:       lipgloss.Color("#dc322f"),
		Green:     lipgloss.Color("#859900"),
		Yellow:    lipgloss.Color("#b58900"),
		Blue:      lipgloss.Color("#268bd2"),
		Mauve:     lipgloss.Color("#6c71c4"),
		Pink:      lipgloss.Color("#d33682"),
		Teal:      lipgloss.Color("#2aa198"),
		Sky:       lipgloss.Color("#2aa198"),
		Peach:     lipgloss.Color("#cb4b16"),
		Maroon:    lipgloss.Color("#dc322f"),
		Lavender:  lipgloss.Color("#8894c7"),
		Flamingo:  lipgloss.Color("#d33682"),
		Rosewater: lipgloss.Color("#eee8d5"),
		Sapphire:  lipgloss.Color("#268bd2"),

		DiffAddedLineBg:   lipgloss.Color("#e6edd5"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#f0ddd5"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#eee8d5"), // Surface0
	}
}

// OneDark returns the One Dark theme — the iconic Atom editor theme with
// a rich, vibrant palette against a cool dark background.
func OneDark() Theme {
	return Theme{
		Name:     "one-dark",
		Base:     lipgloss.Color("#282c34"),
		Mantle:   lipgloss.Color("#21252b"),
		Crust:    lipgloss.Color("#1b1f27"),
		Surface0: lipgloss.Color("#2c313a"),
		Surface1: lipgloss.Color("#353b45"),
		Surface2: lipgloss.Color("#3e4451"),
		Overlay0: lipgloss.Color("#545862"),
		Overlay1: lipgloss.Color("#636d83"),
		Text:     lipgloss.Color("#abb2bf"),
		Subtext0: lipgloss.Color("#828997"),
		Subtext1: lipgloss.Color("#9da5b4"),

		Red:       lipgloss.Color("#e06c75"),
		Green:     lipgloss.Color("#98c379"),
		Yellow:    lipgloss.Color("#e5c07b"),
		Blue:      lipgloss.Color("#61afef"),
		Mauve:     lipgloss.Color("#c678dd"),
		Pink:      lipgloss.Color("#c678dd"),
		Teal:      lipgloss.Color("#56b6c2"),
		Sky:       lipgloss.Color("#56b6c2"),
		Peach:     lipgloss.Color("#d19a66"),
		Maroon:    lipgloss.Color("#be5046"),
		Lavender:  lipgloss.Color("#a9b2d0"),
		Flamingo:  lipgloss.Color("#e06c75"),
		Rosewater: lipgloss.Color("#d7c4b1"),
		Sapphire:  lipgloss.Color("#61afef"),

		DiffAddedLineBg:   lipgloss.Color("#2d3b2e"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#3b2c30"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#2c313a"), // Surface0
	}
}

// GitHubDark returns the GitHub Dark theme — GitHub's default dark mode
// palette with a blue-gray base and familiar accent colors.
func GitHubDark() Theme {
	return Theme{
		Name:     "github-dark",
		Base:     lipgloss.Color("#0d1117"),
		Mantle:   lipgloss.Color("#010409"),
		Crust:    lipgloss.Color("#000000"),
		Surface0: lipgloss.Color("#161b22"),
		Surface1: lipgloss.Color("#21262d"),
		Surface2: lipgloss.Color("#30363d"),
		Overlay0: lipgloss.Color("#484f58"),
		Overlay1: lipgloss.Color("#6e7681"),
		Text:     lipgloss.Color("#e6edf3"),
		Subtext0: lipgloss.Color("#8b949e"),
		Subtext1: lipgloss.Color("#c9d1d9"),

		Red:       lipgloss.Color("#ff7b72"),
		Green:     lipgloss.Color("#3fb950"),
		Yellow:    lipgloss.Color("#d29922"),
		Blue:      lipgloss.Color("#58a6ff"),
		Mauve:     lipgloss.Color("#bc8cff"),
		Pink:      lipgloss.Color("#f778ba"),
		Teal:      lipgloss.Color("#39d353"),
		Sky:       lipgloss.Color("#79c0ff"),
		Peach:     lipgloss.Color("#d29922"),
		Maroon:    lipgloss.Color("#f85149"),
		Lavender:  lipgloss.Color("#a5b4fc"),
		Flamingo:  lipgloss.Color("#ffa198"),
		Rosewater: lipgloss.Color("#ffd7d5"),
		Sapphire:  lipgloss.Color("#58a6ff"),

		DiffAddedLineBg:   lipgloss.Color("#12261e"), // GitHub's diff green
		DiffRemovedLineBg: lipgloss.Color("#2d1316"), // GitHub's diff red
		DiffHunkHeaderBg:  lipgloss.Color("#161b22"), // Surface0
	}
}

// GitHubLight returns the GitHub Light theme — GitHub's default light mode
// palette with a clean white base.
func GitHubLight() Theme {
	return Theme{
		Name:     "github-light",
		Base:     lipgloss.Color("#ffffff"),
		Mantle:   lipgloss.Color("#f6f8fa"),
		Crust:    lipgloss.Color("#eaeef2"),
		Surface0: lipgloss.Color("#f6f8fa"),
		Surface1: lipgloss.Color("#eaeef2"),
		Surface2: lipgloss.Color("#d0d7de"),
		Overlay0: lipgloss.Color("#afb8c1"),
		Overlay1: lipgloss.Color("#6e7781"),
		Text:     lipgloss.Color("#1f2328"),
		Subtext0: lipgloss.Color("#656d76"),
		Subtext1: lipgloss.Color("#424a53"),

		Red:       lipgloss.Color("#cf222e"),
		Green:     lipgloss.Color("#1a7f37"),
		Yellow:    lipgloss.Color("#9a6700"),
		Blue:      lipgloss.Color("#0969da"),
		Mauve:     lipgloss.Color("#8250df"),
		Pink:      lipgloss.Color("#bf3989"),
		Teal:      lipgloss.Color("#1b7c83"),
		Sky:       lipgloss.Color("#218bff"),
		Peach:     lipgloss.Color("#bc4c00"),
		Maroon:    lipgloss.Color("#a40e26"),
		Lavender:  lipgloss.Color("#6e77db"),
		Flamingo:  lipgloss.Color("#cf222e"),
		Rosewater: lipgloss.Color("#e8c4b8"),
		Sapphire:  lipgloss.Color("#0550ae"),

		DiffAddedLineBg:   lipgloss.Color("#dafbe1"), // GitHub's diff green
		DiffRemovedLineBg: lipgloss.Color("#ffebe9"), // GitHub's diff red
		DiffHunkHeaderBg:  lipgloss.Color("#f6f8fa"), // Surface0
	}
}

// GruvboxLight returns the Gruvbox Light theme — the warm retro palette
// adapted for light backgrounds with earthy tones.
func GruvboxLight() Theme {
	return Theme{
		Name:     "gruvbox-light",
		Base:     lipgloss.Color("#fbf1c7"),
		Mantle:   lipgloss.Color("#f2e5bc"),
		Crust:    lipgloss.Color("#ebdbb2"),
		Surface0: lipgloss.Color("#ebdbb2"),
		Surface1: lipgloss.Color("#d5c4a1"),
		Surface2: lipgloss.Color("#bdae93"),
		Overlay0: lipgloss.Color("#a89984"),
		Overlay1: lipgloss.Color("#928374"),
		Text:     lipgloss.Color("#282828"),
		Subtext0: lipgloss.Color("#504945"),
		Subtext1: lipgloss.Color("#3c3836"),

		Red:       lipgloss.Color("#cc241d"),
		Green:     lipgloss.Color("#98971a"),
		Yellow:    lipgloss.Color("#d79921"),
		Blue:      lipgloss.Color("#458588"),
		Mauve:     lipgloss.Color("#b16286"),
		Pink:      lipgloss.Color("#b16286"),
		Teal:      lipgloss.Color("#689d6a"),
		Sky:       lipgloss.Color("#458588"),
		Peach:     lipgloss.Color("#d65d0e"),
		Maroon:    lipgloss.Color("#9d0006"),
		Lavender:  lipgloss.Color("#8f7497"),
		Flamingo:  lipgloss.Color("#d65d0e"),
		Rosewater: lipgloss.Color("#d5c4a1"),
		Sapphire:  lipgloss.Color("#076678"),

		DiffAddedLineBg:   lipgloss.Color("#e5e8c0"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#f0d3c3"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#ebdbb2"), // Surface0
	}
}

// EverforestDark returns the Everforest Dark theme — a green-tinted palette
// designed for comfortable, low-contrast coding inspired by natural forests.
func EverforestDark() Theme {
	return Theme{
		Name:     "everforest-dark",
		Base:     lipgloss.Color("#2d353b"),
		Mantle:   lipgloss.Color("#272e33"),
		Crust:    lipgloss.Color("#232a2e"),
		Surface0: lipgloss.Color("#343f44"),
		Surface1: lipgloss.Color("#3d484d"),
		Surface2: lipgloss.Color("#475258"),
		Overlay0: lipgloss.Color("#59656b"),
		Overlay1: lipgloss.Color("#6c787e"),
		Text:     lipgloss.Color("#d3c6aa"),
		Subtext0: lipgloss.Color("#9da9a0"),
		Subtext1: lipgloss.Color("#b5c1b3"),

		Red:       lipgloss.Color("#e67e80"),
		Green:     lipgloss.Color("#a7c080"),
		Yellow:    lipgloss.Color("#dbbc7f"),
		Blue:      lipgloss.Color("#7fbbb3"),
		Mauve:     lipgloss.Color("#d699b6"),
		Pink:      lipgloss.Color("#d699b6"),
		Teal:      lipgloss.Color("#83c092"),
		Sky:       lipgloss.Color("#7fbbb3"),
		Peach:     lipgloss.Color("#e69875"),
		Maroon:    lipgloss.Color("#c85552"),
		Lavender:  lipgloss.Color("#b3a6c9"),
		Flamingo:  lipgloss.Color("#e69875"),
		Rosewater: lipgloss.Color("#e0c8a8"),
		Sapphire:  lipgloss.Color("#7fbbb3"),

		DiffAddedLineBg:   lipgloss.Color("#354038"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#3e3438"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#343f44"), // Surface0
	}
}

// Nightfox returns the Nightfox theme — a calm, dark blue theme from the
// popular Neovim nightfox.nvim plugin.
func Nightfox() Theme {
	return Theme{
		Name:     "nightfox",
		Base:     lipgloss.Color("#192330"),
		Mantle:   lipgloss.Color("#131a24"),
		Crust:    lipgloss.Color("#0e151e"),
		Surface0: lipgloss.Color("#212e3f"),
		Surface1: lipgloss.Color("#29394f"),
		Surface2: lipgloss.Color("#39506d"),
		Overlay0: lipgloss.Color("#526482"),
		Overlay1: lipgloss.Color("#668199"),
		Text:     lipgloss.Color("#cdcecf"),
		Subtext0: lipgloss.Color("#aeafb0"),
		Subtext1: lipgloss.Color("#c0c1c2"),

		Red:       lipgloss.Color("#c94f6d"),
		Green:     lipgloss.Color("#81b29a"),
		Yellow:    lipgloss.Color("#dbc074"),
		Blue:      lipgloss.Color("#719cd6"),
		Mauve:     lipgloss.Color("#9d79d6"),
		Pink:      lipgloss.Color("#d67ad2"),
		Teal:      lipgloss.Color("#63cdcf"),
		Sky:       lipgloss.Color("#719cd6"),
		Peach:     lipgloss.Color("#f4a261"),
		Maroon:    lipgloss.Color("#a84040"),
		Lavender:  lipgloss.Color("#a7b0d6"),
		Flamingo:  lipgloss.Color("#f4a261"),
		Rosewater: lipgloss.Color("#e8cfc0"),
		Sapphire:  lipgloss.Color("#6085b6"),

		DiffAddedLineBg:   lipgloss.Color("#22343a"), // 20% green
		DiffRemovedLineBg: lipgloss.Color("#2d2333"), // 20% red
		DiffHunkHeaderBg:  lipgloss.Color("#212e3f"), // Surface0
	}
}

func init() {
	Themes["tokyo-night"] = TokyoNight
	Themes["dracula"] = Dracula
	Themes["nord"] = Nord
	Themes["gruvbox-dark"] = GruvboxDark
	Themes["gruvbox-light"] = GruvboxLight
	Themes["rose-pine"] = RosePine
	Themes["kanagawa-wave"] = KanagawaWave
	Themes["solarized-dark"] = SolarizedDark
	Themes["solarized-light"] = SolarizedLight
	Themes["one-dark"] = OneDark
	Themes["github-dark"] = GitHubDark
	Themes["github-light"] = GitHubLight
	Themes["everforest-dark"] = EverforestDark
	Themes["nightfox"] = Nightfox
}
