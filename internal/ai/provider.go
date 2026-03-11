// Package ai provides AI provider abstractions for generating commit messages
// and other AI-powered features. It supports Anthropic, OpenAI, and any
// OpenAI-compatible endpoint (Ollama, Groq, DeepSeek, LM Studio, etc.).
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/heesungjang/kommit/internal/config"
)

// CommitMessage holds the AI-generated commit message split into summary and
// description. The summary is the subject line (max ~72 chars, imperative
// mood). The description explains the "why" and is optional for trivial
// changes.
type CommitMessage struct {
	Summary     string
	Description string
}

// Provider is the interface that all AI providers must implement.
type Provider interface {
	// GenerateCommitMessage generates a commit message from the staged diff
	// and diff stat output. The diff may be truncated for large changesets.
	GenerateCommitMessage(ctx context.Context, diff, stat string) (*CommitMessage, error)
}

// ErrNoAPIKey is returned when no API key is configured for the provider.
var ErrNoAPIKey = errors.New("no API key configured")

// ErrUnsupportedProvider is returned for unknown provider names.
var ErrUnsupportedProvider = errors.New("unsupported AI provider")

// NewProvider creates a Provider based on the given AI config.
// It resolves the API key from the config (which may have been loaded from
// env vars or config file) and returns ErrNoAPIKey if none is available.
func NewProvider(cfg *config.AIConfig) (Provider, error) {
	if cfg == nil {
		return nil, ErrNoAPIKey
	}

	switch cfg.Provider {
	case "anthropic":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("%w: set ANTHROPIC_API_KEY environment variable", ErrNoAPIKey)
		}
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = anthropicDefaultBaseURL
		}
		return &AnthropicProvider{
			apiKey:  cfg.APIKey,
			model:   resolveAnthropicModel(cfg.Model),
			baseURL: baseURL,
		}, nil

	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("%w: set OPENAI_API_KEY environment variable", ErrNoAPIKey)
		}
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = openAIDefaultBaseURL
		}
		return &OpenAIProvider{
			apiKey:  cfg.APIKey,
			model:   resolveOpenAIModel(cfg.Model),
			baseURL: baseURL,
		}, nil

	case "openai-compatible":
		if cfg.BaseURL == "" {
			return nil, errors.New("openai-compatible provider requires a baseUrl in config")
		}
		model := cfg.Model
		if model == "" {
			model = "default"
		}
		return &OpenAIProvider{
			apiKey:  cfg.APIKey, // may be empty for local models (Ollama, etc.)
			model:   model,
			baseURL: cfg.BaseURL,
		}, nil

	case "copilot":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("%w: use 'ctrl+g' to login with GitHub Copilot", ErrNoAPIKey)
		}
		return NewCopilotProvider(cfg.APIKey, cfg.Model), nil

	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProvider, cfg.Provider)
	}
}

// DefaultModel returns the default model for a given provider.
func DefaultModel(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-sonnet-4-6"
	case "openai":
		return "gpt-4o-mini"
	case "copilot":
		return copilotDefaultModel // "gpt-4o"
	default:
		return ""
	}
}

// resolveAnthropicModel returns a valid Anthropic model ID.
// If the configured model is empty or generic, it defaults to a known good model.
func resolveAnthropicModel(model string) string {
	if model == "" {
		return DefaultModel("anthropic")
	}
	return model
}

// resolveOpenAIModel returns a valid OpenAI model ID.
func resolveOpenAIModel(model string) string {
	if model == "" {
		return DefaultModel("openai")
	}
	return model
}

// parseCommitMessage parses the AI response text into a CommitMessage.
// It expects the first line to be the summary and everything after the
// first blank line to be the description.
func parseCommitMessage(text string) *CommitMessage {
	text = strings.TrimSpace(text)
	if text == "" {
		return &CommitMessage{Summary: "update code"}
	}

	// Strip markdown code fences if the model wrapped its response.
	text = stripCodeFences(text)

	lines := strings.SplitN(text, "\n", 2)
	summary := strings.TrimSpace(lines[0])

	// Remove conventional commit type prefix quotes if present.
	// Some models wrap the summary in quotes.
	summary = strings.Trim(summary, "\"'`")

	// Enforce max summary length.
	if len(summary) > 72 {
		summary = summary[:69] + "..."
	}

	var desc string
	if len(lines) > 1 {
		desc = strings.TrimSpace(lines[1])
		// Strip leading blank line(s) between summary and body.
		desc = strings.TrimLeft(desc, "\n")
		desc = strings.TrimSpace(desc)
	}

	return &CommitMessage{
		Summary:     summary,
		Description: desc,
	}
}

// stripCodeFences removes markdown code fences (```...```) that some
// models wrap their response in.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	// Check for opening fence.
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line.
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		// Remove closing fence.
		if strings.HasSuffix(strings.TrimSpace(s), "```") {
			s = strings.TrimSpace(s)
			s = s[:len(s)-3]
			s = strings.TrimSpace(s)
		}
	}
	return s
}
