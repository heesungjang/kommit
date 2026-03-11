package customcmd

import (
	"testing"

	"github.com/heesungjang/kommit/internal/config"
)

func TestExpand_NoTemplate(t *testing.T) {
	result, err := Expand("git log --oneline", TemplateVars{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "git log --oneline" {
		t.Errorf("got %q, want %q", result, "git log --oneline")
	}
}

func TestExpand_WithVars(t *testing.T) {
	vars := TemplateVars{
		Hash:      "abc123def456",
		ShortHash: "abc123d",
		Branch:    "main",
		Path:      "src/foo.go",
	}
	result, err := Expand("git show {{.ShortHash}} -- {{.Path}}", vars)
	if err != nil {
		t.Fatal(err)
	}
	expected := "git show abc123d -- src/foo.go"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestExpand_InvalidTemplate(t *testing.T) {
	_, err := Expand("{{.Invalid", TemplateVars{})
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestExpand_EmptyCommand(t *testing.T) {
	result, err := Expand("", TemplateVars{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestFilterByContext(t *testing.T) {
	cmds := []config.CustomCommand{
		{Name: "A", Context: "global"},
		{Name: "B", Context: "commit"},
		{Name: "C", Context: "file"},
		{Name: "D", Context: ""}, // defaults to global
		{Name: "E", Context: "branch"},
	}

	tests := []struct {
		ctx  string
		want []string
	}{
		{"global", []string{"A", "D"}},
		{"commit", []string{"A", "B", "D"}},
		{"file", []string{"A", "C", "D"}},
		{"branch", []string{"A", "D", "E"}},
	}

	for _, tt := range tests {
		filtered := FilterByContext(cmds, tt.ctx)
		if len(filtered) != len(tt.want) {
			names := make([]string, len(filtered))
			for i, c := range filtered {
				names[i] = c.Name
			}
			t.Errorf("FilterByContext(%q): got %v, want %v", tt.ctx, names, tt.want)
			continue
		}
		for i, c := range filtered {
			if c.Name != tt.want[i] {
				t.Errorf("FilterByContext(%q)[%d] = %q, want %q", tt.ctx, i, c.Name, tt.want[i])
			}
		}
	}
}

func TestRun_SimpleCommand(t *testing.T) {
	out, err := Run("echo hello", "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Errorf("got %q, want %q", out, "hello")
	}
}

func TestRun_FailingCommand(t *testing.T) {
	_, err := Run("false", "/tmp")
	if err == nil {
		t.Error("expected error for failing command")
	}
}

func TestRunInteractive(t *testing.T) {
	cmd := RunInteractive("echo interactive", "/tmp")
	if cmd == nil {
		t.Fatal("RunInteractive returned nil")
	}
	if cmd.Dir != "/tmp" {
		t.Errorf("Dir = %q, want /tmp", cmd.Dir)
	}
}
