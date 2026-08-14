package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorBorder    = lipgloss.Color("240")
	colorActiveTab = lipgloss.Color("39")
	colorDim       = lipgloss.Color("245")
	colorGreen     = lipgloss.Color("42")
	colorRed       = lipgloss.Color("196")
	colorYellow    = lipgloss.Color("214")
	colorBg        = lipgloss.Color("234")

	// Header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	// Tab styles
	tabStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(colorActiveTab).
			Padding(0, 1)

	// Box / Container
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	// Footer / Status
	footerStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	badgeGreen = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	badgeRed = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	badgeYellow = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)
)
