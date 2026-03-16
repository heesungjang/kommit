package pages

import (
	"testing"
)

func TestParseDiffHunkNums(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOld int
		wantNew int
	}{
		{
			name:    "standard hunk header",
			line:    "@@ -1,5 +1,7 @@",
			wantOld: 1,
			wantNew: 1,
		},
		{
			name:    "new file starting at zero",
			line:    "@@ -0,0 +1,3 @@",
			wantOld: 1, // 0 is normalized to 1
			wantNew: 1,
		},
		{
			name:    "no comma single line",
			line:    "@@ -1 +1 @@",
			wantOld: 1,
			wantNew: 1,
		},
		{
			name:    "different start lines",
			line:    "@@ -10,6 +20,8 @@",
			wantOld: 10,
			wantNew: 20,
		},
		{
			name:    "with function context",
			line:    "@@ -100,10 +200,15 @@ func main() {",
			wantOld: 100,
			wantNew: 200,
		},
		{
			name:    "large line numbers",
			line:    "@@ -999,50 +1234,60 @@",
			wantOld: 999,
			wantNew: 1234,
		},
		{
			name:    "malformed no at signs",
			line:    "not a hunk header",
			wantOld: 1,
			wantNew: 1,
		},
		{
			name:    "malformed single at sign pair",
			line:    "@@ -1,5 +1,7",
			wantOld: 1,
			wantNew: 1,
		},
		{
			name:    "empty string",
			line:    "",
			wantOld: 1,
			wantNew: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOld, gotNew := parseDiffHunkNums(tt.line)
			if gotOld != tt.wantOld || gotNew != tt.wantNew {
				t.Errorf("parseDiffHunkNums(%q) = (%d, %d), want (%d, %d)",
					tt.line, gotOld, gotNew, tt.wantOld, tt.wantNew)
			}
		})
	}
}

