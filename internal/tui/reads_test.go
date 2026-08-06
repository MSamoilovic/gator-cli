package tui

import (
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func postInFeed(title string, feedID uuid.UUID) database.Post {
	p := testPost(title)
	p.FeedID = feedID
	return p
}

func TestReadingAPostMarksItRead(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	if m.reads[post.ID] {
		t.Fatal("post read before it was opened")
	}

	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.reads[post.ID] {
		t.Error("opening a post did not mark it read")
	}
	if cmd == nil {
		t.Error("read state was not persisted")
	}
	if got := m.list.Items()[0].(postItem).Title(); strings.Contains(got, "●") {
		t.Errorf("title = %q, want the unread dot gone", got)
	}
}

func TestReopeningAReadPostSkipsTheWrite(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("reopening an already read post wrote to the database again")
	}
}

func TestUnreadTogglesBothWays(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	m, cmd := step(t, m, press("u"))
	if !m.reads[post.ID] {
		t.Fatal("u did not mark the post read")
	}
	if cmd == nil {
		t.Error("u did not persist")
	}
	if !strings.Contains(m.status, "Marked read") {
		t.Errorf("status = %q, want it to say read", m.status)
	}

	m, _ = step(t, m, press("u"))
	if m.reads[post.ID] {
		t.Error("u did not mark the post unread again")
	}
	if !strings.Contains(m.status, "Marked unread") {
		t.Errorf("status = %q, want it to say unread", m.status)
	}
	if got := m.list.Items()[0].(postItem).Title(); !strings.Contains(got, "●") {
		t.Errorf("title = %q, want the unread dot back", got)
	}
}

func TestUnreadCountFollowsReadState(t *testing.T) {
	feed := testFeed("BBC Sport")
	post := postInFeed("Prvi", feed.FeedID)

	m := withFeeds(t, loaded(t, []database.Post{post}), feed)
	m, _ = step(t, m, unreadCountsMsg{
		counts: []database.GetUnreadCountsForUserRow{{FeedID: feed.FeedID, Unread: 3}},
	})

	if got := m.feedList.Items()[1].(feedItem).Title(); !strings.Contains(got, "(3)") {
		t.Fatalf("feed title = %q, want the unread count", got)
	}

	m, _ = step(t, m, press("u"))
	if got, want := m.unread[feed.FeedID], 2; got != want {
		t.Errorf("unread = %d, want %d after marking one read", got, want)
	}

	m, _ = step(t, m, press("u"))
	if got, want := m.unread[feed.FeedID], 3; got != want {
		t.Errorf("unread = %d, want %d after marking it unread again", got, want)
	}
}

func TestUnreadCountNeverGoesNegative(t *testing.T) {
	feed := testFeed("BBC Sport")
	post := postInFeed("Prvi", feed.FeedID)
	m := withFeeds(t, loaded(t, []database.Post{post}), feed)

	m, _ = step(t, m, press("u"))

	if got := m.unread[feed.FeedID]; got < 0 {
		t.Errorf("unread = %d, want it clamped at 0", got)
	}
}

func TestMarkAllReadMarksOnlyUnreadOnes(t *testing.T) {
	feed := testFeed("BBC Sport")
	posts := []database.Post{
		postInFeed("Prvi", feed.FeedID),
		postInFeed("Drugi", feed.FeedID),
		postInFeed("Treci", feed.FeedID),
	}
	m := withFeeds(t, loaded(t, posts), feed)
	m, _ = step(t, m, unreadCountsMsg{
		counts: []database.GetUnreadCountsForUserRow{{FeedID: feed.FeedID, Unread: 3}},
	})

	m, _ = step(t, m, press("u")) // prvi je sad procitan

	m, cmd := step(t, m, press("A"))
	if cmd == nil {
		t.Fatal("A did not persist")
	}
	for _, p := range posts {
		if !m.reads[p.ID] {
			t.Errorf("post %q still unread", p.Title)
		}
	}
	if got, want := m.unread[feed.FeedID], 0; got != want {
		t.Errorf("unread = %d, want %d", got, want)
	}

	m, _ = step(t, m, allReadMsg{count: 2})
	if !strings.Contains(m.status, "2 posts read") {
		t.Errorf("status = %q, want the marked count", m.status)
	}
}

func TestMarkAllReadOnEmptyListSaysSo(t *testing.T) {
	m := loaded(t, nil)

	m, _ = step(t, m, allReadMsg{})

	if !strings.Contains(m.status, "Nothing to mark") {
		t.Errorf("status = %q, want an explanation", m.status)
	}
}

func TestUnreadOnlyFilterToggles(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, cmd := step(t, m, press("U"))
	if !m.unreadOnly {
		t.Fatal("U did not turn the unread filter on")
	}
	if cmd == nil {
		t.Error("U did not reload")
	}
	if !strings.Contains(m.status, "unread only") {
		t.Errorf("status = %q, want it to name the filter", m.status)
	}
	if m.offset != 0 {
		t.Errorf("offset = %d, want paging reset", m.offset)
	}

	m, _ = step(t, m, press("U"))
	if m.unreadOnly {
		t.Error("U did not turn the unread filter off")
	}
	if !strings.Contains(m.status, "all posts") {
		t.Errorf("status = %q, want it to name the filter", m.status)
	}
}

func TestUnreadFilterRefusedInDerivedViews(t *testing.T) {
	m, _ := step(t, loaded(t, fullPage("a")), press("B"))

	m, _ = step(t, m, press("U"))

	if m.unreadOnly {
		t.Error("unread filter turned on inside the bookmarks view")
	}
	if !strings.Contains(m.status, "feed posts only") {
		t.Errorf("status = %q, want an explanation", m.status)
	}
}

func TestMarkAllReadReloadsUnderUnreadFilter(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, press("U"))

	m, cmd := step(t, m, allReadMsg{count: 5})

	if cmd == nil {
		t.Fatal("marking all read did not refresh the unread-only list")
	}
	if !m.unreadOnly {
		t.Error("unread filter was lost")
	}
}

func TestReadsLoadedMarksExistingItems(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	if got := m.list.Items()[0].(postItem).Title(); !strings.Contains(got, "●") {
		t.Fatalf("title = %q, want an unread dot before reads load", got)
	}

	m, _ = step(t, m, readsLoadedMsg{postIDs: []uuid.UUID{post.ID}})

	if got := m.list.Items()[0].(postItem).Title(); strings.Contains(got, "●") {
		t.Errorf("title = %q, want the dot gone once reads loaded", got)
	}
}
