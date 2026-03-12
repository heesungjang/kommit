package ai

import (
	"strings"
	"testing"
)

func TestParsePRDescription_TitleAndBody(t *testing.T) {
	input := "Add user authentication feature\n\nThis PR adds OAuth2-based authentication.\n\n- Login/logout endpoints\n- JWT token handling\n- Session management"
	desc := parsePRDescription(input)

	if desc.Title != "Add user authentication feature" {
		t.Errorf("Title = %q, want %q", desc.Title, "Add user authentication feature")
	}
	if desc.Body == "" {
		t.Error("Body should not be empty")
	}
	if !strings.Contains(desc.Body, "OAuth2") {
		t.Errorf("Body should contain 'OAuth2', got: %q", desc.Body)
	}
}

func TestParsePRDescription_TitleOnly(t *testing.T) {
	input := "Fix typo in readme"
	desc := parsePRDescription(input)

	if desc.Title != "Fix typo in readme" {
		t.Errorf("Title = %q, want %q", desc.Title, "Fix typo in readme")
	}
	if desc.Body != "" {
		t.Errorf("Body should be empty, got: %q", desc.Body)
	}
}

func TestParsePRDescription_StripsCodeFences(t *testing.T) {
	input := "```\nAdd new feature\n\nSome description\n```"
	desc := parsePRDescription(input)

	if desc.Title != "Add new feature" {
		t.Errorf("Title = %q, want %q", desc.Title, "Add new feature")
	}
}

func TestParsePRDescription_StripsHashPrefix(t *testing.T) {
	input := "# Add new feature\n\nDescription here"
	desc := parsePRDescription(input)

	if desc.Title != "Add new feature" {
		t.Errorf("Title = %q, want %q", desc.Title, "Add new feature")
	}
}

func TestParsePRDescription_Empty(t *testing.T) {
	desc := parsePRDescription("")
	if desc.Title != "Update" {
		t.Errorf("Title = %q, want %q", desc.Title, "Update")
	}
}

func TestParsePRDescription_TruncatesLongTitle(t *testing.T) {
	longTitle := "This is a very long pull request title that exceeds the seventy-two character limit we enforce"
	desc := parsePRDescription(longTitle)

	if len(desc.Title) > 72 {
		t.Errorf("Title length %d exceeds 72", len(desc.Title))
	}
}

func TestBuildPRPrompt_IncludesAllParts(t *testing.T) {
	prompt := buildPRPrompt("diff content", "stat content", "abc123 first commit\ndef456 second commit")

	if !strings.Contains(prompt, "diff content") {
		t.Error("prompt should contain diff")
	}
	if !strings.Contains(prompt, "stat content") {
		t.Error("prompt should contain stat")
	}
	if !strings.Contains(prompt, "abc123 first commit") {
		t.Error("prompt should contain commit log")
	}
}

func TestBuildPRPrompt_TruncatesLargeDiff(t *testing.T) {
	largeDiff := make([]byte, 15000)
	for i := range largeDiff {
		largeDiff[i] = 'x'
	}
	prompt := buildPRPrompt(string(largeDiff), "", "")

	if !strings.Contains(prompt, "[diff truncated") {
		t.Error("should indicate truncation")
	}
}
