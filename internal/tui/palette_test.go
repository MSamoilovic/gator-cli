package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func paletteLabels(m model) []string {
	items := m.paletteList.VisibleItems()
	out := make([]string, 0, len(items))
	for _, it := range items {
		if pi, ok := it.(paletteItem); ok {
			out = append(out, pi.label)
		}
	}
	return out
}

func paletteTyping(t *testing.T, m model, text string) model {
	t.Helper()
	for _, r := range text {
		next, cmd := step(t, m, press(string(r)))

		found := make(chan list.FilterMatchesMsg, 1)
		go collect(cmd, found)

		select {
		case msg := <-found:
			next, _ = step(t, next, msg)
		case <-time.After(2 * time.Second):
			t.Fatalf("typing %q into the palette produced no FilterMatchesMsg", string(r))
		}
		m = next
	}
	return m
}

func openPalette(t *testing.T, m model) model {
	t.Helper()
	m = typing(t, m, ":")
	if m.screen != screenPalette {
		t.Fatalf("screen = %v, want the palette", m.screen)
	}
	return m
}

func TestEveryActionInThePaletteHasALabelAndAHandler(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")

	actions := m.paletteActions()
	if len(actions) < 15 {
		t.Fatalf("palette holds %d actions, want the full set", len(actions))
	}

	seen := make(map[string]bool, len(actions))
	for i, a := range actions {
		if strings.TrimSpace(a.label) == "" {
			t.Errorf("action %d has no label", i)
		}
		if a.run == nil {
			t.Errorf("action %q has no handler", a.label)
		}
		if seen[a.label] {
			t.Errorf("action %q is listed twice", a.label)
		}
		seen[a.label] = true
	}
}

func TestPaletteOpensFilteringSoTypingSearches(t *testing.T) {
	m := openPalette(t, manyFolders(t, "Comics", "Go"))

	if got := m.paletteList.FilterState(); got != list.Filtering {
		t.Fatalf("filter state = %v, want Filtering so the first keystroke searches", got)
	}

	before := len(paletteLabels(m))
	m = paletteTyping(t, m, "unfollow")

	got := paletteLabels(m)
	if len(got) == 0 {
		t.Fatal("typing \"unfollow\" matched nothing")
	}
	if len(got) >= before {
		t.Errorf("typing narrowed %d entries to %d, want fewer", before, len(got))
	}
	if !strings.Contains(got[0], "Unfollow") {
		t.Errorf("best match = %q, want the unfollow action ranked first", got[0])
	}
}

func TestPaletteFindsAFolderByNameEvenPastTheDigits(t *testing.T) {
	m := manyFolders(t, "a", "b", "c", "d", "e", "f", "g", "h", "i", "Web development")
	if _, ok := folderByNumber(m.feeds, 10); ok {
		t.Fatal("test needs Web development to be past the ninth folder")
	}

	m = openPalette(t, m)
	m = paletteTyping(t, m, "web")

	got := paletteLabels(m)
	if len(got) == 0 || !strings.Contains(got[0], "Web development") {
		t.Fatalf("matches = %v, want the Web development folder first", got)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenList {
		t.Errorf("screen = %v, want the palette closed", m.screen)
	}
	if !m.expanded["Web development"] {
		t.Error("running the folder entry did not open the folder")
	}
	if m.focus != focusFeeds {
		t.Error("running the folder entry did not move focus to the feed pane")
	}
}

func TestPaletteArrowsMoveTheCursorWhileFiltering(t *testing.T) {
	m := openPalette(t, manyFolders(t, "Comics", "Go"))

	if got := m.paletteList.Index(); got != 0 {
		t.Fatalf("palette opened on index %d, want 0", got)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if got := m.paletteList.Index(); got != 1 {
		t.Errorf("down moved to index %d, want 1 — the list disables its own cursor keys while filtering", got)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if got := m.paletteList.Index(); got != 0 {
		t.Errorf("up moved to index %d, want 0", got)
	}
}

func TestPaletteRunsTheSelectedActionAndCloses(t *testing.T) {
	m := openPalette(t, manyFolders(t, "Comics", "Go"))
	m = paletteTyping(t, m, "expand every")

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenList {
		t.Errorf("screen = %v, want the palette closed", m.screen)
	}
	if got, want := len(m.expanded), 2; got != want {
		t.Errorf("expanded %d folders, want %d — the action did not run", got, want)
	}
}

func TestEscapeClosesThePaletteWithoutRunningAnything(t *testing.T) {
	m := openPalette(t, manyFolders(t, "Comics", "Go"))
	m = paletteTyping(t, m, "expand every")

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.screen != screenList {
		t.Errorf("screen = %v, want the palette closed", m.screen)
	}
	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want esc to have run nothing", m.expanded)
	}
	if got := m.paletteList.FilterState(); got != list.Unfiltered {
		t.Errorf("filter state = %v, want the search text cleared for next time", got)
	}
}

func TestLettersInThePaletteDoNotTriggerTheirOwnShortcuts(t *testing.T) {
	m := openPalette(t, manyFolders(t, "Comics", "Go"))

	m = paletteTyping(t, m, "q0-1")

	if m.screen != screenPalette {
		t.Fatalf("screen = %v, want the palette still open — q must not quit while typing", m.screen)
	}
	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want 0, - and 1 to have gone into the search box", m.expanded)
	}
	if got := m.paletteList.FilterValue(); got != "q0-1" {
		t.Errorf("search box = %q, want %q", got, "q0-1")
	}
}

func TestPaletteOpensFromTheFeedPaneToo(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusFeeds {
		t.Fatal("test did not reach the feed pane")
	}

	m = typing(t, m, ":")
	if m.screen != screenPalette {
		t.Errorf("screen = %v, want the palette", m.screen)
	}
}

func TestCategorizeIsOnlyOfferedWithAFeedSelected(t *testing.T) {
	m := manyFolders(t, "Comics")

	m = openPalette(t, m)
	m = paletteTyping(t, m, "move this feed")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.input == inputCategorize {
		t.Fatal("prompted for a folder with no feed selected")
	}
	if !strings.Contains(m.status, "feed pane") {
		t.Errorf("status = %q, want it to say to pick a feed first", m.status)
	}
}

func TestCategorizePromptsAndSendsTheFeedURL(t *testing.T) {
	m := manyFolders(t, "Comics")
	m = typing(t, m, "1")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})

	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok {
		t.Fatalf("selection = %v, want the feed inside Comics", m.feedList.SelectedItem())
	}

	m = openPalette(t, m)
	m = paletteTyping(t, m, "move this feed")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.input != inputCategorize {
		t.Fatalf("input mode = %v, want the folder prompt", m.input)
	}
	if !strings.Contains(m.prompt.Prompt, item.name) {
		t.Errorf("prompt = %q, want it to name %q", m.prompt.Prompt, item.name)
	}

	m = typing(t, m, "Tech")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.input != inputNone {
		t.Error("the prompt stayed open after enter")
	}
	if !strings.Contains(m.status, "Tech") {
		t.Errorf("status = %q, want it to mention the folder", m.status)
	}
}

