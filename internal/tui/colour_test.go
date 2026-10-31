package tui

import (
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func inColour(t *testing.T) {
	t.Helper()
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
}

func escape(c lipgloss.TerminalColor) string {
	rendered := lipgloss.NewStyle().Foreground(c).Render("x")
	rendered = strings.TrimPrefix(rendered, "\x1b[")
	return strings.TrimSuffix(rendered, "mx\x1b[0m")
}

func TestNoScreenKeepsBubblesDefaultPink(t *testing.T) {
	inColour(t)

	pink := map[string]string{
		"#EE6FF8": escape(lipgloss.Color("#EE6FF8")),
		"#AD58B4": escape(lipgloss.Color("#AD58B4")),
		"#F793FF": escape(lipgloss.Color("#F793FF")),
	}

	m := ready(t, testPost("Prvi"), testPost("Drugi"))
	m = withFeeds(t, m, testFeed("Hacker News RSS"), testFeed("Lobsters"))
	m.commands = readerCommands()
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	screens := map[string]model{"post list": m}
	screens["feed pane"], _ = step(t, m, press("tab"))
	screens["palette"], _ = step(t, m, press(":"))
	screens["menu"], _ = step(t, m, press("m"))
	screens["catalog"], _ = m.openCatalog()

	for where, screen := range screens {
		view := screen.View()
		for name, seq := range pink {
			if strings.Contains(view, seq) {
				t.Errorf("the %s still renders bubbles' default %s — that list was built with "+
					"list.NewDefaultDelegate() or assigned Styles without going through theme.Style", where, name)
			}
		}
	}
}

func TestTheSelectedRowCarriesTheAccentColour(t *testing.T) {
	inColour(t)

	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	if !strings.Contains(m.View(), escape(theme.Highlight)) {
		t.Error("the post list highlights nothing with theme.Highlight, so the pink check proves nothing")
	}
}

func TestTheMenuPaintsItselfFromTheSamePalette(t *testing.T) {
	inColour(t)

	m := ready(t, testPost("Prvi"))
	m.commands = readerCommands()
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, press("m"))

	if !strings.Contains(m.View(), escape(theme.Accent)) {
		t.Error("the menu uses theme.Accent nowhere, so internal/menu has drifted from internal/theme")
	}
}
