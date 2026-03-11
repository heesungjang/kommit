package pages

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"hello world", 3, "hel"},
		{"hello world", 2, "he"},
		{"hello world", 1, "h"},
		{"", 5, ""},
		{"a", 1, "a"},
		{"ab", 1, "a"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestHexToRGB(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b uint8
	}{
		{"000000", 0, 0, 0},
		{"ffffff", 255, 255, 255},
		{"ff0000", 255, 0, 0},
		{"00ff00", 0, 255, 0},
		{"0000ff", 0, 0, 255},
		{"1e1e2e", 0x1e, 0x1e, 0x2e},
		{"89b4fa", 0x89, 0xb4, 0xfa},
	}
	for _, tt := range tests {
		r, g, b := hexToRGB(tt.hex)
		if r != tt.r || g != tt.g || b != tt.b {
			t.Errorf("hexToRGB(%q) = (%d, %d, %d), want (%d, %d, %d)", tt.hex, r, g, b, tt.r, tt.g, tt.b)
		}
	}
}

func TestHexToRGB_Invalid(t *testing.T) {
	r, g, b := hexToRGB("short")
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("hexToRGB(short) = (%d, %d, %d), want (0, 0, 0)", r, g, b)
	}
}

func TestSplitCommitMessage(t *testing.T) {
	tests := []struct {
		input   string
		summary string
		desc    string
	}{
		{"Fix bug", "Fix bug", ""},
		{"Fix bug\n\nDetailed description", "Fix bug", "Detailed description"},
		{"Fix bug\n\nLine 1\nLine 2", "Fix bug", "Line 1\nLine 2"},
		{"", "", ""},
		{"Single line\n", "Single line", ""},
		{"Title\n\n", "Title", ""},
	}
	for _, tt := range tests {
		summary, desc := splitCommitMessage(tt.input)
		if summary != tt.summary || desc != tt.desc {
			t.Errorf("splitCommitMessage(%q) = (%q, %q), want (%q, %q)",
				tt.input, summary, desc, tt.summary, tt.desc)
		}
	}
}

func TestPanelLayout_Defaults(t *testing.T) {
	// Create a minimal LogPage with no config to test defaults.
	l := LogPage{width: 200, height: 50}
	pw := l.panelLayout()

	// Sidebar should be ~15% of 200 = 30, clamped to max 40.
	if pw.sidebar < 18 || pw.sidebar > 40 {
		t.Errorf("sidebar = %d, want [18, 40]", pw.sidebar)
	}
	// Center + right + borders should roughly fill the remaining width.
	if pw.center <= 0 {
		t.Error("center width should be positive")
	}
	if pw.right <= 0 {
		t.Error("right width should be positive")
	}
}

func TestPanelLayout_NarrowTerminal(t *testing.T) {
	l := LogPage{width: 80, height: 24}
	pw := l.panelLayout()

	if pw.sidebar < 18 {
		t.Errorf("sidebar = %d, want >= 18", pw.sidebar)
	}
	if pw.center < 10 {
		t.Errorf("center = %d, want >= 10", pw.center)
	}
}

func TestAnsiBgRe(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		{"\x1b[40m", true},             // basic black bg
		{"\x1b[47m", true},             // basic white bg
		{"\x1b[49m", true},             // default bg
		{"\x1b[100m", true},            // bright black bg
		{"\x1b[107m", true},            // bright white bg
		{"\x1b[48;5;42m", true},        // 256-color bg
		{"\x1b[48;2;255;0;128m", true}, // 24-bit bg
		{"\x1b[31m", false},            // foreground red (not bg)
		{"\x1b[0m", false},             // reset (not bg)
		{"hello", false},
	}
	for _, tt := range tests {
		got := ansiBgRe.MatchString(tt.input)
		if got != tt.match {
			t.Errorf("ansiBgRe.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
		}
	}
}
