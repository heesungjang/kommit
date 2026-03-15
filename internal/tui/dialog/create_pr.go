package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuictx "github.com/heesungjang/kommit/internal/tui/context"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

// CreatePRSubmitMsg is sent when the user submits the create PR form.
type CreatePRSubmitMsg struct {
	Title string
	Body  string
	Head  string // source branch
	Base  string // target branch
	Draft bool
}

// CreatePRCancelMsg is sent when the user cancels PR creation.
type CreatePRCancelMsg struct{}

// CreatePRRequestAIMsg is sent to request AI generation of title and body.
type CreatePRRequestAIMsg struct {
	BaseBranch string
}

// CreatePRAIResultMsg carries the AI-generated title and body back.
type CreatePRAIResultMsg struct {
	Title string
	Body  string
}

// CreatePRAIErrorMsg is sent when AI generation fails.
type CreatePRAIErrorMsg struct {
	Err error
}

// CreatePRStatsMsg carries commit/diff stats loaded asynchronously.
type CreatePRStatsMsg struct {
	CommitCount  int
	FilesChanged int
	Additions    int
	Deletions    int
}

// CreatePRBranchPushedMsg tells the dialog whether the head branch is on the remote.
type CreatePRBranchPushedMsg struct {
	Pushed bool
}

// CreatePRRefreshStatsMsg asks the app shell to reload stats for a new base branch.
type CreatePRRefreshStatsMsg struct {
	BaseBranch string
}

// CreatePRPushRequestMsg asks the app shell to push the head branch to origin.
type CreatePRPushRequestMsg struct{}

// CreatePRPushDoneMsg carries the push result back to the dialog.
type CreatePRPushDoneMsg struct {
	Err error
}

// ---------------------------------------------------------------------------
// CreatePR dialog
// ---------------------------------------------------------------------------

type createPRField int

const (
	prFieldBase createPRField = iota
	prFieldTitle
	prFieldBody
)

// CreatePR is a dialog for creating a new pull request.
type CreatePR struct {
	Base Base
	ctx  *tuictx.ProgramContext

	// Branch info
	headBranch     string   // current branch (source) — read-only
	baseBranch     string   // target branch (editable via picker)
	defaultBranch  string   // repository default branch (for reset)
	remoteBranches []string // available remote branch names

	// Inline branch picker state
	basePicking bool   // true when the branch picker overlay is open
	baseCursor  int    // cursor in branch list
	baseFilter  string // type-to-filter in branch picker

	// Fields
	titleInput textinput.Model
	bodyInput  textarea.Model
	field      createPRField
	editing    bool // true when actively typing in textarea

	// Options
	draft bool

	// Stats (loaded async)
	commitCount  int
	filesChanged int
	additions    int
	deletions    int
	statsLoading bool

	// Remote push state
	branchPushed bool
	pushChecked  bool // whether we've checked remote push status
	pushing      bool // push in progress

	// AI generation state
	aiGenerating bool
	errMsg       string
}

// NewCreatePR creates a new Create PR dialog.
func NewCreatePR(headBranch, baseBranch string, remoteBranches []string, ctx *tuictx.ProgramContext) CreatePR {
	t := theme.Active

	ti := textinput.New()
	ti.Placeholder = "PR title"
	ti.CharLimit = 120
	ti.Width = 55
	ti.PromptStyle = lipgloss.NewStyle().Foreground(t.Blue).Background(t.Surface0)
	ti.TextStyle = lipgloss.NewStyle().Foreground(t.Text).Background(t.Surface0)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(t.Overlay0).Background(t.Surface0)

	ta := textarea.New()
	ta.Placeholder = "PR description (optional, supports markdown)"
	ta.CharLimit = 4096
	ta.SetWidth(55)
	ta.SetHeight(6)
	ta.ShowLineNumbers = false

	// Make textarea fully opaque with Surface0 background.
	baseBg := lipgloss.NewStyle().Background(t.Surface0)
	ta.FocusedStyle.Base = baseBg
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(t.Text)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(t.Overlay0)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(t.Surface1)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(t.Blue)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(t.Surface2)

	ta.BlurredStyle.Base = baseBg
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(t.Subtext0)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(t.Overlay0)
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(t.Surface2)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(t.Surface2)

	ta.Blur()
	ti.Blur()

	hint := "tab:next  ctrl+s:create  ctrl+g:AI  ctrl+d:draft  ctrl+p:push  esc:cancel"

	return CreatePR{
		Base:           NewBaseWithContext("Create Pull Request", hint, 65, 40, ctx),
		ctx:            ctx,
		headBranch:     headBranch,
		baseBranch:     baseBranch,
		defaultBranch:  baseBranch,
		remoteBranches: remoteBranches,
		titleInput:     ti,
		bodyInput:      ta,
		field:          prFieldBase,
		statsLoading:   true,
	}
}

