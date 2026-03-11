package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/heesungjang/kommit/internal/tui/styles"
	"github.com/heesungjang/kommit/internal/tui/theme"
)

// TabItem represents a single tab in the tab bar.
type TabItem struct {
	Key   string // Number key shortcut (e.g., "1")
	Label string // Display label (e.g., "Status")
}

// DefaultTabs returns the standard tab configuration.
func DefaultTabs() []TabItem {
	return []TabItem{
		{Key: "1", Label: "Status"},
		{Key: "2", Label: "Log"},
		{Key: "3", Label: "Branches"},
		{Key: "4", Label: "Remotes"},
		{Key: "5", Label: "Stash"},
		{Key: "6", Label: "PRs"},
		{Key: "7", Label: "CI"},
	}
}

// TabChangedMsg is sent when the active tab changes.
type TabChangedMsg struct {
	Index int
}

// Tabs is a tab bar component.
type Tabs struct {
	items  []TabItem
	active int
	width  int
}

// NewTabs creates a new Tabs component with default tabs.
func NewTabs() Tabs {
	return Tabs{
		items: DefaultTabs(),
		width: 80,
	}
}

// NewTabsWithItems creates a new Tabs component with custom tab items.
func NewTabsWithItems(items []TabItem) Tabs {
	return Tabs{
		items: items,
		width: 80,
	}
}

// SetActive sets the active tab index.
func (t Tabs) SetActive(index int) Tabs {
	if index >= 0 && index < len(t.items) {
		t.active = index
	}
	return t
}

// Active returns the current active tab index.
func (t Tabs) Active() int {
	return t.active
}

// SetSize sets the width of the tab bar.
func (t Tabs) SetSize(width int) Tabs {
	t.width = width
	return t
}

// ActiveLabel returns the label of the active tab.
func (t Tabs) ActiveLabel() string {
	if t.active >= 0 && t.active < len(t.items) {
		return t.items[t.active].Label
	}
	return ""
}

// Init implements tea.Model.
func (t Tabs) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (t Tabs) Update(msg tea.Msg) (Tabs, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(msg, tabKeys.Tab1):
			if len(t.items) > 0 {
				t.active = 0
				return t, sendTabChanged(0)
			}
		case key.Matches(msg, tabKeys.Tab2):
			if len(t.items) > 1 {
				t.active = 1
				return t, sendTabChanged(1)
			}
		case key.Matches(msg, tabKeys.Tab3):
			if len(t.items) > 2 {
				t.active = 2
				return t, sendTabChanged(2)
			}
		case key.Matches(msg, tabKeys.Tab4):
			if len(t.items) > 3 {
				t.active = 3
				return t, sendTabChanged(3)
			}
		case key.Matches(msg, tabKeys.Tab5):
			if len(t.items) > 4 {
				t.active = 4
				return t, sendTabChanged(4)
			}
		case key.Matches(msg, tabKeys.Tab6):
			if len(t.items) > 5 {
				t.active = 5
				return t, sendTabChanged(5)
			}
		case key.Matches(msg, tabKeys.Tab7):
			if len(t.items) > 6 {
				t.active = 6
				return t, sendTabChanged(6)
			}
		}
	}

	return t, nil
}

// View implements tea.Model.
func (t Tabs) View() string {
	th := theme.Active

	tabParts := make([]string, 0, len(t.items))
	for i, item := range t.items {
		isActive := i == t.active
		label := fmt.Sprintf("[%s]%s", item.Key, item.Label)

		tabStyle := styles.TabStyle(isActive)
		tabParts = append(tabParts, tabStyle.Render(label))
	}

	sep := lipgloss.NewStyle().Background(th.Mantle).Render(" ")
	tabLine := strings.Join(tabParts, sep)

	barStyle := lipgloss.NewStyle().
		Width(t.width).
		Background(th.Mantle).
		Padding(0, 1)

	return barStyle.Render(tabLine)
}

func sendTabChanged(index int) tea.Cmd {
	return func() tea.Msg {
		return TabChangedMsg{Index: index}
	}
}

// tabKeyMap defines keybindings for the tab bar.
type tabKeyMap struct {
	Tab1 key.Binding
	Tab2 key.Binding
	Tab3 key.Binding
	Tab4 key.Binding
	Tab5 key.Binding
	Tab6 key.Binding
	Tab7 key.Binding
}

var tabKeys = tabKeyMap{
	Tab1: key.NewBinding(key.WithKeys("1")),
	Tab2: key.NewBinding(key.WithKeys("2")),
	Tab3: key.NewBinding(key.WithKeys("3")),
	Tab4: key.NewBinding(key.WithKeys("4")),
	Tab5: key.NewBinding(key.WithKeys("5")),
	Tab6: key.NewBinding(key.WithKeys("6")),
	Tab7: key.NewBinding(key.WithKeys("7")),
}

// Count returns the number of tabs.
func (t Tabs) Count() int {
	return len(t.items)
}

// Items returns the tab items.
func (t Tabs) Items() []TabItem {
	return t.items
}