func TestCategorizedFeedLandsInAnOpenFolder(t *testing.T) {
	m := manyFolders(t, "Comics")

	m, _ = step(t, m, feedCategorizedMsg{name: "XKCD", folder: "Tech"})

	if !m.expanded["Tech"] {
		t.Error("the folder the feed moved into stayed closed, so the move looked like a disappearance")
	}
	if !strings.Contains(m.status, "XKCD") || !strings.Contains(m.status, "Tech") {
		t.Errorf("status = %q, want it to name the feed and the folder", m.status)
	}
}

func TestEveryBoundKeyAppearsInTheFullHelp(t *testing.T) {
	m := manyFolders(t, "Comics")
	full := m.help.FullHelpView(m.keys.fullHelp(m.remote()))

	onlyOnTheCatalogScreen := map[string]bool{"⏎ select": true, "space pick": true}
	catalog := m.help.ShortHelpView(m.keys.catalogHelp())

	var missing []string
	for _, b := range m.keys.all() {
		h := b.Help()
		if h.Key == "" {
			continue
		}

		label := h.Key + " " + h.Desc
		where := full
		if onlyOnTheCatalogScreen[label] {
			where = catalog
		}
		if !strings.Contains(where, h.Key) || !strings.Contains(where, h.Desc) {
			missing = append(missing, label)
		}
	}
	if len(missing) > 0 {
		t.Errorf("no help view shows %v:\n%s\n%s", missing, full, catalog)
	}

	for label := range onlyOnTheCatalogScreen {
		if strings.Contains(full, label) {
			t.Errorf("%q is listed in ? help but only works on the catalog screen", label)
		}
	}
}

func TestEveryFooterFitsEightyColumnsWithRoomForTheUserTag(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")

	lines := map[string][]key.Binding{
		"feeds":                m.keys.feedsHelp(),
		"list":                 m.keys.listHelp(true, false),
		"list bez feed panela": m.keys.listHelp(false, false),
		"izvedeni prikaz":      m.keys.listHelp(true, true),
		"catalog":              m.keys.catalogHelp(),
		"detail":               m.keys.detailHelp(),
		"palette":              m.keys.paletteHelp(),
	}

	const tag = 6
	for name, bindings := range lines {
		w := lipgloss.Width(m.help.ShortHelpView(bindings))
		if w+tag >= m.width {
			t.Errorf("%s help is %d columns; with the %d-column @user tag it does not fit %d, so the tag silently disappears",
				name, w, tag, m.width)
		}
	}
}

func TestPaletteFitsEightyByTwentyFour(t *testing.T) {
	m := manyFolders(t, "Basketball", "C# & .NET", "Comics", "Film & TV", "Fitness & training",
		"Food & cooking", "Gaming", "Go", "Guitar", "Machine learning & AI", "Motorsport",
		"Programming", "Python", "Science", "Sci-fi & fantasy", "Sci-fi & fantasy books",
		"Security", "Sport", "Tech", "TypeScript & JavaScript", "Web development")
	m = openPalette(t, m)

	view := m.View()
	if got, want := lipgloss.Height(view), 24; got > want {
		t.Errorf("palette is %d rows tall, want at most %d:\n%s", got, want, view)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 80 {
			t.Errorf("palette row %d is %d columns wide, want at most 80: %q", i, got, line)
		}
	}
	longest := ""
	for _, a := range m.paletteActions() {
		if len(a.label) > len(longest) {
			longest = a.label
		}
	}
	if !strings.Contains(view, longest) {
		t.Errorf("the longest label %q is cut off at 80 columns:\n%s", longest, view)
	}

	m = paletteTyping(t, m, "folder")
	narrowed := m.View()
	for i, line := range strings.Split(narrowed, "\n") {
		if got := lipgloss.Width(line); got > 80 {
			t.Errorf("filtered palette row %d is %d columns wide: %q", i, got, line)
		}
	}
}
