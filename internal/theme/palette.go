package theme

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	Accent       = lipgloss.AdaptiveColor{Light: "#0B5A94", Dark: "#1E7FC4"}
	OnAccent     = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFDF5"}
	Muted        = lipgloss.AdaptiveColor{Light: "#5C5C5C", Dark: "#9B9B9B"}
	Highlight    = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#5CD6F0"}
	HighlightDim = lipgloss.AdaptiveColor{Light: "#7FD3E8", Dark: "#3A9CB8"}
)

func ItemStyles() list.DefaultItemStyles {
	s := list.NewDefaultItemStyles()
	s.SelectedTitle = s.SelectedTitle.BorderForeground(HighlightDim).Foreground(Highlight)
	s.SelectedDesc = s.SelectedDesc.BorderForeground(HighlightDim).Foreground(HighlightDim)
	return s
}

func Style(l *list.Model) {
	s := list.DefaultStyles()
	s.Title = s.Title.Background(Accent).Foreground(OnAccent)
	s.FilterCursor = s.FilterCursor.Foreground(Highlight)
	l.Styles = s
	l.FilterInput.PromptStyle = s.FilterPrompt
	l.FilterInput.Cursor.Style = s.FilterCursor
}

func Delegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = ItemStyles()
	return d
}
