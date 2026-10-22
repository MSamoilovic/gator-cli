package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	paletteTitle     = "Commands"
	paletteLabelCols = 38
)

type paletteItem struct {
	label     string
	hint      string
	localOnly bool
	run       func(model) (tea.Model, tea.Cmd)
}

func (i paletteItem) Title() string {
	if i.hint == "" {
		return i.label
	}
	return fmt.Sprintf("%-*s %s", paletteLabelCols, i.label, i.hint)
}

func (i paletteItem) Description() string { return "" }

func (i paletteItem) FilterValue() string { return i.label }

func (m model) paletteActions() []paletteItem {
	return []paletteItem{
		{label: "Fetch feeds now", hint: "R", localOnly: true, run: model.fetchAll},
		{label: "Reload the list", hint: "r", run: model.reload},
		{label: "Search posts…", hint: "s", run: promptFor(inputSearch, "search: ")},
		{label: "Add a feed by URL…", hint: "a", run: promptFor(inputAddFeed, "feed url: ")},
		{label: "Browse the feed catalog", hint: "c", run: func(m model) (tea.Model, tea.Cmd) {
			return m.openCatalog()
		}},
		{label: "Move this feed to a folder…", run: model.promptCategory},
		{label: "Unfollow the selected feed", hint: "d", run: model.askUnfollow},
		{label: "Show saved posts", hint: "B", run: model.toggleBookmarksView},
		{label: "Show unread posts only", hint: "U", run: model.toggleUnreadOnly},
		{label: "Show read posts only", hint: "H", run: model.toggleReadView},
		{label: "Show all posts again", hint: "esc", run: model.leaveDerivedView},
		{label: "Mark everything listed as read", hint: "A", run: model.markListRead},
		{label: "Mark this post read or unread", hint: "u", run: model.toggleSelectedRead},
		{label: "Save or unsave this post", hint: "b", run: model.toggleSelectedBookmark},
		{label: "Open this post in the browser", hint: "o", run: model.openSelectedInBrowser},
		{label: "Copy this post's URL", hint: "y", run: model.copySelectedURL},
		{label: "Read this post in the pager", hint: "e", run: model.readSelectedInPager},
		{label: "Flip the sort order", hint: "S", run: model.toggleSort},
		{label: "Switch the list density", run: model.toggleDensity},
		{label: "Cycle the time range", hint: "t", run: model.cycleTimeRange},
		{label: "Collapse every folder", hint: "0", run: foldAll(false)},
		{label: "Expand every folder", hint: "-", run: foldAll(true)},
	}
}

func promptFor(mode inputMode, prompt string) func(model) (tea.Model, tea.Cmd) {
	return func(m model) (tea.Model, tea.Cmd) {
		return m, m.startInput(mode, prompt)
	}
}

func foldAll(expand bool) func(model) (tea.Model, tea.Cmd) {
	return func(m model) (tea.Model, tea.Cmd) {
		return m.refoldAll(expand)
	}
}

func jumpTo(name string) func(model) (tea.Model, tea.Cmd) {
	return func(m model) (tea.Model, tea.Cmd) {
		return m.openFolder(name)
	}
}

func (m model) paletteItems() []list.Item {
	actions := m.paletteActions()
	folders := groupFeeds(m.feeds)

	items := make([]list.Item, 0, len(actions)+len(folders))
	for _, a := range actions {
		if a.localOnly && m.remote() {
			continue
		}
		items = append(items, a)
	}

	if !hasFolders(m.feeds) {
		return items
	}

	for i, fol := range folders {
		hint := ""
		if i < maxFolderKeys {
			hint = fmt.Sprintf("%d", i+1)
		}
		items = append(items, paletteItem{
			label: "Folder: " + fol.name,
			hint:  hint,
			run:   jumpTo(fol.name),
		})
	}
	return items
}

func (m model) openPalette() (tea.Model, tea.Cmd) {
	m.screen = screenPalette
	m.paletteList.ResetSelected()

	cmd := m.paletteList.SetItems(m.paletteItems())
	m.paletteList.SetSize(m.width, m.panelHeight())
	m.paletteList.SetFilterText("")
	m.paletteList.SetFilterState(list.Filtering)
	return m, cmd
}

func (m model) closePalette() model {
	m.screen = screenList
	m.paletteList.ResetFilter()
	return m
}

func (m model) runPalette() (tea.Model, tea.Cmd) {
	item, ok := m.paletteList.SelectedItem().(paletteItem)
	if !ok {
		return m.closePalette(), nil
	}
	return item.run(m.closePalette())
}

func (m model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEsc:
		return m.closePalette(), nil

	case tea.KeyEnter:
		return m.runPalette()

	case tea.KeyUp:
		m.paletteList.CursorUp()
		return m, nil

	case tea.KeyDown:
		m.paletteList.CursorDown()
		return m, nil
	}

	switch msg.String() {
	case "ctrl+p":
		m.paletteList.CursorUp()
		return m, nil
	case "ctrl+n":
		m.paletteList.CursorDown()
		return m, nil
	}

	var cmd tea.Cmd
	m.paletteList, cmd = m.paletteList.Update(msg)
	return m, cmd
}

func (m model) paletteView() string {
	return m.paletteList.View() + "\n" + m.footer()
}
