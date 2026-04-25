package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/system"
	"github.com/I3-rett/devcfg/internal/tui/steps"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type stepModel interface {
	tea.Model
	IsDone() bool
	Title() string
	// CanQuit returns false when the step must intercept Ctrl+C itself
	// (e.g. when a PTY is focused and the keystroke should go to the process).
	CanQuit() bool
	// CanSwitchTabs returns false when the step is consuming left/right arrow
	// keys for its own navigation (text inputs, popups, etc.).
	CanSwitchTabs() bool
}

// AppModel is the root Bubble Tea model.
type AppModel struct {
	tabs        []stepModel
	current     int
	initialized []bool
	width       int
	height      int
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom(" ", " ", " ")

	tabActive = lipgloss.NewStyle().
			Border(activeTabBorder, true).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Bold(true).
			Padding(0, 1)

	tabInactive = lipgloss.NewStyle().
			Border(inactiveTabBorder, true).
			BorderForeground(lipgloss.Color("#6B7280")).
			Foreground(lipgloss.Color("#6B7280")).
			Padding(0, 1)

	tabGapStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#7C3AED"))
)

func newApp() *AppModel {
	sys := system.Detect()
	tabs := []stepModel{
		steps.NewToolsModel(sys),
		steps.NewGitModel(),
		steps.NewDockerModel(),
		steps.NewShellModel(),
	}
	return &AppModel{
		tabs:        tabs,
		initialized: make([]bool, len(tabs)),
	}
}

// Init initialises the first tab.
func (a *AppModel) Init() tea.Cmd {
	a.initialized[0] = true
	return a.tabs[0].Init()
}

// Update handles global navigation and forwards messages to the active tab.
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward to all tabs so layout adapts even for non-active ones.
		var cmds []tea.Cmd
		for i, tab := range a.tabs {
			updated, cmd := tab.Update(msg)
			a.tabs[i] = updated.(stepModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			// Tab bar occupies rows 0-2 (top border, content, bottom border).
			if msg.Y <= 2 {
				x := 0
				for i, tab := range a.tabs {
					label := tab.Title()
					if tab.IsDone() {
						label = "✓ " + label
					}
					var w int
					if i == a.current {
						w = lipgloss.Width(tabActive.Render(label))
					} else {
						w = lipgloss.Width(tabInactive.Render(label))
					}
					if msg.X >= x && msg.X < x+w {
						if i != a.current {
							a.current = i
							return a, a.ensureInit(i)
						}
						return a, nil
					}
					x += w
				}
				return a, nil
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if a.tabs[a.current].CanQuit() {
				return a, tea.Quit
			}
		case "q":
			if a.tabs[a.current].CanQuit() && a.tabs[a.current].CanSwitchTabs() {
				return a, tea.Quit
			}
		case "left":
			if a.tabs[a.current].CanSwitchTabs() && a.current > 0 {
				a.current--
				return a, a.ensureInit(a.current)
			}
		case "right":
			if a.tabs[a.current].CanSwitchTabs() && a.current < len(a.tabs)-1 {
				a.current++
				return a, a.ensureInit(a.current)
			}
		}
	}

	updated, cmd := a.tabs[a.current].Update(msg)
	a.tabs[a.current] = updated.(stepModel)
	return a, cmd
}

// ensureInit calls Init() on a tab the first time it is visited.
func (a *AppModel) ensureInit(idx int) tea.Cmd {
	if a.initialized[idx] {
		return nil
	}
	a.initialized[idx] = true
	return a.tabs[idx].Init()
}

// View renders the tab bar followed by the active tab's content.
func (a *AppModel) View() string {
	var sb strings.Builder

	// Render each tab
	tabViews := make([]string, len(a.tabs))
	for i, tab := range a.tabs {
		label := tab.Title()
		if tab.IsDone() {
			label = "✓ " + label
		}
		if i == a.current {
			tabViews[i] = tabActive.Render(label)
		} else {
			tabViews[i] = tabInactive.Render(label)
		}
	}

	// Join tabs, then extend the bottom border to fill the rest of the line.
	row := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	gap := tabGapStyle.Render(strings.Repeat(" ", max(0, a.width-lipgloss.Width(row)-2)))
	tabBar := lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
	sb.WriteString(tabBar + "\n")

	// Active tab content
	sb.WriteString(a.tabs[a.current].View())

	// Footer hints
	var hints []string
	if a.tabs[a.current].CanSwitchTabs() {
		hints = append(hints, "←/→: switch tabs")
	}
	hints = append(hints, "q/Ctrl+C: quit")
	sb.WriteString("\n" + tuistyles.StatusStyle.Render(fmt.Sprintf("%s", strings.Join(hints, "  "))) + "\n")

	return sb.String()
}

// Run starts the TUI application.
func Run() error {
	app := newApp()
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
