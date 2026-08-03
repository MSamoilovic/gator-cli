package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Scroll   key.Binding
	Read     key.Binding
	Select   key.Binding
	Open     key.Binding
	Bookmark key.Binding
	Search   key.Binding
	Filter   key.Binding
	Tab      key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Scroll: key.NewBinding(
			key.WithKeys("up", "down"),
			key.WithHelp("↑/↓", "scroll"),
		),
		Read: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "read"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "select"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open"),
		),
		Bookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "mark"),
		),
		Search: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab", "shift+tab"),
			key.WithHelp("tab", "pane"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}
 
func (k keyMap) listHelp(withFeeds, inSearch bool) []key.Binding {
	b := []key.Binding{k.Read, k.Open, k.Bookmark, k.Search, k.Filter}
	if withFeeds {
		b = append(b, k.Tab)
	}
	if inSearch {
		b = append(b, k.Back)
	}
	return append(b, k.Quit)
}

func (k keyMap) feedsHelp() []key.Binding {
	return []key.Binding{k.Select, k.Tab, k.Quit}
}

func (k keyMap) detailHelp() []key.Binding {
	return []key.Binding{k.Scroll, k.Open, k.Bookmark, k.Back, k.Quit}
}

func (k keyMap) searchHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "search")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}
