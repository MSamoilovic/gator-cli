package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func feedsFocused(t *testing.T, m model) model {
	t.Helper()
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	return m
}

func TestAddFeedPromptsForURL(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	m, _ = step(t, m, press("a"))
	if m.input != inputAddFeed {
		t.Fatal("a did not open the add-feed prompt")
	}
	if !strings.Contains(m.View(), "feed url:") {
		t.Errorf("prompt not rendered:\n%s", m.View())
	}

	m = typeText(t, m, "https://example.com/rss")
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("enter did not start adding the feed")
	}
	if m.input != inputNone {
		t.Error("prompt still open after enter")
	}
	if !strings.Contains(m.status, "Adding") {
		t.Errorf("status = %q, want progress feedback", m.status)
	}
}

func TestAddFeedCancelsOnEscape(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	m, _ = step(t, m, press("a"))
	m = typeText(t, m, "https://example.com/rss")
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.input != inputNone {
		t.Error("esc did not close the prompt")
	}
	if cmd != nil {
		t.Error("esc still added the feed")
	}
}

func TestAddFeedIgnoresBlankURL(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	m, _ = step(t, m, press("a"))
	m = typeText(t, m, "   ")
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("blank input tried to add a feed")
	}
	if m.input != inputNone {
		t.Error("prompt stayed open")
	}
}

func TestAddFeedTypingDoesNotReachTheFeedList(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"), testFeed("CBR"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	before := m.feedList.Index()
	m, _ = step(t, m, press("a"))
	m = typeText(t, m, "djad")

	if got := m.feedList.Index(); got != before {
		t.Errorf("feed cursor moved while typing: %d, want %d", got, before)
	}
	if m.confirming {
		t.Error("d opened the unfollow confirmation while typing")
	}
	if got, want := m.prompt.Value(), "djad"; got != want {
		t.Errorf("prompt value = %q, want %q", got, want)
	}
}

func TestFeedAddedRefreshesPanels(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))

	m, cmd := step(t, m, feedAddedMsg{name: "Ars Technica", created: true})

	if cmd == nil {
		t.Fatal("adding a feed did not refresh anything")
	}
	if !strings.Contains(m.status, "Added Ars Technica") {
		t.Errorf("status = %q, want it to name the feed", m.status)
	}
}

func TestAddingAnExistingFeedSaysFollowing(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))

	m, cmd := step(t, m, feedAddedMsg{name: "CBR", created: false})

	if cmd == nil {
		t.Fatal("following an existing feed did not refresh anything")
	}
	if !strings.Contains(m.status, "Now following CBR") {
		t.Errorf("status = %q, want it to say the feed already existed", m.status)
	}
}

func TestUnfollowAsksBeforeDeleting(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := feedsFocused(t, withFeeds(t, loaded(t, fullPage("a")), feed))

	m, cmd := step(t, m, press("d"))
	if !m.confirming {
		t.Fatal("d did not ask for confirmation")
	}
	if cmd != nil {
		t.Error("d deleted without waiting for an answer")
	}
	if !strings.Contains(m.View(), "Unfollow BBC Sport?") {
		t.Errorf("confirmation not shown:\n%s", m.View())
	}

	m, cmd = step(t, m, press("y"))
	if m.confirming {
		t.Error("confirmation stayed open")
	}
	if cmd == nil {
		t.Fatal("y did not unfollow")
	}
}

func TestUnfollowCancelled(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := feedsFocused(t, withFeeds(t, loaded(t, fullPage("a")), feed))

	m, _ = step(t, m, press("d"))
	m, cmd := step(t, m, press("n"))

	if m.confirming {
		t.Error("confirmation stayed open")
	}
	if cmd == nil {
		t.Fatal("no status command")
	}
	if !strings.Contains(m.status, "Cancelled") {
		t.Errorf("status = %q, want a cancellation notice", m.status)
	}
}

func TestUnfollowRefusedOnAllFeeds(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	m, cmd := step(t, m, press("d"))

	if m.confirming {
		t.Error("asked to unfollow the All feeds entry")
	}
	if cmd == nil {
		t.Fatal("no status command")
	}
	if !strings.Contains(m.status, "Pick a feed") {
		t.Errorf("status = %q, want an explanation", m.status)
	}
}

func TestUnfollowingTheActiveFeedFallsBackToAll(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := feedsFocused(t, withFeeds(t, loaded(t, fullPage("a")), feed))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.feedID != feed.FeedID {
		t.Fatal("feed filter was not applied")
	}

	m, cmd := step(t, m, feedUnfollowMsg{name: "BBC Sport"})

	if m.feedID != uuid.Nil {
		t.Error("unfollowing the active feed left it as the filter")
	}
	if got, want := m.list.Title, postsTitle; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if cmd == nil {
		t.Error("unfollow did not refresh the panels")
	}
}

func TestUnfollowingAnotherFeedKeepsTheFilter(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := feedsFocused(t, withFeeds(t, loaded(t, fullPage("a")), feed, testFeed("CBR")))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, feedUnfollowMsg{name: "CBR"})

	if m.feedID != feed.FeedID {
		t.Error("unfollowing another feed cleared the active filter")
	}
	if got, want := m.list.Title, "BBC Sport"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestManageKeysAreFeedPanelOnly(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))

	for _, k := range []string{"a", "d"} {
		next, _ := step(t, m, press(k))
		if next.input != inputNone {
			t.Errorf("%q opened a prompt from the post list", k)
		}
		if next.confirming {
			t.Errorf("%q opened a confirmation from the post list", k)
		}
	}
}

func TestFeedPanelHelpListsManageKeys(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})

	view := m.View()
	for _, want := range []string{"a add feed", "d unfollow"} {
		if !strings.Contains(view, want) {
			t.Errorf("feed hints missing %q:\n%s", want, view)
		}
	}
}