func (d CreatePR) Init() tea.Cmd {
	return nil
}

func (d CreatePR) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CreatePRAIResultMsg:
		d.aiGenerating = false
		d.errMsg = ""
		if msg.Title != "" {
			d.titleInput.SetValue(msg.Title)
		}
		if msg.Body != "" {
			d.bodyInput.SetValue(msg.Body)
		}
		return d, nil

	case CreatePRAIErrorMsg:
		d.aiGenerating = false
		d.errMsg = "AI error: " + msg.Err.Error()
		return d, nil

	case CreatePRStatsMsg:
		d.statsLoading = false
		d.commitCount = msg.CommitCount
		d.filesChanged = msg.FilesChanged
		d.additions = msg.Additions
		d.deletions = msg.Deletions
		return d, nil

	case CreatePRBranchPushedMsg:
		d.pushChecked = true
		d.branchPushed = msg.Pushed
		d.pushing = false
		return d, nil

	case CreatePRPushDoneMsg:
		d.pushing = false
		if msg.Err != nil {
			d.errMsg = "Push failed: " + msg.Err.Error()
			return d, nil
		}
		d.branchPushed = true
		return d, nil

	case tea.KeyMsg:
		if d.basePicking {
			return d.handleBranchPickerKey(msg)
		}
		return d.handleKey(msg)
	}

	return d, nil
}

func (d CreatePR) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		if d.editing {
			d.editing = false
			d.bodyInput.Blur()
			return d, nil
		}
		return d, func() tea.Msg { return CreatePRCancelMsg{} }

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
		return d.submit()

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+g"))):
		if d.aiGenerating {
			return d, nil
		}
		d.aiGenerating = true
		d.errMsg = ""
		base := d.baseBranch
		return d, func() tea.Msg {
			return CreatePRRequestAIMsg{BaseBranch: base}
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+d"))):
		d.draft = !d.draft
		return d, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+p"))):
		if d.branchPushed || d.pushing {
			return d, nil
		}
		d.pushing = true
		d.errMsg = ""
		return d, func() tea.Msg { return CreatePRPushRequestMsg{} }

	case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
		return d.nextField()

	case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
		return d.prevField()

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		switch d.field {
		case prFieldBase:
			// Open inline branch picker
			d.basePicking = true
			d.baseCursor = 0
			d.baseFilter = ""
			// Place cursor on current base branch
			for i, b := range d.filteredBranches() {
				if b == d.baseBranch {
					d.baseCursor = i
					break
				}
			}
			return d, nil
		case prFieldTitle:
			// Enter in title = submit
			return d.submit()
		}
		// In body field, enter inserts newline — handled by textarea below
	}

	// Route to focused field
	var cmd tea.Cmd
	switch d.field {
	case prFieldTitle:
		d.titleInput, cmd = d.titleInput.Update(msg)
	case prFieldBody:
		d.bodyInput, cmd = d.bodyInput.Update(msg)
	}
	return d, cmd
}

func (d CreatePR) handleBranchPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := d.filteredBranches()

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		d.basePicking = false
		d.baseFilter = ""
		return d, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if len(filtered) > 0 && d.baseCursor < len(filtered) {
			newBase := filtered[d.baseCursor]
			if newBase != d.baseBranch {
				d.baseBranch = newBase
				d.statsLoading = true
				d.basePicking = false
				d.baseFilter = ""
				base := d.baseBranch
				return d, func() tea.Msg {
					return CreatePRRefreshStatsMsg{BaseBranch: base}
				}
			}
		}
		d.basePicking = false
		d.baseFilter = ""
		return d, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if d.baseCursor > 0 {
			d.baseCursor--
		}
		return d, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if d.baseCursor < len(filtered)-1 {
			d.baseCursor++
		}
		return d, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))):
		if d.baseFilter != "" {
			d.baseFilter = d.baseFilter[:len(d.baseFilter)-1]
			d.baseCursor = 0
		}
		return d, nil

	default:
		// Type-to-filter: append printable characters
		str := msg.String()
		if len(str) == 1 && str[0] >= 32 && str[0] <= 126 {
			d.baseFilter += str
			d.baseCursor = 0
		}
		return d, nil
	}
}

