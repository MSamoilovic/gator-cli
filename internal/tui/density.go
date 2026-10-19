package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type density int

const (
	densityComfortable density = iota
	densityCompact
)

const (
	nameComfortable = "comfortable"
	nameCompact     = "compact"
)

func parseDensity(s string) density {
	if s == nameCompact {
		return densityCompact
	}
	return densityComfortable
}

func (d density) String() string {
	if d == densityCompact {
		return nameCompact
	}
	return nameComfortable
}

func (d density) flipped() density {
	if d == densityCompact {
		return densityComfortable
	}
	return densityCompact
}

func postDelegate(d density) list.DefaultDelegate {
	del := list.NewDefaultDelegate()
	if d == densityCompact {
		del.ShowDescription = false
		del.SetSpacing(0)
	}
	return del
}

func (m model) toggleDensity() (tea.Model, tea.Cmd) {
	m.density = m.density.flipped()
	m.list.SetDelegate(postDelegate(m.density))
	return m.withStatus("List density: " + m.density.String())
}
