package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/heesungjang/kommit/internal/config"
)

// commitMessageSystemPrompt is the system prompt for commit message generation.
const commitMessageSystemPrompt = `You are a commit message generator for a git repository. Given a diff of staged changes, write a concise, informative commit message.

Rules:
- First line is the SUMMARY: max 72 characters, imperative mood ("add", "fix", "refactor", "update", "remove", etc.)
- Use conventional commit style without the type prefix unless the changes clearly fit one (e.g. "fix:", "feat:", "refactor:", "docs:", "chore:")
- If you include a description, separate it from the summary with a blank line
- The description MUST use bullet points (- prefix), one per logical change
- Each bullet should be concise (one line) and explain WHAT was changed and WHY
- Keep to 2-5 bullet points max
- For trivial changes (typo fixes, formatting, single-line changes), output ONLY the summary with no description
- Do NOT wrap your response in markdown code fences
- Do NOT include any preamble, explanation, or commentary — output ONLY the commit message
- If the diff is too large to fully understand, focus on the file names and stat summary to infer intent`

// CommitMessageSystemPrompt returns the system prompt for commit message generation.
func CommitMessageSystemPrompt() string {
	return commitMessageSystemPrompt
}

// BuildCommitPrompt constructs the user prompt for commit message generation.
// It includes the diff stat summary and the (potentially truncated) diff.
func BuildCommitPrompt(diff, stat string) string {
	return buildCommitPrompt(diff, stat)
}

// buildCommitPrompt constructs the user prompt for commit message generation.
// It includes the diff stat summary and the (potentially truncated) diff.
func buildCommitPrompt(diff, stat string) string {
	// Truncate diff if too large to fit in context window.
	// Most models handle ~100K tokens; ~8K chars of diff is a reasonable
	// limit that keeps cost low and latency fast while providing enough
	// context for a good commit message.
	const maxDiffChars = 8000

	truncated := false
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars]
		truncated = true
	}

	var prompt string
	if stat != "" {
		prompt = fmt.Sprintf("Diff stat summary:\n%s\n\n", stat)
	}
	prompt += fmt.Sprintf("Diff:\n%s", diff)
	if truncated {
		prompt += "\n\n[diff truncated — use the stat summary above for full file list]"
	}

	return prompt
}

// ---------------------------------------------------------------------------
// AI Code Explanation
// ---------------------------------------------------------------------------

// explainSystemPrompt is the system prompt for explaining code changes.
const explainSystemPrompt = `You are a code reviewer explaining git changes to a developer. Given a diff (and optionally a commit message), provide a clear, concise explanation of what the code changes do and why they were likely made.

Rules:
- Be concise: 3-8 sentences for simple changes, up to a short paragraph for complex ones
- Focus on the "what" and "why", not line-by-line description
- Mention any potential concerns (breaking changes, edge cases, performance)
- Use plain language — avoid jargon unless the code is domain-specific
- Do NOT wrap your response in markdown code fences
- Do NOT include any preamble like "This diff..." — start directly with the explanation`

// ExplainResult holds the AI-generated explanation of a code change.
type ExplainResult struct {
	Explanation string
}

// buildExplainPrompt constructs the user prompt for code explanation.
func buildExplainPrompt(diff, subject string) string {
	const maxDiffChars = 10000

	truncated := false
	if len(diff) > maxDiffChars {
		diff = diff[:maxDiffChars]
		truncated = true
	}

	var parts []string
	if subject != "" {
		parts = append(parts, fmt.Sprintf("Commit message: %s", subject))
	}
	parts = append(parts, fmt.Sprintf("Diff:\n%s", diff))
	if truncated {
		parts = append(parts, "[diff truncated]")
	}

	return strings.Join(parts, "\n\n")
}

// GenerateExplanation uses the configured AI provider to explain a diff.
func GenerateExplanation(ctx context.Context, cfg *config.AIConfig, apiKey, diff, subject string) (*ExplainResult, error) {
	if cfg == nil {
		return nil, ErrNoAPIKey
	}

	aiCfg := *cfg
	aiCfg.APIKey = apiKey

	provider, err := NewProvider(&aiCfg)
	if err != nil {
		return nil, err
	}

	prompt := buildExplainPrompt(diff, subject)

	result, err := generateWithSystemPrompt(ctx, provider, explainSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	return &ExplainResult{Explanation: strings.TrimSpace(result)}, nil
}
