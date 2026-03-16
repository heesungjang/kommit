package components

import "testing"

func TestHexToBgSeq(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{
			name: "black with hash prefix",
			hex:  "#000000",
			want: "\x1b[48;2;0;0;0m",
		},
		{
			name: "white with hash prefix",
			hex:  "#ffffff",
			want: "\x1b[48;2;255;255;255m",
		},
		{
			name: "red without hash",
			hex:  "ff0000",
			want: "\x1b[48;2;255;0;0m",
		},
		{
			name: "catppuccin surface0",
			hex:  "#313244",
			want: "\x1b[48;2;49;50;68m",
		},
		{
			name: "mixed case hex",
			hex:  "#aaBBcc",
			want: "\x1b[48;2;170;187;204m",
		},
		{
			name: "empty string",
			hex:  "",
			want: "",
		},
		{
			name: "too short",
			hex:  "#fff",
			want: "",
		},
		{
			name: "too long",
			hex:  "#fffffff",
			want: "",
		},
		{
			name: "just hash",
			hex:  "#",
			want: "",
		},
		{
			name: "no hash too short",
			hex:  "abc",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hexToBgSeq(tt.hex)
			if got != tt.want {
				t.Errorf("hexToBgSeq(%q) = %q, want %q", tt.hex, got, tt.want)
			}
		})
	}
}

func TestNewStatusBar(t *testing.T) {
	sb := NewStatusBar()

	if sb.Branch() != "main" {
		t.Errorf("Branch() = %q, want %q", sb.Branch(), "main")
	}
	if !sb.clean {
		t.Error("clean should default to true")
	}
	if sb.width != 80 {
		t.Errorf("width = %d, want 80", sb.width)
	}
	if sb.ahead != 0 || sb.behind != 0 {
		t.Errorf("ahead/behind = %d/%d, want 0/0", sb.ahead, sb.behind)
	}
	if sb.IsBisecting() {
		t.Error("IsBisecting() should default to false")
	}
	if sb.IsComparing() {
		t.Error("IsComparing() should default to false")
	}
}

func TestStatusBar_SetBranch(t *testing.T) {
	tests := []struct {
		name   string
		branch string
	}{
		{"feature branch", "feature/login"},
		{"empty string", ""},
		{"with slashes", "release/v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStatusBar().SetBranch(tt.branch)
			if sb.Branch() != tt.branch {
				t.Errorf("Branch() = %q, want %q", sb.Branch(), tt.branch)
			}
		})
	}
}

func TestStatusBar_SetAheadBehind(t *testing.T) {
	tests := []struct {
		name   string
		ahead  int
		behind int
	}{
		{"both zero", 0, 0},
		{"ahead only", 5, 0},
		{"behind only", 0, 3},
		{"both nonzero", 2, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStatusBar().SetAheadBehind(tt.ahead, tt.behind)
			if sb.ahead != tt.ahead {
				t.Errorf("ahead = %d, want %d", sb.ahead, tt.ahead)
			}
			if sb.behind != tt.behind {
				t.Errorf("behind = %d, want %d", sb.behind, tt.behind)
			}
		})
	}
}

func TestStatusBar_SetClean(t *testing.T) {
	tests := []struct {
		name  string
		clean bool
	}{
		{"dirty", false},
		{"clean", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStatusBar().SetClean(tt.clean)
			if sb.clean != tt.clean {
				t.Errorf("clean = %v, want %v", sb.clean, tt.clean)
			}
		})
	}
}

func TestStatusBar_SetRepoDir(t *testing.T) {
	sb := NewStatusBar().SetRepoDir("/home/user/project")
	if sb.repoDir != "/home/user/project" {
		t.Errorf("repoDir = %q, want %q", sb.repoDir, "/home/user/project")
	}
}

