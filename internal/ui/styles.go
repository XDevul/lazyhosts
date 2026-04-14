package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorGreen     = lipgloss.Color("#73D216")
	colorRed       = lipgloss.Color("#FF5555")
	colorYellow    = lipgloss.Color("#F4BF75")
	colorGray      = lipgloss.Color("#6C7086")
	colorWhite     = lipgloss.Color("#CDD6F4")
	colorHighlight = lipgloss.Color("#45475A")

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray).
			Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	// Title
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	// Profile list items
	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(colorHighlight).
				Bold(true).
				Padding(0, 1)

	EnabledStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	DisabledStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Padding(0, 1)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(0, 1)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Padding(0, 1)

	// Help
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	// Detail panel
	DetailLabelStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true)

	DetailValueStyle = lipgloss.NewStyle().
				Foreground(colorWhite)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Confirm dialog
	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorYellow).
			Padding(1, 2).
			Align(lipgloss.Center)

	// Loading
	LoadingStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	// Search
	SearchStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Help popup overlay
	HelpPopupStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Width(50)

	// Input popup overlay
	InputPopupStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorGreen).
			Padding(1, 2)
)