func TestStripHunkContext(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "with function context",
			line: "@@ -10,6 +20,8 @@ func main() {",
			want: "@@ -10,6 +20,8 @@",
		},
		{
			name: "without function context",
			line: "@@ -1,5 +1,7 @@",
			want: "@@ -1,5 +1,7 @@",
		},
		{
			name: "trailing whitespace after closing",
			line: "@@ -1,5 +1,7 @@ ",
			want: "@@ -1,5 +1,7 @@",
		},
		{
			name: "no at signs",
			line: "not a hunk header",
			want: "not a hunk header",
		},
		{
			name: "single at sign pair only",
			line: "@@ no closing",
			want: "@@ no closing",
		},
		{
			name: "empty string",
			line: "",
			want: "",
		},
		{
			name: "complex context",
			line: "@@ -50,10 +55,12 @@ type Foo struct {",
			want: "@@ -50,10 +55,12 @@",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHunkContext(tt.line)
			if got != tt.want {
				t.Errorf("stripHunkContext(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestExpandTabs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tabWidth int
		want     string
	}{
		{
			name:     "single tab at start",
			input:    "\thello",
			tabWidth: 4,
			want:     "    hello",
		},
		{
			name:     "tab after one char",
			input:    "a\tb",
			tabWidth: 4,
			want:     "a   b",
		},
		{
			name:     "tab after three chars",
			input:    "abc\td",
			tabWidth: 4,
			want:     "abc d",
		},
		{
			name:     "tab after four chars aligns to next stop",
			input:    "abcd\te",
			tabWidth: 4,
			want:     "abcd    e",
		},
		{
			name:     "multiple tabs",
			input:    "\t\thello",
			tabWidth: 4,
			want:     "        hello",
		},
		{
			name:     "no tabs",
			input:    "hello world",
			tabWidth: 4,
			want:     "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			tabWidth: 4,
			want:     "",
		},
		{
			name:     "only tab",
			input:    "\t",
			tabWidth: 4,
			want:     "    ",
		},
		{
			name:     "mixed tabs and spaces",
			input:    "  \thello",
			tabWidth: 4,
			want:     "    hello",
		},
		{
			name:     "tab width param is ignored uses hardcoded 4",
			input:    "\tx",
			tabWidth: 8,
			want:     "    x", // always uses internal tabWidth=4
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandTabs(tt.input, tt.tabWidth)
			if got != tt.want {
				t.Errorf("expandTabs(%q, %d) = %q, want %q",
					tt.input, tt.tabWidth, got, tt.want)
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "shorter than width",
			input: "hello",
			width: 10,
			want:  "hello",
		},
		{
			name:  "exact width",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "longer than width",
			input: "hello world",
			width: 5,
			want:  "hello",
		},
		{
			name:  "width of one",
			input: "hello",
			width: 1,
			want:  "h",
		},
		{
			name:  "width of zero",
			input: "hello",
			width: 0,
			want:  "",
		},
		{
			name:  "negative width",
			input: "hello",
			width: -1,
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			width: 10,
			want:  "",
		},
		{
			name:  "empty string zero width",
			input: "",
			width: 0,
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToWidth(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("truncateToWidth(%q, %d) = %q, want %q",
					tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestHorizontalSlice(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		offset int
		width  int
		want   string
	}{
		{
			name:   "no offset",
			input:  "hello world",
			offset: 0,
			width:  5,
			want:   "hello",
		},
		{
			name:   "offset into middle",
			input:  "hello world",
			offset: 6,
			width:  5,
			want:   "world",
		},
		{
			name:   "offset past end",
			input:  "hello",
			offset: 10,
			width:  5,
			want:   "",
		},
		{
			name:   "offset exactly at end",
			input:  "hello",
			offset: 5,
			width:  5,
			want:   "",
		},
		{
			name:   "negative offset acts as zero",
			input:  "hello world",
			offset: -1,
			width:  5,
			want:   "hello",
		},
		{
			name:   "width larger than remaining",
			input:  "hello world",
			offset: 6,
			width:  100,
			want:   "world",
		},
		{
			name:   "empty string",
			input:  "",
			offset: 0,
			width:  5,
			want:   "",
		},
		{
			name:   "zero width",
			input:  "hello",
			offset: 0,
			width:  0,
			want:   "",
		},
		{
			name:   "offset one width one",
			input:  "abcdef",
			offset: 2,
			width:  1,
			want:   "c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := horizontalSlice(tt.input, tt.offset, tt.width)
			if got != tt.want {
				t.Errorf("horizontalSlice(%q, %d, %d) = %q, want %q",
					tt.input, tt.offset, tt.width, got, tt.want)
			}
		})
	}
}

func TestCountVisibleLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		n     int
		want  int
	}{
		{
			name:  "all visible",
			lines: []string{"@@ -1,3 +1,3 @@", "+added", "-removed", " context"},
			n:     4,
			want:  4,
		},
		{
			name: "skip diff git header",
			lines: []string{
				"diff --git a/file.go b/file.go",
				"index abc123..def456 100644",
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,3 +1,3 @@",
				" context",
			},
			n:    6,
			want: 2, // only @@ line and context are visible
		},
		{
			name: "n less than total lines",
			lines: []string{
				"diff --git a/file.go b/file.go",
				"index abc123..def456 100644",
				"--- a/file.go",
				"+++ b/file.go",
				"@@ -1,3 +1,3 @@",
				" context",
			},
			n:    4,
			want: 0, // first 4 lines are all invisible headers
		},
		{
			name:  "empty lines slice",
			lines: []string{},
			n:     0,
			want:  0,
		},
		{
			name:  "n is zero",
			lines: []string{"@@ -1 +1 @@", " hello"},
			n:     0,
			want:  0,
		},
		{
			name:  "n exceeds length",
			lines: []string{" context1", " context2"},
			n:     10,
			want:  2,
		},
		{
			name: "mixed visible and invisible",
			lines: []string{
				"diff --git a/a.go b/a.go",
				"--- a/a.go",
				"+++ b/a.go",
				"@@ -1,2 +1,2 @@",
				"-old line",
				"+new line",
				" context",
			},
			n:    7,
			want: 4, // @@, -old, +new, context
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countVisibleLines(tt.lines, tt.n)
			if got != tt.want {
				t.Errorf("countVisibleLines(lines, %d) = %d, want %d",
					tt.n, got, tt.want)
			}
		})
	}
}

func TestIsInvisibleHeader(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "diff git line",
			line: "diff --git a/file.go b/file.go",
			want: true,
		},
		{
			name: "index line",
			line: "index abc123..def456 100644",
			want: true,
		},
		{
			name: "old file line",
			line: "--- a/file.go",
			want: true,
		},
		{
			name: "new file line",
			line: "+++ b/file.go",
			want: true,
		},
		{
			name: "hunk header is visible",
			line: "@@ -1,5 +1,7 @@",
			want: false,
		},
		{
			name: "added line is visible",
			line: "+new line",
			want: false,
		},
		{
			name: "removed line is visible",
			line: "-old line",
			want: false,
		},
		{
			name: "context line is visible",
			line: " unchanged line",
			want: false,
		},
		{
			name: "empty line is visible",
			line: "",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInvisibleHeader(tt.line)
			if got != tt.want {
				t.Errorf("isInvisibleHeader(%q) = %v, want %v",
					tt.line, got, tt.want)
			}
		})
	}
}
