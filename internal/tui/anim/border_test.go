package anim

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// parseHex tests
// ---------------------------------------------------------------------------

func TestParseHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantR float64
		wantG float64
		wantB float64
	}{
		{
			name:  "valid black",
			input: "#000000",
			wantR: 0, wantG: 0, wantB: 0,
		},
		{
			name:  "valid white",
			input: "#ffffff",
			wantR: 255, wantG: 255, wantB: 255,
		},
		{
			name:  "valid red",
			input: "#ff0000",
			wantR: 255, wantG: 0, wantB: 0,
		},
		{
			name:  "valid mixed case",
			input: "#1A2B3C",
			wantR: 26, wantG: 43, wantB: 60,
		},
		{
			name:  "missing hash prefix",
			input: "ff0000",
			wantR: 0, wantG: 0, wantB: 0,
		},
		{
			name:  "short hex 3-digit",
			input: "#fff",
			wantR: 0, wantG: 0, wantB: 0,
		},
		{
			name:  "empty string",
			input: "",
			wantR: 0, wantG: 0, wantB: 0,
		},
		{
			name:  "hash only",
			input: "#",
			wantR: 0, wantG: 0, wantB: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := parseHex(tt.input)
			if r != tt.wantR {
				t.Errorf("parseHex(%q) r = %v, want %v", tt.input, r, tt.wantR)
			}
			if g != tt.wantG {
				t.Errorf("parseHex(%q) g = %v, want %v", tt.input, g, tt.wantG)
			}
			if b != tt.wantB {
				t.Errorf("parseHex(%q) b = %v, want %v", tt.input, b, tt.wantB)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// clamp8 tests
// ---------------------------------------------------------------------------

func TestClamp8(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  uint8
	}{
		{"negative", -10, 0},
		{"zero", 0, 0},
		{"mid", 127, 127},
		{"max", 255, 255},
		{"above max", 300, 255},
		{"large negative", -1000, 0},
		{"fractional low", 0.9, 0},
		{"fractional mid", 128.7, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp8(tt.input)
			if got != tt.want {
				t.Errorf("clamp8(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lerpColor tests
// ---------------------------------------------------------------------------

func TestLerpColor(t *testing.T) {
	black := lipgloss.Color("#000000")
	white := lipgloss.Color("#ffffff")
	red := lipgloss.Color("#ff0000")
	blue := lipgloss.Color("#0000ff")

	tests := []struct {
		name string
		a    lipgloss.Color
		b    lipgloss.Color
		t    float64
		want lipgloss.Color
	}{
		{
			name: "t=0 returns a",
			a:    black, b: white, t: 0,
			want: lipgloss.Color("#000000"),
		},
		{
			name: "t=1 returns b",
			a:    black, b: white, t: 1,
			want: lipgloss.Color("#ffffff"),
		},
		{
			name: "t=0.5 midpoint black to white",
			a:    black, b: white, t: 0.5,
			want: lipgloss.Color("#7f7f7f"),
		},
		{
			name: "t=0.5 red to blue",
			a:    red, b: blue, t: 0.5,
			want: lipgloss.Color("#7f007f"),
		},
		{
			name: "same color returns same",
			a:    red, b: red, t: 0.5,
			want: lipgloss.Color("#ff0000"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lerpColor(tt.a, tt.b, tt.t)
			if got != tt.want {
				t.Errorf("lerpColor(%q, %q, %v) = %q, want %q", tt.a, tt.b, tt.t, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SetFocus tests
// ---------------------------------------------------------------------------

func TestSetFocus(t *testing.T) {
	t.Run("focus activates animation", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		if !b.Active() {
			t.Error("expected Active() == true after SetFocus(true)")
		}
		if b.target[BorderSidebar] != 1.0 {
			t.Errorf("target = %v, want 1.0", b.target[BorderSidebar])
		}
	})

	t.Run("unfocus activates animation when previously focused", func(t *testing.T) {
		var b BorderAnim
		// First reach focused state.
		b.progress[BorderCenter] = 1.0
		b.target[BorderCenter] = 1.0

		b.SetFocus(BorderCenter, false)
		if !b.Active() {
			t.Error("expected Active() == true after SetFocus(false) from focused state")
		}
		if b.target[BorderCenter] != 0.0 {
			t.Errorf("target = %v, want 0.0", b.target[BorderCenter])
		}
	})

	t.Run("setting same target is no-op", func(t *testing.T) {
		var b BorderAnim
		// target already 0, setting unfocused should not activate.
		b.SetFocus(BorderSidebar, false)
		if b.Active() {
			t.Error("expected Active() == false when target unchanged")
		}
	})

	t.Run("independent borders", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		b.SetFocus(BorderRight, true)
		if b.target[BorderSidebar] != 1.0 {
			t.Errorf("sidebar target = %v, want 1.0", b.target[BorderSidebar])
		}
		if b.target[BorderRight] != 1.0 {
			t.Errorf("right target = %v, want 1.0", b.target[BorderRight])
		}
		if b.target[BorderCenter] != 0.0 {
			t.Errorf("center target = %v, want 0.0 (untouched)", b.target[BorderCenter])
		}
	})
}

// ---------------------------------------------------------------------------
// Tick tests
// ---------------------------------------------------------------------------

func TestTick(t *testing.T) {
	t.Run("single step advances progress", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		b.Tick()
		if b.progress[BorderSidebar] != animStep {
			t.Errorf("progress after 1 tick = %v, want %v", b.progress[BorderSidebar], animStep)
		}
	})

	t.Run("full animation reaches target", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		// animStep is 0.125, so 8 ticks should reach 1.0.
		for i := 0; i < 20; i++ {
			if !b.Active() {
				break
			}
			b.Tick()
		}
		if b.progress[BorderSidebar] != 1.0 {
			t.Errorf("progress after full animation = %v, want 1.0", b.progress[BorderSidebar])
		}
		if b.Active() {
			t.Error("expected Active() == false after animation completes")
		}
	})

	t.Run("reverse animation reaches zero", func(t *testing.T) {
		var b BorderAnim
		b.progress[BorderSidebar] = 1.0
		b.target[BorderSidebar] = 1.0

		b.SetFocus(BorderSidebar, false)
		for i := 0; i < 20; i++ {
			if !b.Active() {
				break
			}
			b.Tick()
		}
		if b.progress[BorderSidebar] != 0.0 {
			t.Errorf("progress after reverse animation = %v, want 0.0", b.progress[BorderSidebar])
		}
	})

	t.Run("tick returns false when idle", func(t *testing.T) {
		var b BorderAnim
		still := b.Tick()
		if still {
			t.Error("Tick() returned true with no active animation")
		}
	})

	t.Run("tick returns true while animating", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		still := b.Tick()
		// After one step from 0 → 1, progress is 0.125 which is < 1.0.
		if !still {
			t.Error("Tick() returned false while animation is in progress")
		}
	})
}

// ---------------------------------------------------------------------------
// Color tests
// ---------------------------------------------------------------------------

func TestColor(t *testing.T) {
	unfocused := lipgloss.Color("#333333")
	focused := lipgloss.Color("#ffffff")

	t.Run("returns unfocused when progress is 0", func(t *testing.T) {
		var b BorderAnim
		got := b.Color(BorderSidebar, unfocused, focused)
		if got != unfocused {
			t.Errorf("Color() = %q, want unfocused %q", got, unfocused)
		}
	})

	t.Run("returns focused when progress is 1", func(t *testing.T) {
		var b BorderAnim
		b.progress[BorderSidebar] = 1.0
		got := b.Color(BorderSidebar, unfocused, focused)
		if got != focused {
			t.Errorf("Color() = %q, want focused %q", got, focused)
		}
	})

	t.Run("returns interpolated color mid-animation", func(t *testing.T) {
		var b BorderAnim
		b.progress[BorderSidebar] = 0.5
		got := b.Color(BorderSidebar, unfocused, focused)
		// Should be neither unfocused nor focused.
		if got == unfocused || got == focused {
			t.Errorf("Color() at 0.5 should be interpolated, got %q", got)
		}
		// Midpoint between #333333 and #ffffff: (0x33+0xff)/2 = 0x99.
		want := lipgloss.Color("#999999")
		if got != want {
			t.Errorf("Color() at 0.5 = %q, want %q", got, want)
		}
	})

	t.Run("different borders are independent", func(t *testing.T) {
		var b BorderAnim
		b.progress[BorderSidebar] = 1.0
		b.progress[BorderCenter] = 0.0

		gotSidebar := b.Color(BorderSidebar, unfocused, focused)
		gotCenter := b.Color(BorderCenter, unfocused, focused)

		if gotSidebar != focused {
			t.Errorf("sidebar Color() = %q, want %q", gotSidebar, focused)
		}
		if gotCenter != unfocused {
			t.Errorf("center Color() = %q, want %q", gotCenter, unfocused)
		}
	})
}

// ---------------------------------------------------------------------------
// Active tests
// ---------------------------------------------------------------------------

func TestActive(t *testing.T) {
	t.Run("initially inactive", func(t *testing.T) {
		var b BorderAnim
		if b.Active() {
			t.Error("expected new BorderAnim to be inactive")
		}
	})

	t.Run("active after SetFocus", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		if !b.Active() {
			t.Error("expected Active() after SetFocus")
		}
	})

	t.Run("inactive after animation completes", func(t *testing.T) {
		var b BorderAnim
		b.SetFocus(BorderSidebar, true)
		for i := 0; i < 20; i++ {
			b.Tick()
		}
		if b.Active() {
			t.Error("expected inactive after animation completes")
		}
	})
}