func (d CreatePR) submit() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(d.titleInput.Value())
	if title == "" {
		d.errMsg = "Title is required"
		return d, nil
	}
	return d, func() tea.Msg {
		return CreatePRSubmitMsg{
			Title: title,
			Body:  strings.TrimSpace(d.bodyInput.Value()),
			Head:  d.headBranch,
			Base:  d.baseBranch,
			Draft: d.draft,
		}
	}
}

func (d CreatePR) nextField() (CreatePR, tea.Cmd) {
	switch d.field {
	case prFieldBase:
		d.field = prFieldTitle
		d.titleInput.Focus()
		return d, textinput.Blink
	case prFieldTitle:
		d.field = prFieldBody
		d.titleInput.Blur()
		d.bodyInput.Focus()
		d.editing = true
		return d, nil
	case prFieldBody:
		d.field = prFieldBase
		d.editing = false
		d.bodyInput.Blur()
		return d, nil
	}
	return d, nil
}

func (d CreatePR) prevField() (CreatePR, tea.Cmd) {
	switch d.field {
	case prFieldBase:
		d.field = prFieldBody
		d.bodyInput.Focus()
		d.editing = true
		return d, nil
	case prFieldTitle:
		d.field = prFieldBase
		d.titleInput.Blur()
		return d, nil
	case prFieldBody:
		d.field = prFieldTitle
		d.editing = false
		d.bodyInput.Blur()
		d.titleInput.Focus()
		return d, textinput.Blink
	}
	return d, nil
}

// filteredBranches returns the remote branch list filtered by the current
// baseFilter. The head branch is excluded.
func (d CreatePR) filteredBranches() []string {
	filter := strings.ToLower(d.baseFilter)
	var out []string
	for _, b := range d.remoteBranches {
		if b == d.headBranch {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(b), filter) {
			continue
		}
		out = append(out, b)
	}
	return out
}

func (d CreatePR) View() string {
	return d.Base.Render(d.buildContentLines())
}

