package tui

import (
	"github.com/MSamoilovic/gator-cli/internal/menu"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	menuTitle = "gator — what the reader cannot do itself"
	menuHint  = "↑↓ move · ⏎ run it and leave the reader · / filter · esc back"
)

func (m model) openMenu() (tea.Model, tea.Cmd) {
	if len(m.commands) == 0 {
		return m.withStatus("This build offers no commands to run")
	}

	m.screen = screenMenu
	m.menu = menu.New(menu.Config{
		Title:    menuTitle,
		Greeting: m.menuGreeting(),
		Hint:     menuHint,
		Items:    m.commands,
	})
	m.menu.SetSize(m.width, m.height)
	return m, nil
}

func (m model) menuGreeting() string {
	tags := m.footerTags()
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}

func (m model) closeMenu() model {
	m.screen = screenList
	m.menu = menu.Model{}
	return m
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	next, cmd := m.menu.Update(msg)
	m.menu = next

	if choice, chosen := next.Chosen(); chosen {
		m.choice = choice
		m.chose = true
		return m, tea.Quit
	}
	if next.Cancelled() {
		return m.closeMenu(), nil
	}
	return m, cmd
}

func (m model) menuView() string { return m.menu.View() }
