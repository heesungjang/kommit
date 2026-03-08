package keys

import "github.com/charmbracelet/bubbles/key"

// Context represents the current keybinding context that determines which
// bindings are active and shown in the help bar.
type Context int

const (
	ContextGlobal    Context = iota
	ContextStatus            // Status / staging page
	ContextLog               // Commit log page
	ContextBranches          // Branches page
	ContextDiff              // Diff viewer
	ContextStash             // Stash page
	ContextRemotes           // Remotes page
	ContextPR                // Pull request page
	ContextCI                // CI/CD page
	ContextWorkspace         // Workspace / worktree page
	ContextDialog            // Modal dialog overlay
)

// ActiveContext tracks the currently active keybinding context.
// It is updated whenever the user switches pages or opens a dialog.
var ActiveContext = ContextStatus

// ---------------------------------------------------------------------------
// Singleton key sets – initialised once, reused everywhere
// ---------------------------------------------------------------------------

var (
	Global     = NewGlobalKeys()
	Navigation = NewNavigationKeys()
	Status     = NewStatusKeys()
	Branch     = NewBranchKeys()
	Diff       = NewDiffKeys()
	Stash      = NewStashKeys()
	PR         = NewPRKeys()
)

// ---------------------------------------------------------------------------
// ShortHelp – compact list of the most relevant bindings for a context
// ---------------------------------------------------------------------------

// ShortHelp returns a concise set of key bindings for the given context,
// suitable for rendering in a single-line help bar.
func ShortHelp(ctx Context) []key.Binding {
	// Global keys that always appear in the short help.
	common := []key.Binding{
		Global.Help,
		Global.Quit,
	}

	switch ctx {
	case ContextStatus:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Status.Stage,
			Status.Unstage,
			Status.Commit,
			Status.Push,
			Status.AICommit,
		}, common...)

	case ContextLog:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Navigation.Select,
			Global.Search,
		}, common...)

	case ContextBranches:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Branch.Checkout,
			Branch.New,
			Branch.Delete,
			Branch.Merge,
		}, common...)

	case ContextDiff:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Diff.NextHunk,
			Diff.PrevHunk,
			Diff.StageHunk,
			Diff.ToggleView,
		}, common...)

	case ContextStash:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Stash.Save,
			Stash.Pop,
			Stash.Apply,
			Stash.Drop,
		}, common...)

	case ContextRemotes:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Status.Fetch,
			Status.Push,
			Status.Pull,
		}, common...)

	case ContextPR:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			PR.Create,
			PR.Approve,
			PR.Merge,
			PR.Comment,
			PR.AIDescription,
		}, common...)

	case ContextCI:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Navigation.Select,
			Status.Refresh,
		}, common...)

	case ContextWorkspace:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Navigation.Select,
		}, common...)

	case ContextDialog:
		return []key.Binding{
			Navigation.Select,
			Global.Quit,
		}

	default: // ContextGlobal
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Navigation.Select,
			Global.Search,
		}, common...)
	}
}

// ---------------------------------------------------------------------------
// FullHelp – grouped bindings for the expanded help overlay
// ---------------------------------------------------------------------------

// FullHelp returns all key bindings organised into logical groups for the
// given context, suitable for rendering in a multi-column help overlay.
func FullHelp(ctx Context) [][]key.Binding {
	globalGroup := []key.Binding{
		Global.Quit,
		Global.ForceQuit,
		Global.Help,
		Global.Search,
		Global.CommandPalette,
	}

	tabGroup := []key.Binding{
		Global.Tab1,
		Global.Tab2,
		Global.Tab3,
		Global.Tab4,
		Global.Tab5,
		Global.Tab6,
		Global.Tab7,
		Global.Tab8,
	}

	panelGroup := []key.Binding{
		Global.NextPanel,
		Global.PrevPanel,
	}

	navGroup := []key.Binding{
		Navigation.Up,
		Navigation.Down,
		Navigation.Left,
		Navigation.Right,
		Navigation.PageUp,
		Navigation.PageDown,
		Navigation.Home,
		Navigation.End,
		Navigation.Select,
	}

	base := [][]key.Binding{globalGroup, tabGroup, panelGroup, navGroup}

	switch ctx {
	case ContextStatus:
		return append(base, []key.Binding{
			Status.Stage,
			Status.Unstage,
			Status.StageAll,
			Status.StageHunk,
			Status.Commit,
			Status.CommitAmend,
			Status.Push,
			Status.Pull,
			Status.Fetch,
			Status.Discard,
			Status.AICommit,
			Status.Undo,
			Status.Refresh,
		})

	case ContextBranches:
		return append(base, []key.Binding{
			Branch.New,
			Branch.Delete,
			Branch.Rename,
			Branch.Checkout,
			Branch.Merge,
			Branch.Rebase,
		})

	case ContextDiff:
		return append(base, []key.Binding{
			Diff.NextHunk,
			Diff.PrevHunk,
			Diff.StageHunk,
			Diff.ToggleView,
		})

	case ContextStash:
		return append(base, []key.Binding{
			Stash.Save,
			Stash.Pop,
			Stash.Apply,
			Stash.Drop,
		})

	case ContextRemotes:
		return append(base, []key.Binding{
			Status.Push,
			Status.Pull,
			Status.Fetch,
			Status.Refresh,
		})

	case ContextLog:
		return append(base, []key.Binding{
			Navigation.Select,
			Navigation.PageUp,
			Navigation.PageDown,
		})

	case ContextPR:
		return append(base, []key.Binding{
			PR.Create,
			PR.Approve,
			PR.Merge,
			PR.Comment,
			PR.AIDescription,
		})

	default:
		return base
	}
}
