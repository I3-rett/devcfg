package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

// ConfigModel displays runtime configuration info about devcfg.
type ConfigModel struct {
	version    string
	binaryPath string
	configDir  string
	installDir string
}

// NewConfigModel creates a ConfigModel with the given build version.
func NewConfigModel(version string) *ConfigModel {
	m := &ConfigModel{version: version}

	// Resolve binary path and derive the install directory from it.
	if exe, err := os.Executable(); err == nil {
		m.binaryPath = exe
		m.installDir = filepath.Dir(exe)
	} else {
		m.binaryPath = "(unknown)"
		m.installDir = "(unknown)"
	}

	// Config directory: $XDG_CONFIG_HOME/devcfg or ~/.config/devcfg.
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		m.configDir = filepath.Join(xdg, "devcfg")
	} else if home, err := os.UserHomeDir(); err == nil {
		m.configDir = filepath.Join(home, ".config", "devcfg")
	} else {
		m.configDir = "(unknown)"
	}

	return m
}

// Title returns the display name of this tab.
func (m *ConfigModel) Title() string { return "Config" }

// IsDone always returns false – the Config tab has no completion state.
func (m *ConfigModel) IsDone() bool { return false }

// CanQuit always returns true for the Config tab.
func (m *ConfigModel) CanQuit() bool { return true }

// CanSwitchTabs always returns true for the Config tab.
func (m *ConfigModel) CanSwitchTabs() bool { return true }

// Init is a no-op for the Config tab.
func (m *ConfigModel) Init() tea.Cmd { return nil }

// Update handles messages for the Config tab (currently no interactive state).
func (m *ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders the Config tab content.
func (m *ConfigModel) View() string {
	var sb strings.Builder

	heading := func(s string) string {
		return tuistyles.TitleStyle.Render(s)
	}
	label := func(s string) string {
		return tuistyles.SelectedItemStyle.Render(s)
	}
	value := func(s string) string {
		return tuistyles.ItemStyle.Render(s)
	}
	row := func(lbl, val string) string {
		return fmt.Sprintf("  %s  %s", label(fmt.Sprintf("%-11s", lbl)), value(val))
	}

	sb.WriteString(heading("devcfg — Configuration") + "\n\n")

	sb.WriteString(tuistyles.DividerStyle.Render("─── Version ─────────────────────────") + "\n")
	sb.WriteString(row("Version:", m.version) + "\n\n")

	sb.WriteString(tuistyles.DividerStyle.Render("─── Paths ───────────────────────────") + "\n")
	sb.WriteString(row("Binary:", m.binaryPath) + "\n")
	sb.WriteString(row("Install:", m.installDir) + "\n")
	sb.WriteString(row("Config dir:", m.configDir) + "\n\n")

	sb.WriteString(tuistyles.DividerStyle.Render("─── Overrides ───────────────────────") + "\n")
	sb.WriteString(tuistyles.StatusStyle.Render("  Interactive JSON config editor coming soon.") + "\n")

	return sb.String()
}