func (d CreatePR) buildContentLines() []string {
	th := theme.Active
	var lines []string

	labelStyle := lipgloss.NewStyle().Foreground(th.Subtext0).Background(th.Surface0)
	branchStyle := lipgloss.NewStyle().Foreground(th.Sapphire).Background(th.Surface0).Bold(true)
	arrowStyle := lipgloss.NewStyle().Foreground(th.Overlay0).Background(th.Surface0)
	dimStyle := lipgloss.NewStyle().Foreground(th.Overlay0).Background(th.Surface0)

	// -- Branch section -----------------------------------------------------

	// From (head branch — read-only)
	fromLabel := labelStyle.Render("  From   ")
	fromBranch := branchStyle.Render(d.headBranch)
	lines = append(lines, fromLabel+fromBranch)

	// Arrow
	lines = append(lines, arrowStyle.Render("    ↓"))

	// Into (base branch — selectable)
	intoLabelText := "  Into   "
	intoLabelFg := th.Subtext0
	if d.field == prFieldBase {
		intoLabelText = "▸ Into   "
		intoLabelFg = th.Blue
	}
	intoLabel := lipgloss.NewStyle().Foreground(intoLabelFg).Background(th.Surface0).Render(intoLabelText)
	baseName := lipgloss.NewStyle().Foreground(th.Green).Background(th.Surface0).Bold(true).Render(d.baseBranch)
	indicator := dimStyle.Render(" ▾")
	lines = append(lines, intoLabel+baseName+indicator)

	// Inline branch picker (shown when basePicking is true)
	if d.basePicking {
		lines = append(lines, d.buildBranchPickerLines()...)
	}

	lines = append(lines, "")

	// -- Stats line ---------------------------------------------------------
	if d.statsLoading {
		lines = append(lines, dimStyle.Render("  Loading stats..."))
	} else if d.commitCount > 0 || d.filesChanged > 0 {
		commitWord := "commits"
		if d.commitCount == 1 {
			commitWord = "commit"
		}
		fileWord := "files changed"
		if d.filesChanged == 1 {
			fileWord = "file changed"
		}

		statsStr := fmt.Sprintf("  %d %s · %d %s",
			d.commitCount, commitWord, d.filesChanged, fileWord)

		addStr := lipgloss.NewStyle().Foreground(th.Green).Background(th.Surface0).
			Render(fmt.Sprintf("+%d", d.additions))
		delStr := lipgloss.NewStyle().Foreground(th.Red).Background(th.Surface0).
			Render(fmt.Sprintf("-%d", d.deletions))
		lines = append(lines, dimStyle.Render(statsStr)+" "+addStr+" "+delStr)
	} else {
		lines = append(lines, dimStyle.Render("  No changes between branches"))
	}

	// -- Push warning -------------------------------------------------------
	if d.pushChecked && !d.branchPushed {
		lines = append(lines, "")
		if d.pushing {
			lines = append(lines,
				lipgloss.NewStyle().Foreground(th.Blue).Background(th.Surface0).
					Render("  ⟳ Pushing branch to remote..."))
		} else {
			lines = append(lines,
				lipgloss.NewStyle().Foreground(th.Yellow).Background(th.Surface0).
					Render("  ⚠ Branch not pushed to remote  ctrl+p to push"))
		}
	}

	lines = append(lines, "")

	// -- Draft badge --------------------------------------------------------
	if d.draft {
		draftBadge := lipgloss.NewStyle().
			Foreground(th.Base).Background(th.Overlay0).
			Bold(true).Padding(0, 1).
			Render("DRAFT")
		lines = append(lines, "  "+draftBadge)
		lines = append(lines, "")
	}

	// -- Title field --------------------------------------------------------
	titleLabel := "  Title"
	titleLabelFg := th.Subtext0
	if d.field == prFieldTitle {
		titleLabel = "▸ Title"
		titleLabelFg = th.Blue
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(titleLabelFg).Background(th.Surface0).Render(titleLabel))
	lines = append(lines, FlattenLines(d.titleInput.View())...)
	lines = append(lines, "")

	// -- Body field ---------------------------------------------------------
	bodyLabel := "  Description"
	bodyLabelFg := th.Subtext0
	if d.field == prFieldBody {
		bodyLabel = "▸ Description"
		bodyLabelFg = th.Blue
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(bodyLabelFg).Background(th.Surface0).Render(bodyLabel))
	lines = append(lines, FlattenLines(d.bodyInput.View())...)

	// -- AI generating indicator --------------------------------------------
	if d.aiGenerating {
		lines = append(lines, "")
		lines = append(lines,
			lipgloss.NewStyle().Foreground(th.Blue).Background(th.Surface0).
				Render("  ⟳ Generating AI description..."))
	}

	// -- Error message ------------------------------------------------------
	if d.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines,
			lipgloss.NewStyle().Foreground(th.Red).Background(th.Surface0).
				Render("  "+d.errMsg))
	}

	return lines
}

// buildBranchPickerLines renders the inline branch picker as content lines.
func (d CreatePR) buildBranchPickerLines() []string {
	th := theme.Active
	filtered := d.filteredBranches()

	var lines []string

	// Filter input
	if d.baseFilter != "" {
		filterLine := lipgloss.NewStyle().Foreground(th.Blue).Background(th.Surface0).
			Render("    / " + d.baseFilter + "▏")
		lines = append(lines, filterLine)
	}

	// Show max 8 branches at a time
	maxVisible := 8
	if len(filtered) < maxVisible {
		maxVisible = len(filtered)
	}

	// Calculate scroll window
	start := 0
	if d.baseCursor >= maxVisible {
		start = d.baseCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(filtered) {
		end = len(filtered)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		b := filtered[i]
		selected := i == d.baseCursor
		isActive := b == d.baseBranch

		prefix := "      "
		fg := th.Text
		bg := th.Surface0
		if isActive {
			prefix = "    ✓ "
		}
		if selected {
			fg = th.Base
			bg = th.Blue
			if isActive {
				prefix = "    ✓ "
			} else {
				prefix = "    > "
			}
		}

		style := lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(selected)
		lines = append(lines, style.Render(prefix+b))
	}

	if len(filtered) == 0 {
		lines = append(lines,
			lipgloss.NewStyle().Foreground(th.Overlay0).Background(th.Surface0).
				Render("      No matching branches"))
	}

	// Scroll indicators
	if len(filtered) > maxVisible {
		count := lipgloss.NewStyle().Foreground(th.Overlay0).Background(th.Surface0).
			Render(fmt.Sprintf("      %d of %d branches", maxVisible, len(filtered)))
		lines = append(lines, count)
	}

	return lines
}

var _ tea.Model = CreatePR{}
