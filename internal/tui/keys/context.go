package keys

import "github.com/charmbracelet/bubbles/key"

// Context represents the current keybinding context that determines which
// bindings are active and shown in the help bar.
type Context int

const (
	ContextGlobal    Context = iota
	ContextStatus            // WIP / staging panel (right panel when WIP selected)
	ContextLog               // Commit log list (center panel)
	ContextBranches          // Sidebar: branches section
	ContextDiff              // Diff viewer (center panel in diff mode)
	ContextStash             // Sidebar: stash section
	ContextRemotes           // Sidebar: remotes section
	ContextDetail            // Commit detail panel (right panel, non-WIP)
	ContextPR                // Pull request page (future)
	ContextCI                // CI/CD page (future)
	ContextWorkspace         // Workspace / worktree page (future)
	ContextDialog            // Modal dialog overlay
)

// ActiveContext tracks the currently active keybinding context.
// It is updated whenever the user switches pages or opens a dialog.
var ActiveContext = ContextLog

// ContextLabel returns a short human-readable label for the given context.
func ContextLabel(ctx Context) string {
	switch ctx {
	case ContextStatus:
		return "WIP"
	case ContextLog:
		return "Commits"
	case ContextBranches:
		return "Branches"
	case ContextDiff:
		return "Diff"
	case ContextStash:
		return "Stash"
	case ContextRemotes:
		return "Remotes"
	case ContextDetail:
		return "Detail"
	case ContextPR:
		return "Pull Requests"
	case ContextDialog:
		return "Dialog"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Singleton key sets – initialized once, reused everywhere
// ---------------------------------------------------------------------------

var (
	Global     = NewGlobalKeys()
	Navigation = NewNavigationKeys()
	Status     = NewStatusKeys()
	Branch     = NewBranchKeys()
	CommitOps  = NewCommitOpsKeys()
	Diff       = NewDiffKeys()
	Stash      = NewStashKeys()
	PR         = NewPRKeys()
	RemoteOps  = NewRemoteOpsKeys()
)

// ---------------------------------------------------------------------------
// ShortHelp – compact list of the most relevant bindings for a context
// ---------------------------------------------------------------------------

// ShortHelp returns a concise set of key bindings for the given context,
// suitable for rendering in a single-line help bar.
func ShortHelp(ctx Context) []key.Binding {
	common := []key.Binding{
		Global.Help,
		Global.CommandPalette,
		Global.Settings,
	}

	switch ctx {
	case ContextStatus:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Status.Stage,
			Status.Unstage,
			Status.StageAll,
			Status.Commit,
			Status.CommitAmend,
			Status.Discard,
		}, common...)

	case ContextLog:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			CommitOps.CopyHash,
			CommitOps.ResetMenu,
			CommitOps.BisectMenu,
			CommitOps.CompareRef,
			CommitOps.Undo,
			Global.Search,
		}, common...)

	case ContextDetail:
		return append([]key.Binding{
			Navigation.Left,
			Navigation.Right,
			Navigation.Up,
			Navigation.Down,
			Navigation.Select,
		}, common...)

	case ContextBranches:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Branch.Checkout,
			Branch.New,
			Branch.Delete,
			Branch.Rename,
			Branch.Merge,
			Branch.Rebase,
		}, common...)

	case ContextDiff:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			Diff.NextHunk,
			Diff.PrevHunk,
			Diff.StageHunk,
			Diff.UnstageHunk,
			Diff.VisualMode,
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
			RemoteOps.Fetch,
			RemoteOps.Push,
			RemoteOps.Pull,
		}, common...)

	case ContextPR:
		return append([]key.Binding{
			Navigation.Up,
			Navigation.Down,
			PR.Create,
			PR.Approve,
			PR.Merge,
			PR.Comment,
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

// FullHelp returns all key bindings organized into logical groups for the
// given context, suitable for rendering in a multi-column help overlay.
func FullHelp(ctx Context) [][]key.Binding {
	globalGroup := []key.Binding{
		Global.Quit,
		Global.ForceQuit,
		Global.Help,
		Global.Search,
		Global.CommandPalette,
		Global.CustomCommands,
		Global.Settings,
	}

	panelGroup := []key.Binding{
		Global.Panel1,
		Global.Panel2,
		Global.Panel3,
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

	base := [][]key.Binding{globalGroup, panelGroup, navGroup}

	switch ctx {
	case ContextStatus:
		return append(base, []key.Binding{
			Status.Stage,
			Status.Unstage,
			Status.StageAll,
			Status.StageHunk,
			Status.Commit,
			Status.CommitAmend,
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
			Diff.UnstageHunk,
			Diff.VisualMode,
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
			RemoteOps.Push,
			RemoteOps.ForcePush,
			RemoteOps.Pull,
			RemoteOps.Fetch,
		})

	case ContextLog:
		return append(base,
			[]key.Binding{
				CommitOps.Revert,
				CommitOps.CherryPick,
				CommitOps.CopyHash,
				CommitOps.ResetMenu,
				CommitOps.Squash,
				CommitOps.Fixup,
				CommitOps.Drop,
				CommitOps.CompareRef,
				CommitOps.BisectMenu,
				CommitOps.Undo,
				CommitOps.Redo,
			},
			[]key.Binding{
				RemoteOps.Push,
				RemoteOps.Pull,
				RemoteOps.Fetch,
			},
		)

	case ContextDetail:
		return append(base, []key.Binding{
			Navigation.Left,
			Navigation.Right,
			Navigation.Select,
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
