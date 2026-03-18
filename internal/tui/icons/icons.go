// Package icons provides Unicode and Nerd Font glyph constants for the TUI.
// When nerdFonts mode is enabled, richer glyphs are used for branches, tags,
// pull requests, stash entries, and other UI elements. When disabled, simple
// Unicode fallbacks are used that work in any terminal font.
package icons

// Icons holds the icon set for the current mode (nerd fonts or plain).
type Icons struct {
	// Sidebar sections
	Branch      string
	BranchOpen  string
	Remote      string
	RemoteOpen  string
	Tag         string
	TagOpen     string
	Stash       string
	StashOpen   string
	PullRequest string
	PROpen      string
	PRMerged    string
	PRClosed    string
	PRDraft     string

	// File status
	Modified  string
	Added     string
	Deleted   string
	Renamed   string
	Copied    string
	Untracked string

	// Git operations
	Commit string
	Push   string
	Pull   string
	Fetch  string
	Merge  string
	Rebase string
	Undo   string
	Redo   string

	// UI elements
	Folder    string
	File      string
	Search    string
	Settings  string
	AI        string
	Check     string
	Cross     string
	Warning   string
	Info      string
	Spinner   string
	Collapsed string
	Expanded  string
	Dirty     string
	Clean     string
	Ahead     string
	Behind    string
	Lock      string
}

// Plain returns the default icon set using simple Unicode characters
// that work in any terminal and font.
func Plain() Icons {
	return Icons{
		Branch:      "⑂",
		BranchOpen:  "⑂",
		Remote:      "☁",
		RemoteOpen:  "☁",
		Tag:         "◆",
		TagOpen:     "◆",
		Stash:       "◌",
		StashOpen:   "◌",
		PullRequest: "⊙",
		PROpen:      "●",
		PRMerged:    "◉",
		PRClosed:    "○",
		PRDraft:     "◌",

		Modified:  "M",
		Added:     "A",
		Deleted:   "D",
		Renamed:   "R",
		Copied:    "C",
		Untracked: "?",

		Commit: "●",
		Push:   "↑",
		Pull:   "↓",
		Fetch:  "⟳",
		Merge:  "⑂",
		Rebase: "↻",
		Undo:   "↺",
		Redo:   "↻",

		Folder:    "▸",
		File:      "·",
		Search:    "/",
		Settings:  "⚙",
		AI:        "✦",
		Check:     "✓",
		Cross:     "✕",
		Warning:   "▲",
		Info:      "ℹ",
		Spinner:   "◐",
		Collapsed: "▸",
		Expanded:  "▾",
		Dirty:     "●",
		Clean:     "○",
		Ahead:     "↑",
		Behind:    "↓",
		Lock:      "🔒",
	}
}

// NerdFont returns the icon set using Nerd Font glyphs.
// Requires a Nerd Font patched terminal font (e.g., JetBrainsMono Nerd Font,
// FiraCode Nerd Font, Hack Nerd Font, etc.).
func NerdFont() Icons {
	return Icons{
		Branch:      "\ue0a0", //  (branch)
		BranchOpen:  "\ue0a0",
		Remote:      "\uf0c2", //  (cloud)
		RemoteOpen:  "\uf0c2",
		Tag:         "\uf02b", //  (tag)
		TagOpen:     "\uf02b",
		Stash:       "\uf01c", //  (inbox)
		StashOpen:   "\uf01c",
		PullRequest: "\uf407", //  (git-pull-request)
		PROpen:      "\uf407",
		PRMerged:    "\uf408", //  (git-merge)
		PRClosed:    "\uf406", //  (git-pull-request)
		PRDraft:     "\uf444", //  (circle-dotted)

		Modified:  "\uf044", //  (pencil)
		Added:     "\uf067", //  (plus)
		Deleted:   "\uf068", //  (minus)
		Renamed:   "\uf074", //  (arrow-right-arrow-left)
		Copied:    "\uf0c5", //  (copy)
		Untracked: "\uf128", //  (question)

		Commit: "\uf417", //  (git-commit)
		Push:   "\uf062", //  (arrow-up)
		Pull:   "\uf063", //  (arrow-down)
		Fetch:  "\uf021", //  (refresh)
		Merge:  "\uf408", //  (git-merge)
		Rebase: "\uf0e2", //  (rotate-left)
		Undo:   "\uf0e2", //  (rotate-left)
		Redo:   "\uf01e", //  (rotate-right)

		Folder:    "\uf07b", //  (folder)
		File:      "\uf15b", //  (file)
		Search:    "\uf002", //  (search)
		Settings:  "\uf013", //  (gear)
		AI:        "\uf0eb", //  (lightbulb)
		Check:     "\uf00c", //  (check)
		Cross:     "\uf00d", //  (xmark)
		Warning:   "\uf071", //  (triangle-exclamation)
		Info:      "\uf05a", //  (circle-info)
		Spinner:   "\uf110", //  (spinner)
		Collapsed: "\uf054", //  (chevron-right)
		Expanded:  "\uf078", //  (chevron-down)
		Dirty:     "\uf06a", //  (circle-exclamation)
		Clean:     "\uf058", //  (circle-check)
		Ahead:     "\uf062", //  (arrow-up)
		Behind:    "\uf063", //  (arrow-down)
		Lock:      "\uf023", //  (lock)
	}
}

// Active is the currently active icon set. Components that don't have
// access to ProgramContext can use this package-level variable.
// It defaults to Plain and is updated by the app on startup.
var Active = Plain()

// ForConfig returns the appropriate icon set based on the nerdFonts setting.
func ForConfig(nerdFonts bool) Icons {
	if nerdFonts {
		return NerdFont()
	}
	return Plain()
}
