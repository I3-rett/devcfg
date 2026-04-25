package steps

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type shellOption int

const (
	shellKeep shellOption = iota
	shellZsh
	shellBash
)

var basicAliases = `
# devcfg aliases
alias ll='ls -lah'
alias la='ls -A'
alias gs='git status'
alias gp='git pull'
alias gc='git commit'
alias gco='git checkout'
alias ..='cd ..'
alias ...='cd ../..'
`

type shellInitMsg struct {
	currentShell string
}

type shellApplyDoneMsg struct {
	err error
}

// ShellModel manages the shell setup step.
type ShellModel struct {
	currentShell  string
	selectedShell shellOption
	addAliases    bool
	focusIdx      int // 0=keep, 1=zsh, 2=bash, 3=aliases toggle, 4=continue
	done          bool
	loaded        bool
	statusMsg     string
	statusErr     bool
}

// NewShellModel creates a new ShellModel.
func NewShellModel() *ShellModel {
	return &ShellModel{}
}

// Title returns the display name of this step.
func (m *ShellModel) Title() string { return "Shell Setup" }

// IsDone reports whether the shell setup step has been completed.
func (m *ShellModel) IsDone() bool { return m.done }

// CanQuit always returns true for the Shell step.
func (m *ShellModel) CanQuit() bool { return true }

// CanSwitchTabs always returns true for the Shell step.
func (m *ShellModel) CanSwitchTabs() bool { return true }

// Init detects the current shell asynchronously.
func (m *ShellModel) Init() tea.Cmd {
	return func() tea.Msg {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "unknown"
		}
		return shellInitMsg{currentShell: shell}
	}
}

// Update handles messages for the shell setup step.
func (m *ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case shellInitMsg:
		m.currentShell = msg.currentShell
		m.loaded = true

	case shellApplyDoneMsg:
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = "Shell setup applied successfully."
			m.done = true
		}

	case tea.KeyMsg:
		if !m.loaded {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.focusIdx > 0 {
				m.focusIdx--
			}
		case "down", "j":
			if m.focusIdx < 4 {
				m.focusIdx++
			}
		case " ":
			m.handleSelect()
		case "enter":
			if m.focusIdx == 4 {
				return m, m.applyShellSetup()
			}
			m.handleSelect()
		}
	}
	return m, nil
}

func (m *ShellModel) handleSelect() {
	switch m.focusIdx {
	case 0:
		m.selectedShell = shellKeep
	case 1:
		m.selectedShell = shellZsh
	case 2:
		m.selectedShell = shellBash
	case 3:
		m.addAliases = !m.addAliases
	}
}

func (m *ShellModel) applyShellSetup() tea.Cmd {
	selected := m.selectedShell
	addAliases := m.addAliases
	current := m.currentShell

	return func() tea.Msg {
		var errs []string

		// Change shell if needed
		if selected != shellKeep {
			newShell := "/bin/bash"
			if selected == shellZsh {
				newShell = "/bin/zsh"
				if zshPath := findShell("zsh"); zshPath != "" {
					newShell = zshPath
				}
			}
			u, err := user.Current()
			if err != nil {
				return shellApplyDoneMsg{err: fmt.Errorf("get current user: %w", err)}
			}
			res := executor.Execute([]string{"chsh", "-s", newShell, u.Username})
			if res.Err != nil {
				errs = append(errs, "chsh: "+res.Err.Error())
			}
		}

		// Add aliases
		if addAliases {
			rcFile := rcFileFor(current, selected)
			home, err := os.UserHomeDir()
			if err != nil {
				errs = append(errs, "resolve home dir: "+err.Error())
				return shellApplyDoneMsg{err: fmt.Errorf("%s", strings.Join(errs, "; "))}
			}
			rcPath := filepath.Join(home, rcFile)

			// Check if already added (ignore not-exist errors; treat other errors as empty)
			existing, readErr := os.ReadFile(rcPath) //nolint:gosec
			if readErr != nil && !os.IsNotExist(readErr) {
				errs = append(errs, "read rc: "+readErr.Error())
				return shellApplyDoneMsg{err: fmt.Errorf("%s", strings.Join(errs, "; "))}
			}
			if !strings.Contains(string(existing), "# devcfg aliases") {
				f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec
				if err != nil {
					errs = append(errs, "open rc: "+err.Error())
				} else {
					_, writeErr := fmt.Fprint(f, basicAliases)
					_ = f.Close()
					if writeErr != nil {
						errs = append(errs, "write aliases: "+writeErr.Error())
					}
				}
			}
		}

		if len(errs) > 0 {
			return shellApplyDoneMsg{err: fmt.Errorf("%s", strings.Join(errs, "; "))}
		}
		return shellApplyDoneMsg{}
	}
}

func rcFileFor(current string, selected shellOption) string {
	switch selected {
	case shellZsh:
		return ".zshrc"
	case shellBash:
		return ".bashrc"
	}
	if strings.Contains(current, "zsh") {
		return ".zshrc"
	}
	return ".bashrc"
}

func findShell(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// View renders the shell setup step.
func (m *ShellModel) View() string {
	var sb strings.Builder

	if !m.loaded {
		sb.WriteString(tuistyles.StatusStyle.Render("Loading shell information...") + "\n")
		return sb.String()
	}

	if m.done {
		sb.WriteString(tuistyles.SuccessStyle.Render("✓ "+m.statusMsg) + "\n")
		return sb.String()
	}

	sb.WriteString(tuistyles.StatusStyle.Render(fmt.Sprintf("Current shell: %s", m.currentShell)) + "\n\n")
	sb.WriteString(tuistyles.ItemStyle.Render("Select shell:") + "\n\n")

	options := []struct {
		label string
		val   shellOption
		idx   int
	}{
		{"Keep current shell", shellKeep, 0},
		{"Switch to zsh", shellZsh, 1},
		{"Switch to bash", shellBash, 2},
	}

	for _, opt := range options {
		prefix := "  "
		style := tuistyles.ItemStyle
		radio := "( )"
		if m.selectedShell == opt.val {
			radio = "(●)"
			style = tuistyles.CheckedItemStyle
		}
		if m.focusIdx == opt.idx {
			prefix = "▶ "
			style = tuistyles.SelectedItemStyle
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s%s %s", prefix, radio, opt.label)) + "\n")
	}

	sb.WriteString("\n")

	// Aliases toggle
	aliasStyle := tuistyles.ItemStyle
	aliasPrefix := "  "
	aliasCheck := "[ ]"
	if m.addAliases {
		aliasCheck = "[✓]"
		aliasStyle = tuistyles.CheckedItemStyle
	}
	if m.focusIdx == 3 {
		aliasPrefix = "▶ "
		aliasStyle = tuistyles.SelectedItemStyle
	}
	sb.WriteString(aliasStyle.Render(fmt.Sprintf("%s%s Add basic aliases (ll, la, gs, gp, gc...)", aliasPrefix, aliasCheck)) + "\n\n")

	// Continue button
	btnStyle := tuistyles.ButtonStyle
	if m.focusIdx == 4 {
		btnStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(btnStyle.Render("  Continue  ") + "\n")

	if m.statusMsg != "" {
		sb.WriteString("\n")
		if m.statusErr {
			sb.WriteString(tuistyles.ErrorStyle.Render(m.statusMsg) + "\n")
		} else {
			sb.WriteString(tuistyles.SuccessStyle.Render(m.statusMsg) + "\n")
		}
	}

	return sb.String()
}
