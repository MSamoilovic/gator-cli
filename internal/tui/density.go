package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/MSamoilovic/gator-cli/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func postDelegate(d density) grouped {
	del := theme.Delegate()
	if d == densityCompact {
		del.ShowDescription = false
		del.SetSpacing(0)
	}
	return grouped{DefaultDelegate: del}
}

type grouped struct {
	list.DefaultDelegate
}

func (d grouped) Render(w io.Writer, m list.Model, index int, item list.Item) {
	h, ok := item.(feedHeader)
	if !ok {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}
	fmt.Fprint(w, feedHeaderStyle.Render(rule(h.label(), m.Width()))+strings.Repeat("\n", d.Height()-1))
}

func rule(label string, width int) string {
	line := "── " + label + " "
	if fill := width - headerIndent - lipgloss.Width(line); fill > 0 {
		return line + strings.Repeat("─", fill)
	}
	return lipgloss.NewStyle().MaxWidth(max(width-headerIndent, 1)).Render(line)
}

func (m model) toggleDensity() (tea.Model, tea.Cmd) {
	m.density = m.density.flipped()
	m.list.SetDelegate(postDelegate(m.density))
	return m.withStatus("List density: " + m.density.String())
}
