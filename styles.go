package main

import "charm.land/lipgloss/v2"

var (
	// Pane borders
	listBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	previewBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	// List items
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Client indicators
	activeClientStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("76")) // green

	inactiveClientStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")) // dim

	// Dir path in list
	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)

	// Pane titles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99"))

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)

	// Status messages
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	// Confirm prompt
	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Log pane
	logBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	logDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)
