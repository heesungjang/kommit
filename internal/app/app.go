package app

import (
	"fmt"
	"path/filepath"

	"github.com/heesungjang/kommit/internal/config"
	"github.com/heesungjang/kommit/internal/git"
	"github.com/heesungjang/kommit/internal/tui"
)

// Run is the main entry point for the application.
func Run(repoPath string, debug bool) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if debug {
		cfg.Debug = true
	}

	// Resolve repo path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Open the git repository
	repo, err := git.Open(absPath)
	if err != nil {
		return fmt.Errorf("opening repository at %s: %w", absPath, err)
	}

	// Launch TUI
	return tui.Run(repo, &cfg)
}
