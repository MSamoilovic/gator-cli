package tui

import (
	"strings"
	"testing"
	"time"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTimeRangeCycles(t *testing.T) {
	m := loaded(t, fullPage("a"))

	want := []struct {
		since time.Duration
		title string
	}{
		{24 * time.Hour, "Posts · 24h"},
		{7 * 24 * time.Hour, "Posts · 7d"},
		{30 * 24 * time.Hour, "Posts · 30d"},
		{0, postsTitle},
	}

	for _, step2 := range want {
		var cmd tea.Cmd
		m, cmd = step(t, m, press("t"))

		if m.since != step2.since {
			t.Fatalf("since = %v, want %v", m.since, step2.since)
		}
		if m.list.Title != step2.title {
			t.Errorf("title = %q, want %q", m.list.Title, step2.title)
		}
		if cmd == nil {
			t.Error("time range change did not reload")
		}
	}
}

func TestTimeRangeResetsPagingAndBuildsFilter(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m.offset = pageSize * 2

	m, _ = step(t, m, press("t"))

	if m.offset != 0 {
		t.Errorf("offset = %d, want paging reset", m.offset)
	}

	f := m.filter()
	if f.since.IsZero() {
		t.Fatal("filter carries no cutoff")
	}
	if d := time.Since(f.since); d < 23*time.Hour || d > 25*time.Hour {
		t.Errorf("cutoff is %v ago, want roughly 24h", d)
	}
}

func TestNoTimeRangeMeansZeroCutoff(t *testing.T) {
	m := loaded(t, fullPage("a"))

	if !m.filter().since.IsZero() {
		t.Error("an unset time range must leave the cutoff zero")
	}
}

func TestTimeRangeKeepsFeedAndSort(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, loaded(t, fullPage("a")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, press("t"))

	f := m.filter()
	if f.feedID != feed.FeedID {
		t.Error("time range dropped the feed filter")
	}
	if got, want := m.list.Title, "BBC Sport · 24h"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestTimeRangeRefusedInDerivedViews(t *testing.T) {
	m, _ := step(t, loaded(t, fullPage("a")), press("B"))

	m, _ = step(t, m, press("t"))

	if m.since != 0 {
		t.Error("time range changed inside the bookmarks view")
	}
	if !strings.Contains(m.status, "feed posts only") {
		t.Errorf("status = %q, want an explanation", m.status)
	}
}

func TestUnreadFilterAndTimeRangeCombine(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, press("U"))
	m, _ = step(t, m, press("t"))

	f := m.filter()
	if !f.unreadOnly {
		t.Error("unread filter was lost")
	}
	if f.since.IsZero() {
		t.Error("time range was lost")
	}
}

func TestCopyURLFromList(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	_, cmd := step(t, m, press("y"))
	if cmd == nil {
		t.Fatal("y did not copy")
	}
}

func TestCopyURLDoesNothingOnEmptyList(t *testing.T) {
	m := loaded(t, nil)

	_, cmd := step(t, m, press("y"))
	if cmd != nil {
		t.Error("y copied with no post selected")
	}
}

func TestCopiedStatus(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, copiedMsg{url: "https://example.com/x"})

	if !strings.Contains(m.status, "https://example.com/x") {
		t.Errorf("status = %q, want the copied URL", m.status)
	}
}

func TestNextAndPrevMoveThroughPostsInDetail(t *testing.T) {
	posts := []database.Post{testPost("Prvi"), testPost("Drugi"), testPost("Treci")}
	m := loaded(t, posts)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got, want := m.selected.Title, "Prvi"; got != want {
		t.Fatalf("selected = %q, want %q", got, want)
	}

	m, cmd := step(t, m, press("n"))
	if got, want := m.selected.Title, "Drugi"; got != want {
		t.Errorf("after n selected = %q, want %q", got, want)
	}
	if m.screen != screenDetail {
		t.Error("n closed the detail view")
	}
	if cmd == nil {
		t.Error("n did not mark the new post read")
	}
	if got, want := m.list.Index(), 1; got != want {
		t.Errorf("list index = %d, want %d — the list must follow along", got, want)
	}

	m, _ = step(t, m, press("p"))
	if got, want := m.selected.Title, "Prvi"; got != want {
		t.Errorf("after p selected = %q, want %q", got, want)
	}
}

func TestNextAndPrevStopAtTheEnds(t *testing.T) {
	posts := []database.Post{testPost("Prvi"), testPost("Drugi")}
	m := loaded(t, posts)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m, cmd := step(t, m, press("p"))
	if cmd != nil {
		t.Error("p moved past the first post")
	}
	if got, want := m.selected.Title, "Prvi"; got != want {
		t.Errorf("selected = %q, want %q", got, want)
	}

	m, _ = step(t, m, press("n"))
	m, cmd = step(t, m, press("n"))
	if cmd != nil {
		t.Error("n moved past the last post")
	}
	if got, want := m.selected.Title, "Drugi"; got != want {
		t.Errorf("selected = %q, want %q", got, want)
	}
}

func TestNextRefreshesTheDetailBody(t *testing.T) {
	posts := []database.Post{testPost("Prvi"), testPost("Drugi")}
	m := loaded(t, posts)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, press("n"))

	view := m.View()
	if !strings.Contains(view, "Telo posta Drugi") {
		t.Errorf("detail body was not refreshed:\n%s", view)
	}
	if strings.Contains(view, "Telo posta Prvi") {
		t.Errorf("previous post still rendered:\n%s", view)
	}
}

func TestDetailHelpListsReadingKeys(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	for _, want := range []string{"n next", "p prev", "y copy url"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail hints missing %q:\n%s", want, view)
		}
	}
}
