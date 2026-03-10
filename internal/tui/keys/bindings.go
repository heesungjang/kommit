package keys

import "github.com/charmbracelet/bubbles/key"

// ---------------------------------------------------------------------------
// Global keybindings – always active regardless of context
// ---------------------------------------------------------------------------

// GlobalKeys defines keybindings that are available everywhere in the TUI.
type GlobalKeys struct {
	Quit           key.Binding
	ForceQuit      key.Binding
	Help           key.Binding
	Search         key.Binding
	CommandPalette key.Binding
	Tab1           key.Binding
	Tab2           key.Binding
	Tab3           key.Binding
	Tab4           key.Binding
	Tab5           key.Binding
	Tab6           key.Binding
	Tab7           key.Binding
	Tab8           key.Binding
	NextPanel      key.Binding
	PrevPanel      key.Binding
}

// NewGlobalKeys returns populated global keybindings.
func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		CommandPalette: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "command palette"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "status"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "log"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "branches"),
		),
		Tab4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "remotes"),
		),
		Tab5: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "stash"),
		),
		Tab6: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "pull requests"),
		),
		Tab7: key.NewBinding(
			key.WithKeys("7"),
			key.WithHelp("7", "CI/CD"),
		),
		Tab8: key.NewBinding(
			key.WithKeys("8"),
			key.WithHelp("8", "workspace"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev panel"),
		),
	}
}

// ---------------------------------------------------------------------------
// Navigation keybindings – list / tree movement
// ---------------------------------------------------------------------------

// NavigationKeys defines keybindings for moving through lists and trees.
type NavigationKeys struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Select   key.Binding
}

// NewNavigationKeys returns populated navigation keybindings.
func NewNavigationKeys() NavigationKeys {
	return NavigationKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
	}
}

// ---------------------------------------------------------------------------
// Status page keybindings
// ---------------------------------------------------------------------------

// StatusKeys defines keybindings for the status / staging page.
type StatusKeys struct {
	Stage       key.Binding
	Unstage     key.Binding
	StageAll    key.Binding
	StageHunk   key.Binding
	Commit      key.Binding
	CommitAmend key.Binding
	Push        key.Binding
	Pull        key.Binding
	Fetch       key.Binding
	Discard     key.Binding
	AICommit    key.Binding
	Undo        key.Binding
	Refresh     key.Binding
}

// NewStatusKeys returns populated status page keybindings.
func NewStatusKeys() StatusKeys {
	return StatusKeys{
		Stage: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stage file"),
		),
		Unstage: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "unstage file"),
		),
		StageAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "stage all"),
		),
		StageHunk: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "stage hunk"),
		),
		Commit: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "commit"),
		),
		CommitAmend: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "amend commit"),
		),
		Push: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "push"),
		),
		Pull: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "pull"),
		),
		Fetch: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "fetch"),
		),
		Discard: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "discard changes"),
		),
		AICommit: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "AI commit message"),
		),
		Undo: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "undo"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// ---------------------------------------------------------------------------
// Branch page keybindings
// ---------------------------------------------------------------------------

// BranchKeys defines keybindings for the branches page.
type BranchKeys struct {
	New      key.Binding
	Delete   key.Binding
	Rename   key.Binding
	Checkout key.Binding
	Merge    key.Binding
	Rebase   key.Binding
}

// NewBranchKeys returns populated branch page keybindings.
func NewBranchKeys() BranchKeys {
	return BranchKeys{
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new branch"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete branch"),
		),
		Rename: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "rename branch"),
		),
		Checkout: key.NewBinding(
			key.WithKeys("enter", "o"),
			key.WithHelp("enter/o", "checkout"),
		),
		Merge: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "merge into current"),
		),
		Rebase: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "rebase onto current"),
		),
	}
}

// ---------------------------------------------------------------------------
// Commit operations keybindings (actions on past commits)
// ---------------------------------------------------------------------------

// CommitOpsKeys defines keybindings for operations on past commits.
type CommitOpsKeys struct {
	Revert     key.Binding
	CherryPick key.Binding
	CopyHash   key.Binding
}

// NewCommitOpsKeys returns populated commit operation keybindings.
func NewCommitOpsKeys() CommitOpsKeys {
	return CommitOpsKeys{
		Revert: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "revert commit"),
		),
		CherryPick: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "cherry-pick"),
		),
		CopyHash: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy hash"),
		),
	}
}

// ---------------------------------------------------------------------------
// Diff view keybindings
// ---------------------------------------------------------------------------

// DiffKeys defines keybindings for the diff viewer.
type DiffKeys struct {
	NextHunk   key.Binding
	PrevHunk   key.Binding
	StageHunk  key.Binding
	ToggleView key.Binding
}

// NewDiffKeys returns populated diff view keybindings.
func NewDiffKeys() DiffKeys {
	return DiffKeys{
		NextHunk: key.NewBinding(
			key.WithKeys("n", "]"),
			key.WithHelp("n/]", "next hunk"),
		),
		PrevHunk: key.NewBinding(
			key.WithKeys("N", "["),
			key.WithHelp("N/[", "prev hunk"),
		),
		StageHunk: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stage hunk"),
		),
		ToggleView: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "inline/side-by-side"),
		),
	}
}

// ---------------------------------------------------------------------------
// Stash page keybindings
// ---------------------------------------------------------------------------

// StashKeys defines keybindings for the stash page.
type StashKeys struct {
	Save  key.Binding
	Pop   key.Binding
	Apply key.Binding
	Drop  key.Binding
}

// NewStashKeys returns populated stash page keybindings.
func NewStashKeys() StashKeys {
	return StashKeys{
		Save: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stash save"),
		),
		Pop: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "stash pop"),
		),
		Apply: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "stash apply"),
		),
		Drop: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "stash drop"),
		),
	}
}

// ---------------------------------------------------------------------------
// Remote operations keybindings – always active globally
// ---------------------------------------------------------------------------

// RemoteOpsKeys defines keybindings for push/pull/fetch that work from any page.
type RemoteOpsKeys struct {
	Push      key.Binding
	ForcePush key.Binding
	Pull      key.Binding
	Fetch     key.Binding
}

// NewRemoteOpsKeys returns populated remote operation keybindings.
func NewRemoteOpsKeys() RemoteOpsKeys {
	return RemoteOpsKeys{
		Push: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "push"),
		),
		ForcePush: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "force push"),
		),
		Pull: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "pull"),
		),
		Fetch: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "fetch"),
		),
	}
}

// ---------------------------------------------------------------------------
// Pull request page keybindings
// ---------------------------------------------------------------------------

// PRKeys defines keybindings for the pull request page.
type PRKeys struct {
	Create        key.Binding
	Approve       key.Binding
	Merge         key.Binding
	Comment       key.Binding
	AIDescription key.Binding
}

// NewPRKeys returns populated pull request page keybindings.
func NewPRKeys() PRKeys {
	return PRKeys{
		Create: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "create PR"),
		),
		Approve: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "approve PR"),
		),
		Merge: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "merge PR"),
		),
		Comment: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "add comment"),
		),
		AIDescription: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "AI description"),
		),
	}
}
