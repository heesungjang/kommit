package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/heesungjang/kommit/internal/config"
)

// ---------------------------------------------------------------------------
// parseCommitMessage tests
// ---------------------------------------------------------------------------

func TestParseCommitMessage_SummaryOnly(t *testing.T) {
	msg := parseCommitMessage("fix: resolve nil pointer in sidebar")
	if msg.Summary != "fix: resolve nil pointer in sidebar" {
		t.Errorf("Summary = %q, want %q", msg.Summary, "fix: resolve nil pointer in sidebar")
	}
	if msg.Description != "" {
		t.Errorf("Description = %q, want empty", msg.Description)
	}
}

func TestParseCommitMessage_SummaryAndDescription(t *testing.T) {
	input := "fix: resolve nil pointer in sidebar\n\nThe sidebar component was accessing a nil slice when no branches\nwere loaded. This adds a guard check before iteration."
	msg := parseCommitMessage(input)
	if msg.Summary != "fix: resolve nil pointer in sidebar" {
		t.Errorf("Summary = %q", msg.Summary)
	}
	if msg.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestParseCommitMessage_TruncatesLongSummary(t *testing.T) {
	long := "refactor: completely rewrite the entire rendering pipeline to use a new approach with better performance characteristics and test coverage"
	msg := parseCommitMessage(long)
	if len(msg.Summary) > 72 {
		t.Errorf("Summary length = %d, want <= 72", len(msg.Summary))
	}
}

func TestParseCommitMessage_StripsCodeFences(t *testing.T) {
	input := "```\nfix: update config parser\n\nHandle edge case with empty values.\n```"
	msg := parseCommitMessage(input)
	if msg.Summary != "fix: update config parser" {
		t.Errorf("Summary = %q", msg.Summary)
	}
}

func TestParseCommitMessage_StripsQuotes(t *testing.T) {
	msg := parseCommitMessage(`"fix: update config parser"`)
	if msg.Summary != "fix: update config parser" {
		t.Errorf("Summary = %q", msg.Summary)
	}
}

func TestParseCommitMessage_Empty(t *testing.T) {
	msg := parseCommitMessage("")
	if msg.Summary != "update code" {
		t.Errorf("Summary = %q, want fallback", msg.Summary)
	}
}

// ---------------------------------------------------------------------------
// buildCommitPrompt tests
// ---------------------------------------------------------------------------

func TestBuildCommitPrompt_IncludesStatAndDiff(t *testing.T) {
	prompt := buildCommitPrompt("diff content", "stat summary")
	if prompt == "" {
		t.Error("prompt should not be empty")
	}
	if !contains(prompt, "stat summary") {
		t.Error("prompt should include stat")
	}
	if !contains(prompt, "diff content") {
		t.Error("prompt should include diff")
	}
}

func TestBuildCommitPrompt_TruncatesLargeDiff(t *testing.T) {
	largeDiff := make([]byte, 10000)
	for i := range largeDiff {
		largeDiff[i] = 'x'
	}
	prompt := buildCommitPrompt(string(largeDiff), "")
	if !contains(prompt, "[diff truncated") {
		t.Error("prompt should indicate truncation for large diffs")
	}
}

// ---------------------------------------------------------------------------
// NewProvider tests
// ---------------------------------------------------------------------------

func TestNewProvider_Anthropic(t *testing.T) {
	p, err := NewProvider(&config.AIConfig{
		Provider: "anthropic",
		APIKey:   "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*AnthropicProvider); !ok {
		t.Error("expected *AnthropicProvider")
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	p, err := NewProvider(&config.AIConfig{
		Provider: "openai",
		APIKey:   "sk-test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*OpenAIProvider); !ok {
		t.Error("expected *OpenAIProvider")
	}
}

func TestNewProvider_Compatible(t *testing.T) {
	p, err := NewProvider(&config.AIConfig{
		Provider: "openai-compatible",
		BaseURL:  "http://localhost:11434",
		Model:    "llama2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*OpenAIProvider); !ok {
		t.Error("expected *OpenAIProvider for compatible provider")
	}
}

func TestNewProvider_NoAPIKey(t *testing.T) {
	_, err := NewProvider(&config.AIConfig{
		Provider: "anthropic",
	})
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestNewProvider_UnsupportedProvider(t *testing.T) {
	_, err := NewProvider(&config.AIConfig{
		Provider: "unknown",
		APIKey:   "sk-test",
	})
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestNewProvider_Nil(t *testing.T) {
	_, err := NewProvider(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewProvider_CompatibleNoBaseURL(t *testing.T) {
	_, err := NewProvider(&config.AIConfig{
		Provider: "openai-compatible",
		APIKey:   "key",
	})
	if err == nil {
		t.Error("expected error for compatible provider without baseURL")
	}
}

// ---------------------------------------------------------------------------
// Anthropic integration test (mock HTTP server)
// ---------------------------------------------------------------------------

func TestAnthropicProvider_GenerateCommitMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers.
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version header")
		}

		resp := anthropicResponse{
			Content: []anthropicContent{
				{Type: "text", Text: "fix: resolve nil pointer in sidebar\n\nGuard against empty slice."},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		apiKey:  "test-key",
		model:   "claude-sonnet-4-20250514",
		baseURL: srv.URL,
	}

	msg, err := p.GenerateCommitMessage(context.Background(), "diff --git ...", "1 file changed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Summary != "fix: resolve nil pointer in sidebar" {
		t.Errorf("Summary = %q", msg.Summary)
	}
	if msg.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestAnthropicProvider_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := anthropicResponse{
			Error: &anthropicError{Type: "authentication_error", Message: "invalid api key"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := &AnthropicProvider{apiKey: "bad-key", model: "claude-sonnet-4-20250514", baseURL: srv.URL}
	_, err := p.GenerateCommitMessage(context.Background(), "diff", "stat")
	if err == nil {
		t.Error("expected error for 401 response")
	}
	if !contains(err.Error(), "invalid api key") {
		t.Errorf("error should contain API message, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// OpenAI integration test (mock HTTP server)
// ---------------------------------------------------------------------------

func TestOpenAIProvider_GenerateCommitMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header")
		}

		resp := openAIResponse{
			Choices: []openAIChoice{
				{Message: openAIMessage{Role: "assistant", Content: "refactor: extract sidebar component\n\nImproves maintainability."}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := &OpenAIProvider{apiKey: "test-key", model: "gpt-4o-mini", baseURL: srv.URL}

	msg, err := p.GenerateCommitMessage(context.Background(), "diff --git ...", "2 files changed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Summary != "refactor: extract sidebar component" {
		t.Errorf("Summary = %q", msg.Summary)
	}
}

func TestOpenAIProvider_NoAuthForLocal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Local providers (Ollama) should work without auth header.
		if r.Header.Get("Authorization") != "" {
			t.Errorf("did not expect Authorization header for keyless provider")
		}
		resp := openAIResponse{
			Choices: []openAIChoice{
				{Message: openAIMessage{Role: "assistant", Content: "update readme"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := &OpenAIProvider{apiKey: "", model: "llama2", baseURL: srv.URL}
	msg, err := p.GenerateCommitMessage(context.Background(), "diff", "stat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Summary != "update readme" {
		t.Errorf("Summary = %q", msg.Summary)
	}
}

// ---------------------------------------------------------------------------
// Credentials tests
// ---------------------------------------------------------------------------

func TestCredentials_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	err := SetAPIKey("anthropic", "sk-test-123")
	if err != nil {
		t.Fatalf("SetAPIKey error: %v", err)
	}

	// Verify file was created with restrictive permissions.
	path := filepath.Join(tmpDir, "kommit", "auth.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("auth.json not found: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}

	// Load and verify.
	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials error: %v", err)
	}
	if creds.Providers["anthropic"].APIKey != "sk-test-123" {
		t.Errorf("APIKey = %q, want %q", creds.Providers["anthropic"].APIKey, "sk-test-123")
	}
}

func TestCredentials_LoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials error: %v", err)
	}
	if len(creds.Providers) != 0 {
		t.Error("expected empty providers map")
	}
}

func TestGetAPIKey_ConfigTakesPriority(t *testing.T) {
	key := GetAPIKey("anthropic", "sk-from-config")
	if key != "sk-from-config" {
		t.Errorf("key = %q, want config key", key)
	}
}

func TestGetAPIKey_FallsBackToCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	_ = SetAPIKey("openai", "sk-saved")

	key := GetAPIKey("openai", "")
	if key != "sk-saved" {
		t.Errorf("key = %q, want saved key", key)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
