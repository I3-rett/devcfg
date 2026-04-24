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

	// TitleStyle is the lipgloss style for the application title.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			MarginBottom(1)

	// StepIndicatorStyle is the lipgloss style for the step indicator line.
	StepIndicatorStyle = lipgloss.NewStyle().
				Foreground(gray).
				MarginBottom(1)

	// ItemStyle is the default lipgloss style for list items.
	ItemStyle = lipgloss.NewStyle().
			Foreground(white)

	// SelectedItemStyle is the lipgloss style for the currently focused item.
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(teal).
				Bold(true)

	// CheckedItemStyle is the lipgloss style for selected/checked items.
	CheckedItemStyle = lipgloss.NewStyle().
				Foreground(green)

	// ButtonStyle is the lipgloss style for an inactive button.
	ButtonStyle = lipgloss.NewStyle().
			Foreground(gray).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(gray).
			Padding(0, 2).
			MarginTop(1)

	// ActiveButtonStyle is the lipgloss style for the focused/active button.
	ActiveButtonStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(purple).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(purple).
				Padding(0, 2).
				MarginTop(1)

	// StatusStyle is the lipgloss style for status/info messages.
	StatusStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true)

	// SuccessStyle is the lipgloss style for success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(green)

	// ErrorStyle is the lipgloss style for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(red)

	// WarningStyle is the lipgloss style for warning messages.
	WarningStyle = lipgloss.NewStyle().
			Foreground(yellow)

	// DisabledItemStyle is the lipgloss style for unavailable/disabled items.
	DisabledItemStyle = lipgloss.NewStyle().
				Foreground(gray).
				Faint(true)

	// DividerStyle is the lipgloss style for horizontal dividers.
	DividerStyle = lipgloss.NewStyle().
			Foreground(gray)

	// ConfirmStyle is the lipgloss style for confirmation overlay boxes.
	ConfirmStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(yellow).
			Padding(1, 3).
			MarginTop(1)

	// OpPaneBorderStyle is the border style for the operation pane in the split-screen tool installation view.
	OpPaneBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(gray)

	// LogPaneBorderStyle is the border style for the log pane in the split-screen tool installation view.
	LogPaneBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(teal)

	// PaneTitleStyle is the style for pane titles in the split-screen tool installation view.
	PaneTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			MarginBottom(1)
)
