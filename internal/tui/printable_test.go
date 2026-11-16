package tui

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"

	tea "github.com/charmbracelet/bubbletea"
)

const smuggled = "\x1b]52;c;cm0gLXJmIH4=\x07\x1b[2J\u009b6n‮"

func carriesSmuggledControls(s string) bool {
	return strings.Contains(s, "\x1b]") ||
		strings.Contains(s, "\x1b[2J") ||
		strings.ContainsAny(s, "\x07\u009b‮")
}

func TestNothingFromTheStoreReachesTheScreenWithItsControlCharacters(t *testing.T) {
	f := storetest.New("marko" + smuggled)
	sub := f.Sub("Evil feed"+smuggled, "https://evil.example/feed", "Folder"+smuggled)
	post := testPost("Free money" + smuggled)
	post.FeedID = sub.FeedID
	post.Description = sql.NullString{String: "<p>body" + smuggled + "</p>", Valid: true}
	post.FullText = "full" + smuggled
	f.AllPosts = []store.Post{post}
	f.FullTexts[post.ID] = "fetched" + smuggled

	m, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}, ""), tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = step(t, m, loadFeeds(t.Context(), f)())
	m, _ = step(t, m, loadPosts(t.Context(), f, m.filter.params(), 0)())

	if got := m.View(); carriesSmuggledControls(got) {
		t.Errorf("post list renders a smuggled control sequence:\n%q", got)
	}

	m, _ = step(t, m, press("enter"))
	if m.screen != screenDetail {
		t.Fatalf("enter did not open the post, screen = %v", m.screen)
	}
	if got := m.View(); carriesSmuggledControls(got) {
		t.Errorf("detail renders a smuggled control sequence:\n%q", got)
	}
	if got := articleText(m.selected, "Evil feed", 80); carriesSmuggledControls(got) {
		t.Errorf("text handed to the pager carries a smuggled control sequence:\n%q", got)
	}

	m, _ = step(t, m, fetchFullText(t.Context(), f, m.selected)())
	if got := m.View(); carriesSmuggledControls(got) {
		t.Errorf("full text renders a smuggled control sequence:\n%q", got)
	}

	m, _ = step(t, m, errMsg{errors.New("server says" + smuggled)})
	if got := m.View(); carriesSmuggledControls(got) {
		t.Errorf("an error renders a smuggled control sequence:\n%q", got)
	}
}

func TestCleaningPostsLeavesTheStoresSliceAlone(t *testing.T) {
	posts := []store.Post{testPost("a" + smuggled)}
	printablePosts(posts)
	if !strings.Contains(posts[0].Title, "\x1b") {
		t.Error("printablePosts rewrote the slice it was handed; the offline cache holds that slice")
	}
}

func TestOnlyWebLinksAreOpenedOrCopied(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://go.dev/blog", "https://go.dev/blog"},
		{"http://example.com/a?b=c", "http://example.com/a?b=c"},
		{"\thttps://example.com/tabbed", "https://example.com/tabbed"},
		{"file:///etc/passwd", ""},
		{"javascript:alert(1)", ""},
		{"-a", ""},
		{"--help", ""},
		{"/relative/path", ""},
		{"https:///no-host", ""},
		{"", ""},
	}

	for _, c := range cases {
		got, err := webLink(c.raw)
		switch {
		case c.want == "" && err == nil:
			t.Errorf("webLink(%q) = %q, want it refused", c.raw, got)
		case c.want != "" && (err != nil || got != c.want):
			t.Errorf("webLink(%q) = %q, %v; want %q", c.raw, got, err, c.want)
		}
	}

	var launched []string
	startBrowser = func(link string) error {
		launched = append(launched, "open "+link)
		return nil
	}
	writeClipboard = func(link string) error {
		launched = append(launched, "copy "+link)
		return nil
	}

	if _, ok := openInBrowser("file:///etc/passwd")().(errMsg); !ok {
		t.Error("openInBrowser accepted a file: link")
	}
	if _, ok := copyToClipboard("javascript:alert(1)")().(errMsg); !ok {
		t.Error("copyToClipboard accepted a javascript: link")
	}
	if len(launched) != 0 {
		t.Errorf("a refused link still reached the system: %q", launched)
	}

	if _, ok := openInBrowser(" https://go.dev/blog")().(openedMsg); !ok {
		t.Error("openInBrowser refused a web link")
	}
	if want := []string{"open https://go.dev/blog"}; len(launched) != 1 || launched[0] != want[0] {
		t.Errorf("launched = %q, want %q", launched, want)
	}
}