func TestStatusBar_SetBisecting(t *testing.T) {
	sb := NewStatusBar().SetBisecting(true)
	if !sb.IsBisecting() {
		t.Error("IsBisecting() should be true after SetBisecting(true)")
	}

	sb = sb.SetBisecting(false)
	if sb.IsBisecting() {
		t.Error("IsBisecting() should be false after SetBisecting(false)")
	}
}

func TestStatusBar_SetRebasing(t *testing.T) {
	sb := NewStatusBar().SetRebasing(true)
	if !sb.rebasing {
		t.Error("rebasing should be true")
	}

	sb = sb.SetRebasing(false)
	if sb.rebasing {
		t.Error("rebasing should be false")
	}
}

func TestStatusBar_SetComparing(t *testing.T) {
	tests := []struct {
		name       string
		hash       string
		wantActive bool
	}{
		{"active with hash", "abc123", true},
		{"cleared", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewStatusBar().SetComparing(tt.hash)
			if sb.IsComparing() != tt.wantActive {
				t.Errorf("IsComparing() = %v, want %v", sb.IsComparing(), tt.wantActive)
			}
			if sb.comparing != tt.hash {
				t.Errorf("comparing = %q, want %q", sb.comparing, tt.hash)
			}
		})
	}
}

func TestStatusBar_SetFocusLabel(t *testing.T) {
	sb := NewStatusBar().SetFocusLabel("Commits")
	if sb.focusLabel != "Commits" {
		t.Errorf("focusLabel = %q, want %q", sb.focusLabel, "Commits")
	}
}

func TestStatusBar_SetSize(t *testing.T) {
	sb := NewStatusBar().SetSize(120)
	if sb.width != 120 {
		t.Errorf("width = %d, want 120", sb.width)
	}
}

func TestStatusBar_BuilderChaining(t *testing.T) {
	sb := NewStatusBar().
		SetBranch("develop").
		SetAheadBehind(3, 1).
		SetClean(false).
		SetRepoDir("/tmp/repo").
		SetBisecting(true).
		SetRebasing(true).
		SetComparing("deadbeef").
		SetFocusLabel("Sidebar").
		SetSize(200)

	if sb.Branch() != "develop" {
		t.Errorf("Branch() = %q, want %q", sb.Branch(), "develop")
	}
	if sb.ahead != 3 || sb.behind != 1 {
		t.Errorf("ahead/behind = %d/%d, want 3/1", sb.ahead, sb.behind)
	}
	if sb.clean {
		t.Error("clean should be false")
	}
	if sb.repoDir != "/tmp/repo" {
		t.Errorf("repoDir = %q, want %q", sb.repoDir, "/tmp/repo")
	}
	if !sb.IsBisecting() {
		t.Error("IsBisecting() should be true")
	}
	if !sb.rebasing {
		t.Error("rebasing should be true")
	}
	if !sb.IsComparing() {
		t.Error("IsComparing() should be true")
	}
	if sb.comparing != "deadbeef" {
		t.Errorf("comparing = %q, want %q", sb.comparing, "deadbeef")
	}
	if sb.focusLabel != "Sidebar" {
		t.Errorf("focusLabel = %q, want %q", sb.focusLabel, "Sidebar")
	}
	if sb.width != 200 {
		t.Errorf("width = %d, want 200", sb.width)
	}
}

func TestStatusBar_ImmutableBuilders(t *testing.T) {
	original := NewStatusBar()
	modified := original.SetBranch("feature").SetClean(false).SetBisecting(true)

	if original.Branch() != "main" {
		t.Errorf("original branch mutated: got %q, want %q", original.Branch(), "main")
	}
	if !original.clean {
		t.Error("original clean mutated: got false, want true")
	}
	if original.IsBisecting() {
		t.Error("original bisecting mutated: got true, want false")
	}

	if modified.Branch() != "feature" {
		t.Errorf("modified Branch() = %q, want %q", modified.Branch(), "feature")
	}
	if modified.clean {
		t.Error("modified clean should be false")
	}
	if !modified.IsBisecting() {
		t.Error("modified IsBisecting() should be true")
	}
}
