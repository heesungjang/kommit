package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/heesungjang/kommit/internal/config"
)

// PRDescription holds the AI-generated title and body for a pull request.
type PRDescription struct {
	Title string
	Body  string
}

// prSystemPrompt is the system prompt for PR description generation.
const prSystemPrompt = `You are a pull request description generator. Given a diff between branches and a commit log, write a concise, informative PR title and description.

Rules:
- First line is the TITLE: max 72 characters, descriptive of the overall change
- After a blank line, write the BODY in markdown format
- The body should include:
  - A brief summary (1-2 sentences) of what this PR does
  - A bullet list of key changes if there are multiple
  - Keep it concise — aim for 3-8 lines total in the body
- Use imperative mood for the title ("Add", "Fix", "Update", not "Added", "Fixed", "Updated")
- Do NOT wrap your response in markdown code fences
- Do NOT include any preamble, explanation, or commentary — output ONLY the title and body
- Do NOT include "PR:" or "Pull Request:" prefix in the title`

// buildPRPrompt constructs the user prompt for PR description generation.
func buildPRPrompt(diff, stat, commitLog string) string {
	const maxDiffChars = 12000

	truncated := false
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars]
		truncated = true
	}

	var parts []string

	if commitLog != "" {
		parts = append(parts, fmt.Sprintf("Commits in this branch:\n%s", commitLog))
	}

	if stat != "" {
		parts = append(parts, fmt.Sprintf("Diff stat summary:\n%s", stat))
	}

	parts = append(parts, fmt.Sprintf("Diff:\n%s", diff))

	if truncated {
		parts = append(parts, "[diff truncated — use the stat summary and commit log above for context]")
	}

	return strings.Join(parts, "\n\n")
}

// parsePRDescription parses the AI response into title and body.
func parsePRDescription(text string) *PRDescription {
	text = strings.TrimSpace(text)
	text = stripCodeFences(text)

	if text == "" {
		return &PRDescription{Title: "Update", Body: ""}
	}

	lines := strings.SplitN(text, "\n", 2)
	title := strings.TrimSpace(lines[0])
	title = strings.Trim(title, "\"'`")
	title = strings.TrimPrefix(title, "# ")

	if len(title) > 72 {
		title = title[:69] + "..."
	}

	var body string
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
		body = strings.TrimLeft(body, "\n")
		body = strings.TrimSpace(body)
	}

	return &PRDescription{
		Title: title,
		Body:  body,
	}
}

// GeneratePRDescription uses the configured AI provider to generate a PR
// title and body from the branch diff, stat, and commit log.
func GeneratePRDescription(ctx context.Context, cfg *config.AIConfig, apiKey, diff, stat, commitLog string) (*PRDescription, error) {
	if cfg == nil {
		return nil, ErrNoAPIKey
	}

	aiCfg := *cfg
	aiCfg.APIKey = apiKey

	provider, err := NewProvider(&aiCfg)
	if err != nil {
		return nil, err
	}

	prompt := buildPRPrompt(diff, stat, commitLog)

	// Use the provider's commit message generation but with PR system prompt.
	// We'll call the underlying provider method with our custom prompt.
	result, err := generateWithSystemPrompt(ctx, provider, prSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	return parsePRDescription(result), nil
}

// generateWithSystemPrompt calls the provider's underlying API with a custom system prompt.
// This allows reusing the same provider infrastructure for different generation tasks.
func generateWithSystemPrompt(ctx context.Context, provider Provider, systemPrompt, userPrompt string) (string, error) {
	switch p := provider.(type) {
	case *AnthropicProvider:
		return p.generate(ctx, systemPrompt, userPrompt)
	case *OpenAIProvider:
		return p.generate(ctx, systemPrompt, userPrompt)
	case *CopilotProvider:
		return p.generate(ctx, systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("provider does not support custom system prompts")
	}
}
