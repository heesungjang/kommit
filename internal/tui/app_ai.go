package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heesungjang/kommit/internal/ai"
	"github.com/heesungjang/kommit/internal/tui/dialog"
	"github.com/heesungjang/kommit/internal/tui/pages"
)

// ---------------------------------------------------------------------------
// AI commit message and PR description generation
// ---------------------------------------------------------------------------

// generateAICommitMessage gathers the staged diff and sends it to the
// configured AI provider to generate a commit message. The result is
// delivered back as an AICommitResultMsg or AICommitErrorMsg.
func (a App) generateAICommitMessage() tea.Cmd {
	cfg := a.ctx.Config
	if cfg == nil {
		return func() tea.Msg {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("no configuration loaded")}
		}
	}

	// Resolve API key: config/env > saved credentials.
	apiKey := ai.GetAPIKey(cfg.AI.Provider, cfg.AI.APIKey)
	if apiKey == "" && cfg.AI.Provider != "openai-compatible" {
		// No API key — open the AI Setup dialog for first-time configuration.
		// Reset aiGenerating on the LogPage since we're not actually generating.
		pctx := a.ctx
		return func() tea.Msg {
			return showDialogMsg{model: dialog.NewAISetup(pctx)}
		}
	}

	// Build provider config with resolved key.
	aiCfg := cfg.AI
	aiCfg.APIKey = apiKey

	repo := a.ctx.Repo
	isCopilot := cfg.AI.Provider == "copilot"
	return func() tea.Msg {
		// For Copilot, the saved credential is the GitHub OAuth token.
		// We need to exchange it for a short-lived Copilot bearer token
		// before each generation (the bearer token expires frequently).
		if isCopilot && aiCfg.APIKey != "" {
			exchCtx, exchCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer exchCancel()
			cpToken, err := ai.ExchangeForCopilotToken(exchCtx, aiCfg.APIKey)
			if err != nil {
				return pages.AICommitErrorMsg{Err: fmt.Errorf("copilot token refresh: %w", err)}
			}
			aiCfg.APIKey = cpToken.Token
		}

		// Get staged diff as raw text for the AI prompt.
		diff, err := repo.DiffStagedRaw()
		if err != nil {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("get staged diff: %w", err)}
		}
		if strings.TrimSpace(diff) == "" {
			return pages.AICommitErrorMsg{Err: fmt.Errorf("no staged changes")}
		}

		// Get diff stat for file-level summary.
		stat, _ := repo.DiffStatStagedRaw()

		// Create provider and generate.
		provider, err := ai.NewProvider(&aiCfg)
		if err != nil {
			return pages.AICommitErrorMsg{Err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		msg, err := provider.GenerateCommitMessage(ctx, diff, stat)
		if err != nil {
			return pages.AICommitErrorMsg{Err: err}
		}

		return pages.AICommitResultMsg{
			Summary:     msg.Summary,
			Description: msg.Description,
		}
	}
}

// generatePRDescription uses the AI provider to generate a PR title and body
// from the branch diff against the given base branch.
func (a App) generatePRDescription(baseBranch string) tea.Cmd {
	cfg := a.ctx.Config
	repo := a.repo
	isCopilot := cfg != nil && cfg.AI.Provider == "copilot"

	return func() tea.Msg {
		if cfg == nil {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("no configuration loaded")}
		}

		apiKey := ai.GetAPIKey(cfg.AI.Provider, cfg.AI.APIKey)
		if apiKey == "" && cfg.AI.Provider != "openai-compatible" {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("no AI provider configured — set up in Settings > AI")}
		}

		// For Copilot, exchange for bearer token.
		aiCfg := cfg.AI
		aiCfg.APIKey = apiKey
		if isCopilot && aiCfg.APIKey != "" {
			exchCtx, exchCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer exchCancel()
			cpToken, err := ai.ExchangeForCopilotToken(exchCtx, aiCfg.APIKey)
			if err != nil {
				return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("copilot token refresh: %w", err)}
			}
			aiCfg.APIKey = cpToken.Token
		}

		// Get branch diff.
		diff, err := repo.DiffBranchRaw(baseBranch)
		if err != nil {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("get branch diff: %w", err)}
		}
		if strings.TrimSpace(diff) == "" {
			return dialog.CreatePRAIErrorMsg{Err: fmt.Errorf("no changes between %s and HEAD", baseBranch)}
		}

		stat, _ := repo.DiffStatBranchRaw(baseBranch)
		commitLog, _ := repo.LogBranchOneline(baseBranch)

		genCtx, genCancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer genCancel()

		desc, err := ai.GeneratePRDescription(genCtx, &aiCfg, aiCfg.APIKey, diff, stat, commitLog)
		if err != nil {
			return dialog.CreatePRAIErrorMsg{Err: err}
		}

		return dialog.CreatePRAIResultMsg{Title: desc.Title, Body: desc.Body}
	}
}
