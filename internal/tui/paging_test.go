package tui

import (
	"strconv"
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func fullPage(prefix string) []database.Post {
	posts := make([]database.Post, pageSize)
	for i := range posts {
		posts[i] = testPost(prefix + strconv.Itoa(i))
	}
	return posts
}

func loaded(t *testing.T, posts []database.Post) model {
	t.Helper()
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: posts, paged: true})
	return m
}

func TestFullPageMeansMore(t *testing.T) {
	if m := loaded(t, fullPage("a")); !m.hasMore {
		t.Error("a full page should leave hasMore true")
	}
	if m := loaded(t, fullPage("a")[:pageSize-1]); m.hasMore {
		t.Error("a short page should clear hasMore")
	}
}

func TestScrollingToBottomLoadsNextPage(t *testing.T) {
	m := loaded(t, fullPage("a"))

	var cmd tea.Cmd
	for range pageSize {
		m, cmd = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
		if cmd != nil {
			break
		}
	}

	if cmd == nil {
		t.Fatal("reaching the bottom did not request another page")
	}
	if got, want := m.offset, int32(pageSize); got != want {
		t.Errorf("offset = %d, want %d", got, want)
	}
	if !m.loadingMore {
		t.Error("loadingMore not set while a page is in flight")
	}
}

func TestNextPageIsAppended(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m.offset = pageSize
	m.loadingMore = true

	m, _ = step(t, m, postsLoadedMsg{posts: fullPage("b")[:10], offset: pageSize, paged: true})

	if got, want := len(m.list.Items()), pageSize+10; got != want {
		t.Errorf("items = %d, want %d", got, want)
	}
	if m.loadingMore {
		t.Error("loadingMore still set after the page arrived")
	}
	if m.hasMore {
		t.Error("short page should clear hasMore")
	}
}

func TestStalePageIsIgnored(t *testing.T) {
	m := loaded(t, fullPage("a"))

	// Korisnik je promenio feed dok je strana bila u letu: offset se vratio na 0.
	m, _ = step(t, m, postsLoadedMsg{posts: fullPage("b")[:5], offset: pageSize, paged: true})

	if got, want := len(m.list.Items()), pageSize; got != want {
		t.Errorf("stale page was applied: items = %d, want %d", got, want)
	}
}

func TestSearchResultsDoNotPaginate(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "golang")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, postsLoadedMsg{posts: fullPage("b")})

	if m.hasMore {
		t.Error("search results must not advertise more pages")
	}

	var cmd tea.Cmd
	for range pageSize {
		m, cmd = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
		if cmd != nil {
			t.Fatal("scrolling search results requested another page")
		}
	}
}

func TestFeedSwitchResetsPaging(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, loaded(t, fullPage("a")), feed)
	m.offset = pageSize * 2
	m.hasMore = false

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("feed switch did not reload")
	}
	if m.offset != 0 {
		t.Errorf("offset = %d, want 0 after a feed switch", m.offset)
	}
	if !m.hasMore {
		t.Error("hasMore should be optimistic again after a feed switch")
	}
}

func TestEmptyStateExplainsWhy(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T) model
		want  string
	}{
		{
			name: "no feeds followed",
			setup: func(t *testing.T) model {
				return withFeeds(t, loaded(t, nil))
			},
			want: "not following any feeds",
		},
		{
			name: "feeds but no posts",
			setup: func(t *testing.T) model {
				return withFeeds(t, loaded(t, nil), testFeed("BBC Sport"))
			},
			want: "No posts yet",
		},
		{
			name: "no search results",
			setup: func(t *testing.T) model {
				m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))
				m, _ = step(t, m, press("s"))
				m = typeText(t, m, "nepostojece")
				m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
				m, _ = step(t, m, postsLoadedMsg{})
				return m
			},
			want: `No posts match "nepostojece"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(t)

			view := m.View()
			if !strings.Contains(view, tc.want) {
				t.Errorf("empty state missing %q:\n%s", tc.want, view)
			}
			if got, want := strings.Count(view, "\n")+1, 24; got != want {
				t.Errorf("view = %d lines, want %d", got, want)
			}
			for _, line := range strings.Split(view, "\n") {
				if w := lipgloss.Width(line); w > 80 {
					t.Errorf("line wider than terminal: %d columns", w)
				}
			}
		})
	}
}

func TestEmptyStateSaysNothingBeforeFeedsLoad(t *testing.T) {
	m := loaded(t, nil)

	if strings.Contains(m.View(), "not following any feeds") {
		t.Error("claimed no feeds before the feed list had loaded")
	}
}

func TestToggleFullHelpKeepsLayoutExact(t *testing.T) {
	m := withFeeds(t, loaded(t, fullPage("a")), testFeed("BBC Sport"))

	short := m.footerHeight()
	if short != 1 {
		t.Fatalf("short footer = %d lines, want 1", short)
	}

	m, _ = step(t, m, press("?"))
	if !m.help.ShowAll {
		t.Fatal("? did not expand the help")
	}
	if m.footerHeight() <= 1 {
		t.Fatalf("full footer = %d lines, want more than 1", m.footerHeight())
	}

	view := m.View()
	if got, want := strings.Count(view, "\n")+1, 24; got != want {
		t.Errorf("expanded view = %d lines, want %d", got, want)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line wider than terminal: %d columns", w)
		}
	}

	m, _ = step(t, m, press("?"))
	if m.help.ShowAll {
		t.Error("? did not collapse the help again")
	}
	if got, want := strings.Count(m.View(), "\n")+1, 24; got != want {
		t.Errorf("collapsed view = %d lines, want %d", got, want)
	}
}

func TestFullHelpInDetailKeepsLayoutExact(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, press("?"))

	if m.screen != screenDetail {
		t.Fatal("? closed the detail view")
	}
	if got, want := strings.Count(m.View(), "\n")+1, 24; got != want {
		t.Errorf("detail view with full help = %d lines, want %d", got, want)
	}
}
