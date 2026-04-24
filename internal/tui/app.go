package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/system"
	"github.com/I3-rett/devcfg/internal/tui/steps"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type stepModel interface {
	tea.Model
	IsDone() bool
	Title() string
	// CanQuit returns false when the step wants to intercept quit signals
	// (e.g. while an installation is running) and handle them itself.
	CanQuit() bool
}

// AppModel is the root Bubble Tea model.
type AppModel struct {
	stepsList []stepModel
	current   int
	sysInfo   system.Info
	width     int
	height    int
	allDone   bool
}

func newApp() *AppModel {
	sys := system.Detect()
	stepsList := []stepModel{
		steps.NewToolsModel(sys),
		steps.NewGitModel(),
		steps.NewDockerModel(),
		steps.NewShellModel(),
	}
	return &AppModel{
		stepsList: stepsList,
		sysInfo:   sys,
	}
}

func (a *AppModel) Init() tea.Cmd {
	if len(a.stepsList) == 0 {
		return nil
	}
	return a.stepsList[0].Init()
}

func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Forward size to the current step so it can adapt its layout.
		if !a.allDone && a.current < len(a.stepsList) {
			updated, cmd := a.stepsList[a.current].Update(msg)
			a.stepsList[a.current] = updated.(stepModel)
			return a, cmd
		}
		return a, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Only quit immediately when the current step allows it.
			// When CanQuit() is false the key is forwarded to the step so it
			// can show its own confirmation (e.g. abort running installation).
			if a.allDone || a.stepsList[a.current].CanQuit() {
				return a, tea.Quit
			}
		}
	}

	if a.allDone {
		return a, nil
	}

	current := a.stepsList[a.current]
	updated, cmd := current.Update(msg)
	a.stepsList[a.current] = updated.(stepModel)

	if a.stepsList[a.current].IsDone() {
		a.current++
		if a.current >= len(a.stepsList) {
			a.allDone = true
			return a, nil
		}
		initCmd := a.stepsList[a.current].Init()
		return a, tea.Batch(cmd, initCmd)
	}

	return a, cmd
}

func (a *AppModel) View() string {
	var sb strings.Builder

	// Header
	sb.WriteString(tuistyles.TitleStyle.Render("⚙  devcfg — Environment Configurator") + "\n")
	sb.WriteString(tuistyles.DividerStyle.Render(strings.Repeat("─", 50)) + "\n\n")

	if a.allDone {
		sb.WriteString(tuistyles.SuccessStyle.Render("🎉 All steps complete! Your environment is configured.") + "\n")
		sb.WriteString(tuistyles.StatusStyle.Render("Press q or Ctrl+C to exit.") + "\n")
		return sb.String()
	}

	// Step indicator
	total := len(a.stepsList)
	currentTitle := a.stepsList[a.current].Title()
	indicator := fmt.Sprintf("Step %d/%d — %s", a.current+1, total, currentTitle)
	sb.WriteString(tuistyles.StepIndicatorStyle.Render(indicator) + "\n")

	// Mini breadcrumb
	crumbs := make([]string, total)
	for i, s := range a.stepsList {
		if i < a.current {
			crumbs[i] = tuistyles.SuccessStyle.Render("✓ " + s.Title())
		} else if i == a.current {
			crumbs[i] = tuistyles.SelectedItemStyle.Render("▶ " + s.Title())
		} else {
			crumbs[i] = tuistyles.StatusStyle.Render("○ " + s.Title())
		}
	}
	sb.WriteString(strings.Join(crumbs, "  ") + "\n\n")

	// Current step content
	sb.WriteString(a.stepsList[a.current].View())

	sb.WriteString("\n" + tuistyles.StatusStyle.Render("q/Ctrl+C: quit  ↑/↓: navigate  SPACE/ENTER: select") + "\n")

	return sb.String()
}

// Run starts the TUI application.
func Run() error {
	app := newApp()
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
