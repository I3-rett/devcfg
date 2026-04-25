package steps

import (
	"context"
	"fmt"
	"os/user"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type dockerStatus struct {
	installed    bool
	version      string
	inGroup      bool
	daemonActive bool
}

type dockerStatusMsg struct {
	status dockerStatus
}

type dockerActionDoneMsg struct {
	output string
	err    error
}

// DockerModel manages the Docker setup step.
type DockerModel struct {
	status    dockerStatus
	loaded    bool
	focusIdx  int // 0=addGroup, 1=continue
	done      bool
	actionOut string
	actionErr error
}

// NewDockerModel creates a new DockerModel.
func NewDockerModel() *DockerModel {
	return &DockerModel{}
}

// Title returns the display name of this step.
func (m *DockerModel) Title() string { return "Docker Setup" }

// IsDone reports whether the Docker setup step has been completed.
func (m *DockerModel) IsDone() bool { return m.done }

// CanQuit always returns true for the Docker step.
func (m *DockerModel) CanQuit() bool { return true }

// CanSwitchTabs always returns true for the Docker step.
func (m *DockerModel) CanSwitchTabs() bool { return true }

// Init triggers the initial Docker status check.
func (m *DockerModel) Init() tea.Cmd {
	return m.checkDocker()
}

func (m *DockerModel) checkDocker() tea.Cmd {
	return func() tea.Msg {
		var s dockerStatus

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res := executor.ExecuteWithContext(ctx, []string{"docker", "--version"}, nil)
		if res.Err == nil {
			s.installed = true
			s.version = strings.TrimSpace(res.Output)
		}

		if s.installed {
			u, err := user.Current()
			if err == nil {
				groupRes := executor.ExecuteWithContext(ctx, []string{"id", "-nG", u.Username}, nil)
				if groupRes.Err == nil {
					for _, g := range strings.Fields(groupRes.Output) {
						if g == "docker" {
							s.inGroup = true
							break
						}
					}
				}
			}

			daemonRes := executor.ExecuteWithContext(ctx, []string{"systemctl", "is-active", "docker"}, nil)
			s.daemonActive = strings.TrimSpace(daemonRes.Output) == "active"
		}

		return dockerStatusMsg{status: s}
	}
}

// Update handles messages for the Docker setup step.
func (m *DockerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dockerStatusMsg:
		m.status = msg.status
		m.loaded = true

	case dockerActionDoneMsg:
		m.actionOut = msg.output
		m.actionErr = msg.err

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
			maxIdx := 0
			if m.status.installed && !m.status.inGroup {
				maxIdx = 1
			}
			if m.focusIdx < maxIdx {
				m.focusIdx++
			}
		case "enter":
			if m.status.installed && !m.status.inGroup && m.focusIdx == 0 {
				return m, m.addToDockerGroup()
			}
			// Continue
			m.done = true
		}
	}
	return m, nil
}

func (m *DockerModel) addToDockerGroup() tea.Cmd {
	return func() tea.Msg {
		u, err := user.Current()
		if err != nil {
			return dockerActionDoneMsg{err: fmt.Errorf("get current user: %w", err)}
		}
		res := executor.Execute([]string{"sudo", "usermod", "-aG", "docker", u.Username})
		return dockerActionDoneMsg{output: res.Output, err: res.Err}
	}
}

// View renders the Docker setup step.
func (m *DockerModel) View() string {
	var sb strings.Builder

	if !m.loaded {
		sb.WriteString(tuistyles.StatusStyle.Render("Checking Docker installation...") + "\n")
		return sb.String()
	}

	if m.done {
		sb.WriteString(tuistyles.SuccessStyle.Render("✓ Docker setup complete.") + "\n")
		return sb.String()
	}

	// Docker status
	if m.status.installed {
		sb.WriteString(tuistyles.SuccessStyle.Render("✓ Docker installed: "+m.status.version) + "\n\n")

		if m.status.daemonActive {
			sb.WriteString(tuistyles.SuccessStyle.Render("✓ Docker daemon: active") + "\n")
		} else {
			sb.WriteString(tuistyles.WarningStyle.Render("⚠ Docker daemon: inactive") + "\n")
		}

		if m.status.inGroup {
			sb.WriteString(tuistyles.SuccessStyle.Render("✓ User is in the docker group") + "\n\n")
		} else {
			sb.WriteString(tuistyles.WarningStyle.Render("⚠ User is NOT in the docker group") + "\n\n")

			// Option to add to group
			optStyle := tuistyles.ItemStyle
			prefix := "  "
			if m.focusIdx == 0 {
				optStyle = tuistyles.SelectedItemStyle
				prefix = "▶ "
			}
			sb.WriteString(optStyle.Render(prefix+"Add current user to docker group (requires sudo)") + "\n\n")
		}
	} else {
		sb.WriteString(tuistyles.WarningStyle.Render("⚠ Docker is not installed.") + "\n")
		sb.WriteString(tuistyles.StatusStyle.Render("  Install Docker in the Tools step, then re-run devcfg.") + "\n\n")
	}

	if m.actionOut != "" {
		if m.actionErr != nil {
			sb.WriteString(tuistyles.ErrorStyle.Render(fmt.Sprintf("Error: %s\n%s", m.actionErr.Error(), m.actionOut)) + "\n\n")
		} else {
			sb.WriteString(tuistyles.SuccessStyle.Render("✓ Added to docker group. Log out and back in to apply.") + "\n\n")
		}
	}

	// Continue button
	continueIdx := 0
	if m.status.installed && !m.status.inGroup {
		continueIdx = 1
	}
	btnStyle := tuistyles.ButtonStyle
	if m.focusIdx == continueIdx {
		btnStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(btnStyle.Render("  Continue  ") + "\n")

	return sb.String()
}
