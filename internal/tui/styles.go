package tui

import "github.com/charmbracelet/lipgloss"

var (
	focusedTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
				Padding(0, 1)

	blurredTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
)

func panelTitleStyle(focused bool) lipgloss.Style {
	if focused {
		return focusedTitleStyle
	}
	return blurredTitleStyle
}
