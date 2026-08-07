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
	Saved    key.Binding
	Sort     key.Binding
	Reload   key.Binding
	Fetch    key.Binding
	AddFeed  key.Binding
	Unfollow key.Binding
	Unread   key.Binding
	AllRead  key.Binding
	OnlyNew  key.Binding
	Since    key.Binding
	Copy     key.Binding
	Next     key.Binding
	Prev     key.Binding
	Search   key.Binding
	Filter   key.Binding
	Tab      key.Binding
	Back     key.Binding
	Help     key.Binding
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
		Saved: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "saved"),
		),
		Sort: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort"),
		),
		Reload: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reload"),
		),
		Fetch: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "fetch"),
		),
		AddFeed: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add feed"),
		),
		Unfollow: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "unfollow"),
		),
		Unread: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "unread"),
		),
		AllRead: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "all read"),
		),
		OnlyNew: key.NewBinding(
			key.WithKeys("U"),
			key.WithHelp("U", "only unread"),
		),
		Since: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "time range"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy url"),
		),
		Next: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next"),
		),
		Prev: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev"),
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
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) fullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Read, k.Back},
		{k.Open, k.Bookmark, k.Saved, k.Sort},
		{k.Unread, k.AllRead, k.OnlyNew, k.Since},
		{k.Search, k.Filter, k.Copy, k.Tab},
		{k.Reload, k.Fetch, k.AddFeed, k.Unfollow},
		{k.Next, k.Prev, k.Help, k.Quit},
	}
}

// Kratak help mora da stane u 80 kolona, pa nose samo najcesce akcije —
// ostalo je iza "?".
func (k keyMap) listHelp(withFeeds, canGoBack bool) []key.Binding {
	b := []key.Binding{k.Read, k.Open, k.Bookmark, k.Search}
	if withFeeds {
		b = append(b, k.Tab)
	}
	if canGoBack {
		b = append(b, k.Back)
	}
	return append(b, k.Help, k.Quit)
}

func (k keyMap) feedsHelp() []key.Binding {
	return []key.Binding{k.Select, k.AddFeed, k.Unfollow, k.Tab, k.Help, k.Quit}
}

func (k keyMap) detailHelp() []key.Binding {
	return []key.Binding{k.Scroll, k.Next, k.Prev, k.Open, k.Copy, k.Back, k.Quit}
}
