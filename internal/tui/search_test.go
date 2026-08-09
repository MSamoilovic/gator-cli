package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func typeText(t *testing.T, m model, text string) model {
	t.Helper()
	for _, r := range text {
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestSearchOpensAndCancels(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, press("s"))
	if !(m.input == inputSearch) {
		t.Fatal("s did not open the search input")
	}
	if !strings.Contains(m.View(), "search:") {
		t.Errorf("search prompt not rendered:\n%s", m.View())
	}

	m = typeText(t, m, "golang")
	if got, want := m.prompt.Value(), "golang"; got != want {
		t.Errorf("search value = %q, want %q", got, want)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.input == inputSearch {
		t.Error("esc did not close the search input")
	}
	if m.query != "" {
		t.Errorf("cancelled search still set a query: %q", m.query)
	}
}

func TestSearchRunsQueryAndSetsTitle(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "golang")
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("enter did not trigger a search")
	}
	if m.input == inputSearch {
		t.Error("search input still open after enter")
	}
	if got, want := m.query, "golang"; got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	if got, want := m.list.Title, "Search: golang"; got != want {
		t.Errorf("list title = %q, want %q", got, want)
	}
}

func TestSearchIgnoresBlankQuery(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "   ")
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("blank query triggered a search")
	}
	if m.query != "" {
		t.Errorf("blank query was stored: %q", m.query)
	}
	if got, want := m.list.Title, postsTitle; got != want {
		t.Errorf("list title = %q, want %q", got, want)
	}
}

func TestEscapeLeavesSearchResults(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, ready(t, testPost("Prvi")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "golang")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got, want := m.list.Title, "Search: golang"; got != want {
		t.Fatalf("list title = %q, want %q", got, want)
	}

	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc did not reload the feed")
	}
	if m.query != "" {
		t.Errorf("query still set after esc: %q", m.query)
	}
	if got, want := m.list.Title, "BBC Sport"; got != want {
		t.Errorf("list title = %q, want the feed name %q", got, want)
	}
	if m.feedID != feed.FeedID {
		t.Error("esc from search lost the feed filter")
	}
}

func TestEscapeDoesNothingWithoutSearch(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("esc reloaded posts with no active search")
	}
}

func TestSelectingFeedClearsSearch(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "golang")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.query != "" {
		t.Errorf("query survived a feed switch: %q", m.query)
	}
	if got, want := m.list.Title, "BBC Sport"; got != want {
		t.Errorf("list title = %q, want %q", got, want)
	}
}

func TestSearchKeysDoNotLeakToList(t *testing.T) {
	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	m, _ = step(t, m, press("s"))
	before := m.list.Index()

	m = typeText(t, m, "jjbo")

	if got := m.list.Index(); got != before {
		t.Errorf("list index moved while typing: %d, want %d", got, before)
	}
	if m.status != "" {
		t.Errorf("an action fired while typing: %q", m.status)
	}
	if got, want := m.prompt.Value(), "jjbo"; got != want {
		t.Errorf("search value = %q, want %q", got, want)
	}
}

func TestSearchCtrlCQuits(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, press("s"))
	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("no command returned")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c did not quit from the search input")
	}
}

func TestSpinnerShownWhileLoading(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, uuid.Nil, uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})

	if !strings.Contains(m.View(), "Loading posts") {
		t.Errorf("loading view missing:\n%s", m.View())
	}

	m, cmd := step(t, m, m.spinner.Tick())
	if cmd == nil {
		t.Error("spinner stopped ticking while loading")
	}

	m, _ = step(t, m, postsLoadedMsg{})
	_, cmd = step(t, m, spinner.TickMsg{})
	if cmd != nil {
		t.Error("spinner kept ticking after loading finished")
	}
}

func TestFooterFitsTerminal(t *testing.T) {
	widths := []int{30, 60, 80, 120}

	for _, w := range widths {
		m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))
		m, _ = step(t, m, tea.WindowSizeMsg{Width: w, Height: 24})

		for _, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("width %d: line of %d columns: %q", w, got, line)
			}
		}
	}
}

func TestFullHelpFitsEightyColumns(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	footer := m.footer()

	if strings.Contains(footer, "…") {
		t.Errorf("help truncated at 80 columns: %q", footer)
	}
	for _, want := range []string{"⏎ read", "o open", "b mark", "s search", "tab pane", "? help", "q quit"} {
		if !strings.Contains(footer, want) {
			t.Errorf("short help missing %q: %q", want, footer)
		}
	}

	m, _ = step(t, m, press("?"))
	full := m.footer()
	for _, want := range []string{"saved", "sort", "filter", "back"} {
		if !strings.Contains(full, want) {
			t.Errorf("full help missing %q:\n%s", want, full)
		}
	}
}
