package steps

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

// GitModel manages the Git configuration step.
type GitModel struct {
	nameInput  textinput.Model
	emailInput textinput.Model
	gpgSigning bool
	focusIdx   int // 0=name, 1=email, 2=gpg toggle, 3=continue
	done       bool
	status     string
	statusErr  bool
}

// NewGitModel creates a new GitModel with default text inputs.
func NewGitModel() *GitModel {
	name := textinput.New()
	name.Placeholder = "Your Name"
	name.Focus()
	name.Width = 40

	email := textinput.New()
	email.Placeholder = "your@email.com"
	email.Width = 40

	return &GitModel{
		nameInput:  name,
		emailInput: email,
	}
}

// Title returns the display name of this step.
func (m *GitModel) Title() string { return "Git Configuration" }

// IsDone reports whether the Git configuration step has been completed.
func (m *GitModel) IsDone() bool { return m.done }

// CanQuit always returns true for the Git step.
func (m *GitModel) CanQuit() bool { return true }

// CanSwitchTabs returns false when a text input is focused so that keystrokes
// (including 'q') are forwarded to the input rather than handled as global
// shortcuts.
func (m *GitModel) CanSwitchTabs() bool { return m.focusIdx > 1 }

type gitInitMsg struct {
	name  string
	email string
	gpg   bool
}

// Init reads existing git config and starts cursor blinking.
func (m *GitModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, func() tea.Msg {
		name := strings.TrimSpace(executor.Execute([]string{"git", "config", "--global", "user.name"}).Output)
		email := strings.TrimSpace(executor.Execute([]string{"git", "config", "--global", "user.email"}).Output)
		gpgRaw := strings.TrimSpace(executor.Execute([]string{"git", "config", "--global", "commit.gpgsign"}).Output)
		return gitInitMsg{name: name, email: email, gpg: gpgRaw == "true"}
	})
}

// Update handles messages for the Git configuration step.
func (m *GitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case gitInitMsg:
		m.nameInput.SetValue(msg.name)
		m.emailInput.SetValue(msg.email)
		m.gpgSigning = msg.gpg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focusIdx = (m.focusIdx + 1) % 4
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.focusIdx = (m.focusIdx + 3) % 4
			return m, m.updateFocus()
		case " ":
			if m.focusIdx == 2 {
				m.gpgSigning = !m.gpgSigning
			}
		case "enter":
			switch m.focusIdx {
			case 2:
				m.gpgSigning = !m.gpgSigning
			case 3:
				return m, m.applyGitConfig()
			default:
				m.focusIdx++
				return m, m.updateFocus()
			}
		}

	case gitConfigDoneMsg:
		m.status = msg.status
		m.statusErr = msg.err != nil
		if msg.err == nil {
			m.done = true
		}
	}

	var cmd tea.Cmd
	switch m.focusIdx {
	case 0:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case 1:
		m.emailInput, cmd = m.emailInput.Update(msg)
	}
	return m, cmd
}

func (m *GitModel) updateFocus() tea.Cmd {
	m.nameInput.Blur()
	m.emailInput.Blur()
	switch m.focusIdx {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.emailInput.Focus()
	}
	return textinput.Blink
}

type gitConfigDoneMsg struct {
	status string
	err    error
}

func (m *GitModel) applyGitConfig() tea.Cmd {
	name := m.nameInput.Value()
	email := m.emailInput.Value()
	gpg := m.gpgSigning

	return func() tea.Msg {
		var errs []string
		if name != "" {
			res := executor.Execute([]string{"git", "config", "--global", "user.name", name})
			if res.Err != nil {
				errs = append(errs, "name: "+res.Err.Error())
			}
		}
		if email != "" {
			res := executor.Execute([]string{"git", "config", "--global", "user.email", email})
			if res.Err != nil {
				errs = append(errs, "email: "+res.Err.Error())
			}
		}
		signingVal := "false"
		if gpg {
			signingVal = "true"
		}
		res := executor.Execute([]string{"git", "config", "--global", "commit.gpgsign", signingVal})
		if res.Err != nil {
			errs = append(errs, "gpgsign: "+res.Err.Error())
		}
		if len(errs) > 0 {
			return gitConfigDoneMsg{status: strings.Join(errs, "; "), err: fmt.Errorf("git config errors")}
		}
		return gitConfigDoneMsg{status: "Git configuration applied successfully."}
	}
}

// View renders the Git configuration step.
func (m *GitModel) View() string {
	var sb strings.Builder

	if m.done {
		sb.WriteString(tuistyles.SuccessStyle.Render("✓ "+m.status) + "\n")
		return sb.String()
	}

	sb.WriteString(tuistyles.StatusStyle.Render("Configure your global git identity:") + "\n\n")

	// Name field
	nameLabel := tuistyles.ItemStyle.Render("Name:  ")
	if m.focusIdx == 0 {
		nameLabel = tuistyles.SelectedItemStyle.Render("Name:  ")
	}
	sb.WriteString(nameLabel + m.nameInput.View() + "\n\n")

	// Email field
	emailLabel := tuistyles.ItemStyle.Render("Email: ")
	if m.focusIdx == 1 {
		emailLabel = tuistyles.SelectedItemStyle.Render("Email: ")
	}
	sb.WriteString(emailLabel + m.emailInput.View() + "\n\n")

	// GPG signing toggle
	gpgStr := "[ ] Enable GPG signing"
	gpgStyle := tuistyles.ItemStyle
	if m.gpgSigning {
		gpgStr = "[✓] Enable GPG signing"
		gpgStyle = tuistyles.CheckedItemStyle
	}
	if m.focusIdx == 2 {
		gpgStr = "▶ " + gpgStr
		gpgStyle = tuistyles.SelectedItemStyle
	} else {
		gpgStr = "  " + gpgStr
	}
	sb.WriteString(gpgStyle.Render(gpgStr) + "\n\n")

	// Continue button
	btnStyle := tuistyles.ButtonStyle
	if m.focusIdx == 3 {
		btnStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(btnStyle.Render("  Continue  ") + "\n")

	if m.status != "" {
		sb.WriteString("\n")
		if m.statusErr {
			sb.WriteString(tuistyles.ErrorStyle.Render(m.status) + "\n")
		} else {
			sb.WriteString(tuistyles.SuccessStyle.Render(m.status) + "\n")
		}
	}

	return sb.String()
}
