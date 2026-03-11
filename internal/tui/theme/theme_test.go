package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestGet_KnownThemes(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"catppuccin-mocha", "catppuccin-mocha"},
		{"catppuccin-latte", "catppuccin-latte"},
		{"catppuccin-macchiato", "catppuccin-macchiato"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := Get(tt.name)
			if th.Name != tt.expected {
				t.Errorf("Get(%q).Name = %q, want %q", tt.name, th.Name, tt.expected)
			}
		})
	}
}

func TestGet_UnknownDefaultsToMocha(t *testing.T) {
	th := Get("nonexistent-theme")
	if th.Name != "catppuccin-mocha" {
		t.Errorf("Get(unknown).Name = %q, want %q", th.Name, "catppuccin-mocha")
	}
}

func TestGet_Auto(t *testing.T) {
	// Auto should return either mocha or latte (depends on terminal).
	th := Get("auto")
	if th.Name != "catppuccin-mocha" && th.Name != "catppuccin-latte" {
		t.Errorf("Get(auto).Name = %q, want mocha or latte", th.Name)
	}
}

func TestIsDark(t *testing.T) {
	tests := []struct {
		name string
		base string
		dark bool
	}{
		{"mocha dark", "#1e1e2e", true},
		{"latte light", "#eff1f5", false},
		{"pure black", "#000000", true},
		{"pure white", "#ffffff", false},
		{"mid gray", "#808080", true}, // 128 luminance → dark (< 128)
		{"mid gray bright", "#818181", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := Theme{Base: lipgloss.Color(tt.base)}
			if got := th.IsDark(); got != tt.dark {
				t.Errorf("IsDark() with base=%s: got %v, want %v", tt.base, got, tt.dark)
			}
		})
	}
}

func TestApplyOverrides_Empty(t *testing.T) {
	th := CatppuccinMocha()
	original := th.Blue
	th.ApplyOverrides(ColorOverrides{}) // no overrides
	if th.Blue != original {
		t.Errorf("ApplyOverrides with empty should not change Blue: got %v, want %v", th.Blue, original)
	}
}

func TestApplyOverrides_SingleColor(t *testing.T) {
	th := CatppuccinMocha()
	th.ApplyOverrides(ColorOverrides{Blue: "#ff0000"})
	if th.Blue != lipgloss.Color("#ff0000") {
		t.Errorf("ApplyOverrides: Blue = %v, want #ff0000", th.Blue)
	}
	// Other colors should be unchanged
	expected := CatppuccinMocha()
	if th.Red != expected.Red {
		t.Errorf("ApplyOverrides: Red changed unexpectedly: got %v, want %v", th.Red, expected.Red)
	}
}

func TestApplyOverrides_MultipleColors(t *testing.T) {
	th := CatppuccinMocha()
	th.ApplyOverrides(ColorOverrides{
		Base: "#000000",
		Text: "#ffffff",
		Red:  "#ff0000",
	})
	if th.Base != lipgloss.Color("#000000") {
		t.Errorf("Base = %v, want #000000", th.Base)
	}
	if th.Text != lipgloss.Color("#ffffff") {
		t.Errorf("Text = %v, want #ffffff", th.Text)
	}
	if th.Red != lipgloss.Color("#ff0000") {
		t.Errorf("Red = %v, want #ff0000", th.Red)
	}
}

func TestSemanticAccessors(t *testing.T) {
	th := CatppuccinMocha()
	if th.DiffAdded() != th.Green {
		t.Error("DiffAdded should return Green")
	}
	if th.DiffRemoved() != th.Red {
		t.Error("DiffRemoved should return Red")
	}
	if th.DiffContext() != th.Subtext0 {
		t.Error("DiffContext should return Subtext0")
	}
	if th.StatusModified() != th.Yellow {
		t.Error("StatusModified should return Yellow")
	}
	if th.BranchCurrent() != th.Green {
		t.Error("BranchCurrent should return Green")
	}
	if th.BranchRemote() != th.Mauve {
		t.Error("BranchRemote should return Mauve")
	}
}

func TestHexDigit(t *testing.T) {
	tests := []struct {
		input byte
		want  uint8
	}{
		{'0', 0}, {'9', 9},
		{'a', 10}, {'f', 15},
		{'A', 10}, {'F', 15},
		{'g', 0}, {'z', 0}, // invalid
	}
	for _, tt := range tests {
		got := hexDigit(tt.input)
		if got != tt.want {
			t.Errorf("hexDigit(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAllThemesHaveRequiredColors(t *testing.T) {
	for name, fn := range Themes {
		t.Run(name, func(t *testing.T) {
			th := fn()
			if th.Name == "" {
				t.Error("Theme.Name is empty")
			}
			if th.Base == "" {
				t.Error("Theme.Base is empty")
			}
			if th.Text == "" {
				t.Error("Theme.Text is empty")
			}
			if th.Red == "" {
				t.Error("Theme.Red is empty")
			}
			if th.Green == "" {
				t.Error("Theme.Green is empty")
			}
			if th.Blue == "" {
				t.Error("Theme.Blue is empty")
			}
			if th.DiffAddedLineBg == "" {
				t.Error("Theme.DiffAddedLineBg is empty")
			}
			if th.DiffRemovedLineBg == "" {
				t.Error("Theme.DiffRemovedLineBg is empty")
			}
		})
	}
}
