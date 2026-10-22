package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func TestRenderDetailHeader(t *testing.T) {
	post := database.Post{
		Title:       "Learn Go",
		Url:         "https://www.blog.boot.dev/go/learn-go-fast",
		PublishedAt: sql.NullTime{Time: time.Date(2026, 7, 27, 18, 55, 0, 0, time.UTC), Valid: true},
	}

	got := renderDetailHeader(post, "Boot.dev", 80, 0)
	for _, want := range []string{"Learn Go", "Boot.dev", "blog.boot.dev", "2026-07-27 18:55", "  0%"} {
		if !strings.Contains(got, want) {
			t.Errorf("header missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, post.Url) {
		t.Errorf("header spends a line on the full URL:\n%s — measured, 61%% of real URLs do not fit "+
			"80 columns, so that line was truncated more often than not", got)
	}
	if strings.Contains(got, "www.") {
		t.Errorf("header kept the www. prefix:\n%s", got)
	}

	source := strings.Split(got, "\n")[1]
	if w := lipgloss.Width(source); w != 80 {
		t.Errorf("source line is %d columns, want it padded to 80 so the percentage sits at the "+
			"right edge: %q", w, source)
	}
	if !strings.HasSuffix(source, "0%") {
		t.Errorf("source line = %q, want the scroll percentage last", source)
	}
}

func TestALongTitleWrapsInsteadOfBeingCut(t *testing.T) {
	long := "There were a lot of disagreements across the place: former Marathon director Joe " +
		"Ziegler on leading Bungie's extraction shooter"
	post := database.Post{Title: long, Url: "https://pcgamer.com/a"}

	got := renderDetailHeader(post, "PC Gamer", 80, 0)
	if lipgloss.Height(got) < 4 {
		t.Errorf("header is %d lines, want the title wrapped — 778 of 2647 real titles are longer "+
			"than 78 columns and the detail view is the one screen with room:\n%s", lipgloss.Height(got), got)
	}
	if !strings.Contains(strings.Join(strings.Fields(got), " "), long) {
		t.Errorf("the title lost text:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line is %d columns wide: %q", w, line)
		}
	}
}

func TestTheSourceLineDropsPartsItCannotFit(t *testing.T) {
	post := database.Post{
		Title:       "T",
		Url:         "https://some.rather.long.host.example.com/a",
		FullText:    "the whole article",
		PublishedAt: sql.NullTime{Time: time.Date(2026, 7, 27, 18, 55, 0, 0, time.UTC), Valid: true},
	}

	wide := detailSource(post, "A Feed With A Long Name", 200)
	for _, want := range []string{"A Feed With A Long Name", "some.rather.long.host.example.com", "2026-07-27 18:55", "full text"} {
		if !strings.Contains(wide, want) {
			t.Errorf("wide source = %q, want it to contain %q", wide, want)
		}
	}

	for _, budget := range []int{60, 40, 20, 5} {
		got := detailSource(post, "A Feed With A Long Name", budget)
		if !strings.Contains(got, "A Feed") {
			t.Errorf("source at budget %d = %q, want the feed name kept last of all", budget, got)
		}
		if budget >= 40 && lipgloss.Width(got) > budget {
			t.Errorf("source at budget %d is %d columns: %q", budget, lipgloss.Width(got), got)
		}
	}
}

func TestTheDetailScreenFitsTheTerminal(t *testing.T) {
	m := loaded(t, fullPage("a"))
	for _, title := range []string{"Short", strings.Repeat("long title ", 30)} {
		post := database.Post{ID: uuid.New(), Title: title, Url: "https://example.com/x",
			Description: sql.NullString{String: strings.Repeat("body ", 400), Valid: true}}
		m.list.SetItem(0, postItem{post: post, reads: m.reads, bookmarks: m.bookmarks, feedNames: m.feedNames})
		m.list.Select(0)

		next, _ := step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		view := next.detailView()
		if h := lipgloss.Height(view); h > 24 {
			t.Errorf("detail view is %d lines tall at 80x24 with title %q", h, title)
		}
		if w := lipgloss.Width(view); w > 80 {
			t.Errorf("detail view is %d columns wide with title %q", w, title)
		}
	}
}

func TestRenderDetailHeaderUnknownDate(t *testing.T) {
	got := renderDetailHeader(database.Post{Title: "T", Url: "https://example.com/a"}, "Feed", 80, 0)
	if !strings.Contains(got, "no date") {
		t.Errorf("missing the dateless fallback:\n%s", got)
	}
}

func TestRenderDetailBody(t *testing.T) {
	cases := []struct {
		name string
		desc sql.NullString
		want string
	}{
		{
			name: "html is stripped",
			desc: sql.NullString{String: "<p>Body &amp; soul</p>", Valid: true},
			want: "Body & soul",
		},
		{
			name: "null description",
			desc: sql.NullString{},
			want: "(no description)",
		},
		{
			name: "markup only description",
			desc: sql.NullString{String: "<p></p>", Valid: true},
			want: "(no description)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderDetailBody(database.Post{Description: tc.desc}, 80)
			if !strings.Contains(got, tc.want) {
				t.Errorf("body = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

func TestRenderDetailBodyWraps(t *testing.T) {
	desc := sql.NullString{String: strings.Repeat("rec ", 100), Valid: true}

	got := renderDetailBody(database.Post{Description: desc}, 20)
	for _, line := range strings.Split(got, "\n") {
		if lipgloss.Width(line) > 20 {
			t.Fatalf("line longer than width: %q", line)
		}
	}
	if !strings.Contains(got, "\n") {
		t.Fatal("long description was not wrapped")
	}
}

func TestPostItemNamesItsFeedAndNotItsURL(t *testing.T) {
	feedID := uuid.New()
	post := database.Post{
		ID:          uuid.New(),
		Title:       "Learn Go",
		Url:         "https://blog.boot.dev/go",
		FeedID:      feedID,
		PublishedAt: sql.NullTime{Time: time.Now().Add(-2 * time.Hour), Valid: true},
	}
	item := postItem{
		post:      post,
		reads:     map[uuid.UUID]bool{post.ID: true},
		feedNames: map[uuid.UUID]string{feedID: "Boot.dev"},
	}

	if got, want := item.Title(), "    2h Learn Go"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := item.Description(), "Boot.dev"; got != want {
		t.Errorf("Description() = %q, want %q — the URL was always truncated and the feed never named", got, want)
	}
	if strings.Contains(item.Description(), post.Url) {
		t.Error("the description still spends the second row on a URL the detail view already shows")
	}

	unknown := postItem{post: post, reads: map[uuid.UUID]bool{post.ID: true}}
	if got := unknown.Description(); got != "" {
		t.Errorf("Description() before feeds load = %q, want empty", got)
	}
}

func TestPostItemFilterValueMatchesItsTitle(t *testing.T) {
	post := database.Post{ID: uuid.New(), Title: "Learn Go"}
	item := postItem{post: post, reads: map[uuid.UUID]bool{}, bookmarks: map[uuid.UUID]bool{}}

	if got, want := item.FilterValue(), item.Title(); got != want {
		t.Errorf("FilterValue() = %q, Title() = %q — list highlights matches of the filter value "+
			"on the title, so anything else underlines the wrong characters", got, want)
	}
}

func TestPostItemMarkers(t *testing.T) {
	post := database.Post{ID: uuid.New(), Title: "Learn Go"}

	cases := []struct {
		name     string
		read     bool
		bookmark bool
		want     string
	}{
		{"unread", false, false, "●    - Learn Go"},
		{"unread and saved", false, true, "●★   - Learn Go"},
		{"read and saved", true, true, " ★   - Learn Go"},
		{"read", true, false, "     - Learn Go"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := postItem{
				post:      post,
				reads:     map[uuid.UUID]bool{post.ID: tc.read},
				bookmarks: map[uuid.UUID]bool{post.ID: tc.bookmark},
			}
			if got := item.Title(); got != tc.want {
				t.Errorf("Title() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEveryPostStartsItsTitleInTheSameColumn(t *testing.T) {
	post := database.Post{ID: uuid.New(), Title: "Learn Go"}

	var columns []int
	for _, read := range []bool{false, true} {
		for _, saved := range []bool{false, true} {
			for _, age := range []time.Duration{time.Minute, 20 * time.Hour, 900 * 24 * time.Hour} {
				p := post
				p.PublishedAt = sql.NullTime{Time: time.Now().Add(-age), Valid: true}
				item := postItem{
					post:      p,
					reads:     map[uuid.UUID]bool{post.ID: read},
					bookmarks: map[uuid.UUID]bool{post.ID: saved},
				}
				title := item.Title()
				columns = append(columns, lipgloss.Width(title[:strings.Index(title, post.Title)]))
			}
		}
	}

	for _, got := range columns {
		if got != columns[0] {
			t.Fatalf("titles start at columns %v — a marker or an age that changes width makes the "+
				"whole list jump sideways as posts are read", columns)
		}
	}
}

func TestFeedItemUnreadCount(t *testing.T) {
	id := uuid.New()
	counts := map[uuid.UUID]int{id: 12}

	if got, want := (feedItem{id: id, name: "BBC Sport", unread: counts}).Title(), "BBC Sport (12)"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := (feedItem{id: uuid.New(), name: "CBR", unread: counts}).Title(), "CBR"; got != want {
		t.Errorf("Title() with no unread = %q, want %q", got, want)
	}
	if got, want := (feedItem{id: id, name: "BBC Sport", unread: counts}).FilterValue(), "BBC Sport"; got != want {
		t.Errorf("FilterValue() = %q, want the bare name %q", got, want)
	}
}
