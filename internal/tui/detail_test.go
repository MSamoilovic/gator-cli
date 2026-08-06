package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"gator-cli/internal/database"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func TestStripHTML(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "paragraphs become blank lines",
			input: `<p>Article URL: <a href="http://x">http://x</a></p><p>Points: 42</p>`,
			want:  "Article URL: http://x\n\nPoints: 42",
		},
		{
			name:  "entities are decoded",
			input: "Go &amp; Rust &lt;3 &quot;fast&quot;",
			want:  `Go & Rust <3 "fast"`,
		},
		{
			name:  "br becomes newline",
			input: "prvi<br/>drugi",
			want:  "prvi\ndrugi",
		},
		{
			name:  "attributes on block tags are handled",
			input: `<div class="a b"><p style="x">tekst</p></div>`,
			want:  "tekst",
		},
		{
			name:  "inline tags are dropped without spacing damage",
			input: "<b>bold</b><i>italic</i>",
			want:  "bolditalic",
		},
		{
			name:  "plain text passes through",
			input: "nema tagova",
			want:  "nema tagova",
		},
		{
			name:  "only markup collapses to empty",
			input: "<p></p>",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripHTML(tc.input); got != tc.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRenderDetailHeader(t *testing.T) {
	post := database.Post{
		Title:       "Learn Go",
		Url:         "https://blog.boot.dev/go",
		PublishedAt: sql.NullTime{Time: time.Date(2026, 7, 27, 18, 55, 0, 0, time.UTC), Valid: true},
	}

	got := renderDetailHeader(post, 80)
	for _, want := range []string{"Learn Go", "https://blog.boot.dev/go", "Published: 2026-07-27 18:55"} {
		if !strings.Contains(got, want) {
			t.Errorf("header missing %q:\n%s", want, got)
		}
	}

	// Header mora biti tacno detailChromeHeight - 2 reda da bi racunica
	// visine viewport-a bila tacna.
	if n := strings.Count(got, "\n"); n != detailChromeHeight-2 {
		t.Errorf("header = %d newlines, want %d", n, detailChromeHeight-2)
	}
}

func TestRenderDetailHeaderTruncatesLongLines(t *testing.T) {
	post := database.Post{
		Title: strings.Repeat("x", 200),
		Url:   strings.Repeat("y", 200),
	}

	got := renderDetailHeader(post, 40)
	if n := strings.Count(got, "\n"); n != detailChromeHeight-2 {
		t.Fatalf("long title wrapped: %d newlines, want %d", n, detailChromeHeight-2)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("line longer than width: %d columns", w)
		}
	}
}

func TestRenderDetailHeaderUnknownDate(t *testing.T) {
	got := renderDetailHeader(database.Post{Title: "T", Url: "U"}, 80)
	if !strings.Contains(got, "Published: unknown") {
		t.Errorf("missing unknown date fallback:\n%s", got)
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

func TestPostItem(t *testing.T) {
	published := database.Post{
		ID:          uuid.New(),
		Title:       "Learn Go",
		Url:         "https://blog.boot.dev/go",
		PublishedAt: sql.NullTime{Time: time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	undated := database.Post{Title: "Learn Go", Url: "https://blog.boot.dev/go"}
	read := map[uuid.UUID]bool{published.ID: true}

	if got, want := (postItem{post: published, reads: read}).Title(), "Learn Go"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := (postItem{post: published, reads: read}).FilterValue(), "Learn Go"; got != want {
		t.Errorf("FilterValue() = %q, want %q", got, want)
	}
	if got, want := (postItem{post: published, reads: read}).Description(), "2026-07-27 · https://blog.boot.dev/go"; got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
	if got, want := (postItem{post: undated, reads: read}).Description(), "https://blog.boot.dev/go"; got != want {
		t.Errorf("Description() without date = %q, want %q", got, want)
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
		{"unread", false, false, "● Learn Go"},
		{"unread and saved", false, true, "●★ Learn Go"},
		{"read and saved", true, true, "★ Learn Go"},
		{"read", true, false, "Learn Go"},
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
