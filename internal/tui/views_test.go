package tui

import (
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func TestSavedTogglesBookmarksView(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, cmd := step(t, m, press("B"))
	if !m.showBookmarks {
		t.Fatal("B did not open the bookmarks view")
	}
	if cmd == nil {
		t.Error("B did not load bookmarks")
	}
	if got, want := m.list.Title, bookmarksTitle; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}

	m, cmd = step(t, m, press("B"))
	if m.showBookmarks {
		t.Error("B did not close the bookmarks view")
	}
	if cmd == nil {
		t.Error("leaving bookmarks did not reload posts")
	}
	if got, want := m.list.Title, postsTitle; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestEscapeLeavesBookmarksView(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, loaded(t, fullPage("a")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	m, _ = step(t, m, press("B"))
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("esc did not reload after leaving bookmarks")
	}
	if m.showBookmarks {
		t.Error("esc did not leave the bookmarks view")
	}
	if got, want := m.list.Title, "BBC Sport"; got != want {
		t.Errorf("title = %q, want the feed we came from (%q)", got, want)
	}
}

func TestBookmarksViewDoesNotPaginate(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, press("B"))
	m, _ = step(t, m, postsLoadedMsg{posts: fullPage("b")})

	if m.hasMore {
		t.Error("bookmarks view must not advertise more pages")
	}

	var cmd tea.Cmd
	for range pageSize {
		m, cmd = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
		if cmd != nil {
			t.Fatal("scrolling bookmarks requested another page")
		}
	}
}

func TestUnbookmarkingInsideBookmarksViewReloads(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	m, _ = step(t, m, press("B"))
	_, cmd := step(t, m, bookmarkToggledMsg{postID: post.ID, bookmarked: false})

	if cmd == nil {
		t.Fatal("removing a bookmark inside the bookmarks view did not reload")
	}
}

func TestUnbookmarkingElsewhereDoesNotReload(t *testing.T) {
	post := testPost("Prvi")
	m := loaded(t, []database.Post{post})

	m, _ = step(t, m, bookmarksLoadedMsg{postIDs: []uuid.UUID{post.ID}})
	next, cmd := step(t, m, bookmarkToggledMsg{postID: post.ID, bookmarked: false})

	if next.showBookmarks {
		t.Fatal("unexpected bookmarks view")
	}
	if cmd == nil {
		t.Fatal("status command missing")
	}
	if got := next.list.Items()[0].(postItem).Title(); strings.Contains(got, "★") {
		t.Errorf("title = %q, want the star gone", got)
	}
}

func TestSortToggles(t *testing.T) {
	m := loaded(t, fullPage("a"))

	if got, want := m.sortDir, sortDesc; got != want {
		t.Fatalf("initial sort = %q, want %q", got, want)
	}

	m, cmd := step(t, m, press("S"))
	if got, want := m.sortDir, sortAsc; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}
	if cmd == nil {
		t.Error("sort toggle did not reload")
	}
	if !strings.Contains(m.status, "oldest first") {
		t.Errorf("status = %q, want it to name the new order", m.status)
	}
	if m.offset != 0 {
		t.Errorf("offset = %d, want paging reset after a sort change", m.offset)
	}

	m, _ = step(t, m, press("S"))
	if got, want := m.sortDir, sortDesc; got != want {
		t.Errorf("sort = %q, want %q", got, want)
	}
	if !strings.Contains(m.status, "newest first") {
		t.Errorf("status = %q, want it to name the new order", m.status)
	}
}

func TestSortRefusedInDerivedViews(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T) model
	}{
		{
			name: "bookmarks",
			setup: func(t *testing.T) model {
				m, _ := step(t, loaded(t, fullPage("a")), press("B"))
				return m
			},
		},
		{
			name: "search results",
			setup: func(t *testing.T) model {
				m, _ := step(t, loaded(t, fullPage("a")), press("s"))
				m = typeText(t, m, "golang")
				m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
				return m
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(t)
			before := m.sortDir

			m, _ = step(t, m, press("S"))

			if m.sortDir != before {
				t.Errorf("sort changed in %s view: %q", tc.name, m.sortDir)
			}
			if !strings.Contains(m.status, "feed posts only") {
				t.Errorf("status = %q, want an explanation", m.status)
			}
		})
	}
}

func TestFeedSwitchLeavesBookmarksView(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, loaded(t, fullPage("a")), feed)

	m, _ = step(t, m, press("B"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.showBookmarks {
		t.Error("selecting a feed left the bookmarks view on")
	}
	if got, want := m.list.Title, "BBC Sport"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestEmptyBookmarksExplains(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))

	m, _ = step(t, m, press("B"))
	m, _ = step(t, m, postsLoadedMsg{})

	view := m.View()
	if !strings.Contains(view, "No bookmarks yet") {
		t.Errorf("empty bookmarks view missing explanation:\n%s", view)
	}
	if got, want := strings.Count(view, "\n")+1, 24; got != want {
		t.Errorf("view = %d lines, want %d", got, want)
	}
}

func TestFooterShowsUserName(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	if got := m.footer(); !strings.Contains(got, "@marko") {
		t.Errorf("footer does not name the user: %q", got)
	}

	m, _ = step(t, m, press("enter"))
	if got := m.footer(); !strings.Contains(got, "@marko") {
		t.Errorf("detail footer does not name the user: %q", got)
	}
}

func TestUserNameYieldsToHelpWhenNarrow(t *testing.T) {
	m := ready(t, testPost("Prvi"))
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 30, Height: 24})

	if got := m.footer(); strings.Contains(got, "@marko") {
		t.Errorf("user tag pushed into a 30-column footer: %q", got)
	}

	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, press("?"))
	if got := m.footer(); strings.Contains(got, "@marko") {
		t.Errorf("user tag rendered over the full help: %q", got)
	}
}
