package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/resolver"
	"github.com/I3-rett/devcfg/internal/system"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type installResultMsg struct {
	name   string
	output string
	err    error
}

type ToolsModel struct {
	tools    []registry.Tool
	checked  []bool
	cursor   int
	sysInfo  system.Info
	done     bool
	running  bool
	results  []string
	errors   []string
	msgLines []string
}

func NewToolsModel(sysInfo system.Info) *ToolsModel {
	tools := registry.List()
	return &ToolsModel{
		tools:   tools,
		checked: make([]bool, len(tools)),
		sysInfo: sysInfo,
	}
}

func (m *ToolsModel) Title() string { return "Tools Installation" }
func (m *ToolsModel) IsDone() bool  { return m.done }

func (m *ToolsModel) Init() tea.Cmd { return nil }

func (m *ToolsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.running {
		switch msg := msg.(type) {
		case installResultMsg:
			if msg.err != nil {
				m.errors = append(m.errors, fmt.Sprintf("✗ %s: %s", msg.name, msg.err.Error()))
			} else {
				m.results = append(m.results, fmt.Sprintf("✓ %s installed", msg.name))
			}
			m.msgLines = append(m.msgLines, msg.output)
			// Check if installation is complete
			total := m.countSelected()
			if len(m.results)+len(m.errors) >= total {
				m.running = false
				m.done = true
			}
		}
		return m, nil
	}

	// continueIdx is the index of the Continue button (one past the last tool).
	continueIdx := len(m.tools)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < continueIdx {
				m.cursor++
			}
		case " ":
			if m.cursor < continueIdx {
				m.checked[m.cursor] = !m.checked[m.cursor]
			}
		case "enter":
			if m.cursor < continueIdx {
				m.checked[m.cursor] = !m.checked[m.cursor]
			} else {
				// Continue button
				return m, m.startInstallation()
			}
		}
	}
	return m, nil
}

func (m *ToolsModel) countSelected() int {
	n := 0
	for _, c := range m.checked {
		if c {
			n++
		}
	}
	return n
}

func (m *ToolsModel) startInstallation() tea.Cmd {
	selected := []registry.Tool{}
	for i, c := range m.checked {
		if c {
			selected = append(selected, m.tools[i])
		}
	}
	if len(selected) == 0 {
		m.done = true
		return nil
	}
	m.running = true
	cmds := make([]tea.Cmd, len(selected))
	for i, tool := range selected {
		t := tool
		cmds[i] = func() tea.Msg {
			args, err := resolver.Resolve(t, m.sysInfo)
			if err != nil {
				return installResultMsg{name: t.Name, err: err}
			}
			res := executor.Execute(args)
			return installResultMsg{name: t.Name, output: res.Output, err: res.Err}
		}
	}
	return tea.Batch(cmds...)
}

func (m *ToolsModel) View() string {
	var sb strings.Builder

	if m.running {
		sb.WriteString(tuistyles.StatusStyle.Render("Installing selected tools...") + "\n\n")
		for _, r := range m.results {
			sb.WriteString(tuistyles.SuccessStyle.Render(r) + "\n")
		}
		for _, e := range m.errors {
			sb.WriteString(tuistyles.ErrorStyle.Render(e) + "\n")
		}
		return sb.String()
	}

	if m.done {
		sb.WriteString(tuistyles.SuccessStyle.Render("Installation complete!") + "\n\n")
		for _, r := range m.results {
			sb.WriteString(tuistyles.SuccessStyle.Render(r) + "\n")
		}
		for _, e := range m.errors {
			sb.WriteString(tuistyles.ErrorStyle.Render(e) + "\n")
		}
		return sb.String()
	}

	sb.WriteString(tuistyles.StatusStyle.Render("Select tools to install (SPACE/ENTER to toggle):") + "\n\n")

	for i, tool := range m.tools {
		cursor := "  "
		if m.cursor == i {
			cursor = tuistyles.SelectedItemStyle.Render("▶ ")
		}

		checkbox := "[ ]"
		style := tuistyles.ItemStyle
		if m.checked[i] {
			checkbox = "[✓]"
			style = tuistyles.CheckedItemStyle
		}
		if m.cursor == i {
			style = tuistyles.SelectedItemStyle
		}

		line := fmt.Sprintf("%s%s %s", cursor, checkbox, style.Render(fmt.Sprintf("%-12s %s", tool.Name, tool.Description)))
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n")

	btnIdx := len(m.tools)
	btnStyle := tuistyles.ButtonStyle
	if m.cursor == btnIdx {
		btnStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(btnStyle.Render("  Continue  ") + "\n")

	return sb.String()
}
