package styles

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ClipPanel tests
// ---------------------------------------------------------------------------

func TestClipPanel_ClipsExcessLines(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	got := ClipPanel(content, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("ClipPanel with 5 lines, target 3: got %d lines, want 3", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("ClipPanel clipped wrong lines: got %v", lines)
	}
}

func TestClipPanel_FewerLinesThanTarget(t *testing.T) {
	content := "line1\nline2"
	got := ClipPanel(content, 5)
	if got != content {
		t.Errorf("ClipPanel with fewer lines: got %q, want %q", got, content)
	}
}

func TestClipPanel_ExactLineCount(t *testing.T) {
	content := "line1\nline2\nline3"
	got := ClipPanel(content, 3)
	if got != content {
		t.Errorf("ClipPanel with exact lines: got %q, want %q", got, content)
	}
}

func TestClipPanel_EmptyContent(t *testing.T) {
	got := ClipPanel("", 5)
	// An empty string split on "\n" yields [""], which is 1 element <= 5, so unchanged.
	if got != "" {
		t.Errorf("ClipPanel empty: got %q, want %q", got, "")
	}
}

func TestClipPanel_SingleLine(t *testing.T) {
	got := ClipPanel("hello", 1)
	if got != "hello" {
		t.Errorf("ClipPanel single line: got %q, want %q", got, "hello")
	}
}

func TestClipPanel_TargetOne(t *testing.T) {
	content := "a\nb\nc"
	got := ClipPanel(content, 1)
	if got != "a" {
		t.Errorf("ClipPanel target=1: got %q, want %q", got, "a")
	}
}

// ---------------------------------------------------------------------------
// ParseRef tests
// ---------------------------------------------------------------------------

func TestParseRef(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantType    RefType
		wantDisplay string
	}{
		{
			name:        "HEAD keyword",
			input:       "HEAD",
			wantType:    RefHead,
			wantDisplay: "HEAD",
		},
		{
			name:        "HEAD arrow branch",
			input:       "HEAD -> main",
			wantType:    RefLocalBranch,
			wantDisplay: "main",
		},
		{
			name:        "HEAD arrow feature branch",
			input:       "HEAD -> feature/xyz",
			wantType:    RefLocalBranch,
			wantDisplay: "feature/xyz",
		},
		{
			name:        "remote branch",
			input:       "origin/main",
			wantType:    RefRemoteBranch,
			wantDisplay: "origin/main",
		},
		{
			name:        "tag",
			input:       "tag: v1.0",
			wantType:    RefTag,
			wantDisplay: "v1.0",
		},
		{
			name:        "tag with spaces",
			input:       "tag: release-2.0",
			wantType:    RefTag,
			wantDisplay: "release-2.0",
		},
		{
			name:        "local branch",
			input:       "main",
			wantType:    RefLocalBranch,
			wantDisplay: "main",
		},
		{
			name:        "local branch develop",
			input:       "develop",
			wantType:    RefLocalBranch,
			wantDisplay: "develop",
		},
		{
			name:        "trimmed whitespace",
			input:       "  HEAD  ",
			wantType:    RefHead,
			wantDisplay: "HEAD",
		},
		{
			name:        "refs/heads path treated as remote (has slash)",
			input:       "refs/heads/feature",
			wantType:    RefRemoteBranch,
			wantDisplay: "refs/heads/feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRef(tt.input)
			if got.Type != tt.wantType {
				t.Errorf("ParseRef(%q).Type = %d, want %d", tt.input, got.Type, tt.wantType)
			}
			if got.Display != tt.wantDisplay {
				t.Errorf("ParseRef(%q).Display = %q, want %q", tt.input, got.Display, tt.wantDisplay)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FileListIcon tests
// ---------------------------------------------------------------------------

func TestFileListIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"added", "A"},
		{"deleted", "D"},
		{"modified", "M"},
		{"renamed", "R"},
		{"unknown", "?"},
		{"", "?"},
		{"copied", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := FileListIcon(tt.status)
			if got != tt.want {
				t.Errorf("FileListIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
