package tui

import (
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func testFeed(name string) database.GetFeedFollowsForUserRow {
	return database.GetFeedFollowsForUserRow{FeedID: uuid.New(), FeedName: name}
}

func withFeeds(t *testing.T, m model, feeds ...database.GetFeedFollowsForUserRow) model {
	t.Helper()
	m, _ = step(t, m, feedsLoadedMsg{feeds: feeds})
	return m
}

func TestFeedsLoadedKeepsAllFeedsFirst(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"), testFeed("CBR"))

	items := m.feedList.Items()
	if got, want := len(items), 3; got != want {
		t.Fatalf("feed items = %d, want %d", got, want)
	}
	if got, want := items[0].(feedItem).name, allFeedsLabel; got != want {
		t.Errorf("first item = %q, want %q", got, want)
	}
	if items[0].(feedItem).id != uuid.Nil {
		t.Error("All feeds entry must carry the nil UUID")
	}
	if got, want := items[1].(feedItem).name, "BBC Sport"; got != want {
		t.Errorf("second item = %q, want %q", got, want)
	}
}

func TestTabSwitchesFocus(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	if m.focus != focusPosts {
		t.Fatal("focus should start on posts")
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusFeeds {
		t.Fatal("tab did not move focus to feeds")
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusPosts {
		t.Error("tab did not move focus back to posts")
	}
}

func TestSelectingFeedFiltersPosts(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, ready(t, testPost("Prvi")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("selecting a feed did not trigger a reload")
	}
	if m.feedID != feed.FeedID {
		t.Errorf("feedID = %v, want %v", m.feedID, feed.FeedID)
	}
	if m.focus != focusPosts {
		t.Error("focus should return to posts after selecting a feed")
	}
	if got, want := m.list.Title, "BBC Sport"; got != want {
		t.Errorf("posts title = %q, want %q", got, want)
	}
}

func TestSelectingAllFeedsClearsFilter(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, ready(t, testPost("Prvi")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("selecting All feeds did not trigger a reload")
	}
	if m.feedID != uuid.Nil {
		t.Errorf("feedID = %v, want nil UUID", m.feedID)
	}
	if got, want := m.list.Title, "Posts"; got != want {
		t.Errorf("posts title = %q, want %q", got, want)
	}
}

func TestFeedFocusIgnoresPostActions(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	for _, k := range []string{"o", "b"} {
		next, cmd := step(t, m, press(k))
		if cmd != nil {
			t.Errorf("%q triggered an action while the feed panel had focus", k)
		}
		if next.status != "" {
			t.Errorf("%q set a status while the feed panel had focus: %q", k, next.status)
		}
	}
}

func TestFeedFocusQuits(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	_, cmd := step(t, m, press("q"))

	if cmd == nil {
		t.Fatal("no command returned")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q did not quit from the feed panel")
	}
}

func TestNarrowTerminalHidesFeedPanel(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusFeeds {
		t.Fatal("focus did not move to feeds")
	}

	m, _ = step(t, m, tea.WindowSizeMsg{Width: 40, Height: 24})

	if m.feedWidth != 0 {
		t.Errorf("feedWidth = %d, want 0 on a narrow terminal", m.feedWidth)
	}
	if m.focus != focusPosts {
		t.Error("focus must fall back to posts when the feed panel is hidden")
	}
	if strings.Contains(m.View(), allFeedsLabel) {
		t.Error("feed panel still rendered on a narrow terminal")
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusPosts {
		t.Error("tab must not focus a hidden feed panel")
	}
}

func TestPanelsFitTerminal(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi"), testPost("Drugi")), testFeed("BBC Sport"), testFeed("CBR"))

	view := m.View()

	if got, want := strings.Count(view, "\n")+1, 24; got != want {
		t.Errorf("view = %d lines, want %d", got, want)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Fatalf("line wider than terminal: %d columns", w)
		}
	}
	if !strings.Contains(view, allFeedsLabel) {
		t.Error("feed panel missing from the view")
	}
	if !strings.Contains(view, "Prvi") {
		t.Error("post list missing from the view")
	}
}

func TestFeedHintsShownWhenFeedsFocused(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	if got := m.View(); !strings.Contains(got, "tab pane") {
		t.Errorf("posts hint missing tab:\n%s", got)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	got := m.View()
	if !strings.Contains(got, "⏎ select") {
		t.Errorf("feeds hint not shown:\n%s", got)
	}
	if strings.Contains(got, "b mark") {
		t.Errorf("posts hints shown while feeds have focus:\n%s", got)
	}
}

func TestFeedPanelMarksBrokenFeeds(t *testing.T) {
	healthy := testFeed("BBC Sport")
	broken := testFeed("CBR")
	broken.FeedFailures = 3

	m := withFeeds(t, ready(t, testPost("Prvi")), healthy, broken)

	items := m.feedList.Items()
	if got := items[1].(feedItem).Title(); strings.Contains(got, brokenMark) {
		t.Errorf("healthy feed carries the broken mark: %q", got)
	}
	if got := items[2].(feedItem).Title(); !strings.Contains(got, brokenMark) {
		t.Errorf("failing feed = %q, want it marked with %q", got, brokenMark)
	}

	if view := m.View(); !strings.Contains(view, brokenMark+" CBR") {
		t.Errorf("feed panel does not show the broken feed:\n%s", view)
	}
}

// Marker i brojac nepracitanih moraju da stoje zajedno, ne da se iskljucuju.
func TestBrokenMarkAndUnreadCountCoexist(t *testing.T) {
	broken := testFeed("CBR")
	broken.FeedFailures = 1

	m := withFeeds(t, ready(t, testPost("Prvi")), broken)
	item := m.feedList.Items()[1].(feedItem)
	m.unread[item.id] = 4

	got := item.Title()
	if !strings.Contains(got, brokenMark) || !strings.Contains(got, "(4)") {
		t.Errorf("title = %q, want both the broken mark and the unread count", got)
	}
}
