package ai

import "fmt"

// commitMessageSystemPrompt is the system prompt for commit message generation.
const commitMessageSystemPrompt = `You are a commit message generator for a git repository. Given a diff of staged changes, write a concise, informative commit message.

Rules:
- First line is the SUMMARY: max 72 characters, imperative mood ("add", "fix", "refactor", "update", "remove", etc.)
- Use conventional commit style without the type prefix unless the changes clearly fit one (e.g. "fix:", "feat:", "refactor:", "docs:", "chore:")
- If you include a description, separate it from the summary with a blank line
- The description should explain WHY the change was made, not WHAT changed (the diff shows what)
- Keep the description to 2-4 sentences max
- For trivial changes (typo fixes, formatting, single-line changes), output ONLY the summary with no description
- Do NOT wrap your response in markdown code fences
- Do NOT include any preamble, explanation, or commentary — output ONLY the commit message
- If the diff is too large to fully understand, focus on the file names and stat summary to infer intent`

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
