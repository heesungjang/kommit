package app

import (
	"fmt"
	"path/filepath"

	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui"
)

// Run is the main entry point for the application.
func Run(repoPath string, debug, workspaceMode bool) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if debug {
		cfg.Debug = true
	}

	// If workspace mode is forced, launch without opening a repo.
	if workspaceMode {
		return tui.Run(nil, &cfg)
	}

	// Resolve repo path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Open the git repository
	repo, err := git.Open(absPath)
	if err != nil {
		// If we're not in a git repo but have workspaces or recent repos
		// configured, fall back to workspace mode.
		if len(cfg.Workspaces) > 0 || len(cfg.RecentRepos) > 0 {
			return tui.Run(nil, &cfg)
		}
		return fmt.Errorf("opening repository at %s: %w", absPath, err)
	}

	// Track this repo in recent repos.
	cfg.AddRecentRepo(absPath)
	_ = config.Save(&cfg)

	// Launch TUI
	return tui.Run(repo, &cfg)
}
