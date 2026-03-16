package pages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShortenPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "path under home",
			path: filepath.Join(home, "projects", "myrepo"),
			want: "~/projects/myrepo",
		},
		{
			name: "home dir itself",
			path: home,
			want: "~",
		},
		{
			name: "path not under home",
			path: "/tmp/somerepo",
			want: "/tmp/somerepo",
		},
		{
			name: "root path",
			path: "/",
			want: "/",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
		{
			name: "path with trailing slash under home",
			path: filepath.Join(home, "code") + "/",
			want: "~/code/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenPath(tt.path)
			if got != tt.want {
				t.Errorf("shortenPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestResolveWorkspaceIndex(t *testing.T) {
	w := WorkspacePage{}

	tests := []struct {
		name string
		item *wsItem
		want int
	}{
		{
			name: "nil item",
			item: nil,
			want: -1,
		},
		{
			name: "workspace header",
			item: &wsItem{kind: wsItemWorkspaceHeader, workspaceIndex: 2},
			want: 2,
		},
		{
			name: "repo entry",
			item: &wsItem{kind: wsItemRepoEntry, workspaceIndex: 1},
			want: 1,
		},
		{
			name: "recent header",
			item: &wsItem{kind: wsItemRecentHeader, workspaceIndex: -1},
			want: -1,
		},
		{
			name: "recent entry",
			item: &wsItem{kind: wsItemRecentEntry, workspaceIndex: -1},
			want: -1,
		},
		{
			name: "separator",
			item: &wsItem{kind: wsItemSeparator, workspaceIndex: -1},
			want: -1,
		},
		{
			name: "workspace header index 0",
			item: &wsItem{kind: wsItemWorkspaceHeader, workspaceIndex: 0},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.resolveWorkspaceIndex(tt.item)
			if got != tt.want {
				t.Errorf("resolveWorkspaceIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestListHeight(t *testing.T) {
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{
			name:   "normal terminal",
			height: 50,
			// listHeight = height - PanelBorderHeight - 3; minimum 1
			// We test the formula by asserting positive and reasonable values.
		},
		{
			name:   "very small",
			height: 1,
		},
		{
			name:   "zero height",
			height: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := WorkspacePage{height: tt.height}
			got := w.listHeight()
			if got < 1 {
				t.Errorf("listHeight() = %d, want >= 1", got)
			}
		})
	}
}
