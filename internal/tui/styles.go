package tui

import (
	"github.com/MSamoilovic/gator-cli/internal/theme"

	"github.com/charmbracelet/lipgloss"
)

var (
	accent   = theme.Accent
	onAccent = theme.OnAccent
	muted    = theme.Muted

	focusedTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(onAccent).
				Background(accent).
				Padding(0, 1)

	blurredTitleStyle = lipgloss.NewStyle().
				Foreground(muted).
				Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().Foreground(accent)

	userTagStyle = lipgloss.NewStyle().Foreground(muted)

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
