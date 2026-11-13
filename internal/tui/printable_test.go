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
