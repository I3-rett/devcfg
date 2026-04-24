package tuistyles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	purple = lipgloss.Color("#7C3AED")
	teal   = lipgloss.Color("#0D9488")
	gray   = lipgloss.Color("#6B7280")
	white  = lipgloss.Color("#F9FAFB")
	green  = lipgloss.Color("#10B981")
	yellow = lipgloss.Color("#F59E0B")
	red    = lipgloss.Color("#EF4444")

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		MarginBottom(1)

	StepIndicatorStyle = lipgloss.NewStyle().
		Foreground(gray).
		MarginBottom(1)

	ItemStyle = lipgloss.NewStyle().
		Foreground(white)

	SelectedItemStyle = lipgloss.NewStyle().
		Foreground(teal).
		Bold(true)

	CheckedItemStyle = lipgloss.NewStyle().
		Foreground(green)

	ButtonStyle = lipgloss.NewStyle().
		Foreground(gray).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(gray).
		Padding(0, 2).
		MarginTop(1)

	ActiveButtonStyle = lipgloss.NewStyle().
		Foreground(white).
		Background(purple).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(purple).
		Padding(0, 2).
		MarginTop(1)

	StatusStyle = lipgloss.NewStyle().
		Foreground(gray).
		Italic(true)

	SuccessStyle = lipgloss.NewStyle().
		Foreground(green)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(red)

	WarningStyle = lipgloss.NewStyle().
		Foreground(yellow)

	DisabledItemStyle = lipgloss.NewStyle().
		Foreground(gray).
		Faint(true)

	DividerStyle = lipgloss.NewStyle().
		Foreground(gray)

	ConfirmStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(yellow).
		Padding(1, 3).
		MarginTop(1)

	// Split-screen pane styles used during tool installation.
	OpPaneBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(gray)

	LogPaneBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(teal)

	PaneTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(white).
		MarginBottom(1)
)
