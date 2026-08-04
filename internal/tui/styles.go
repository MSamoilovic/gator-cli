package tui

import "github.com/charmbracelet/lipgloss"


var (
	accent   = lipgloss.AdaptiveColor{Light: "#5A3FD6", Dark: "#7D56F4"}
	onAccent = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFDF5"}
	muted    = lipgloss.AdaptiveColor{Light: "#5C5C5C", Dark: "#9B9B9B"}

	focusedTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(onAccent).
				Background(accent).
				Padding(0, 1)

	blurredTitleStyle = lipgloss.NewStyle().
				Foreground(muted).
				Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().Foreground(accent)

	emptyStateStyle = lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 2)
)

func panelTitleStyle(focused bool) lipgloss.Style {
	if focused {
		return focusedTitleStyle
	}
	return blurredTitleStyle
}
