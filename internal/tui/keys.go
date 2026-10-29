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
	History  key.Binding
	Sort     key.Binding
	Reload   key.Binding
	Fetch    key.Binding
	AddFeed  key.Binding
	Catalog  key.Binding
	Toggle   key.Binding
	Unfollow key.Binding
	Unread   key.Binding
	AllRead  key.Binding
	OnlyNew  key.Binding
	Since    key.Binding
	Copy     key.Binding
	Pager    key.Binding
	FullText key.Binding
	Next     key.Binding
	Prev     key.Binding
	Search   key.Binding
	Filter   key.Binding
	Tab      key.Binding
	Palette  key.Binding
	Menu     key.Binding
	Folder   key.Binding
	StepFold key.Binding
	FoldAll  key.Binding
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
		History: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "read"),
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
		Catalog: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "catalog"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "pick"),
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
		Pager: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "pager"),
		),
		FullText: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "full text"),
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
		Palette: key.NewBinding(
			key.WithKeys(":", "ctrl+k"),
			key.WithHelp(":", "commands"),
		),
		Menu: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "menu"),
		),
		Folder: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("1-9", "folder"),
		),
		StepFold: key.NewBinding(
			key.WithKeys("[", "]"),
			key.WithHelp("[/]", "prev/next"),
		),
		FoldAll: key.NewBinding(
			key.WithKeys("0", "-"),
			key.WithHelp("0/-", "fold all"),
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

func (k keyMap) fullHelp(remote bool) [][]key.Binding {
	fetch := k.Fetch
	if remote {
		fetch = key.NewBinding(key.WithKeys("R"))
	}

	return [][]key.Binding{
		{k.Up, k.Down, k.Read, k.Back, k.Tab, k.Help, k.Quit},
		{k.Open, k.Copy, k.FullText, k.Next, k.Prev, k.Unread, k.Bookmark},
		{k.Saved, k.History, k.OnlyNew, k.AllRead, k.Sort, k.Since, k.Filter},
		{k.Search, k.Palette, k.Reload, fetch, k.AddFeed, k.Unfollow, k.Catalog},
		{k.Folder, k.StepFold, k.FoldAll, k.Pager, k.Menu},
	}
}

func (k keyMap) listHelp(withFeeds, inDerivedView bool) []key.Binding {
	b := []key.Binding{k.Read, k.Bookmark, k.Palette, k.Menu}
	switch {
	case inDerivedView:
		b = append(b, k.Back)
	case withFeeds:
		b = append(b, k.Tab)
	}
	return append(b, k.Help, k.Quit)
}

func (k keyMap) feedsHelp() []key.Binding {
	return []key.Binding{k.Select, k.Palette, k.Menu, k.AddFeed, k.Tab, k.Help, k.Quit}
}

func (k keyMap) catalogHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Select, k.Back, k.Quit}
}

func (k keyMap) detailHelp() []key.Binding {
	return []key.Binding{k.FullText, k.Pager, k.Next, k.Prev, k.Open, k.Back, k.Quit}
}

func (k keyMap) paletteHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Back}
}

func (k keyMap) all() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Read, k.Select, k.Open, k.Bookmark, k.Saved, k.History,
		k.Sort, k.Reload, k.Fetch, k.AddFeed, k.Catalog, k.Toggle, k.Unfollow,
		k.Unread, k.AllRead, k.OnlyNew, k.Since, k.Copy, k.Pager, k.FullText, k.Next,
		k.Prev, k.Search, k.Filter, k.Tab, k.Palette, k.Menu, k.Folder, k.StepFold,
		k.FoldAll, k.Back, k.Help, k.Quit,
	}
}
