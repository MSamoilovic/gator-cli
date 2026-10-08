package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"

	tea "github.com/charmbracelet/bubbletea"
)

func backed(t *testing.T, titles ...string) (model, *storetest.Fake) {
	t.Helper()

	f := storetest.New("marko")
	posts := make([]store.Post, len(titles))
	for i, title := range titles {
		p := testPost(title)
		f.AllPosts = append(f.AllPosts, p)
		posts[i] = p
	}

	m, _ := step(t, newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: posts})
	return m, f
}

func collect[T tea.Msg](cmd tea.Cmd, out chan<- T) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			go collect(c, out)
		}
	case T:
		select {
		case out <- msg:
		default:
		}
	}
}

func pressing[T tea.Msg](t *testing.T, m model, key string) (model, T) {
	t.Helper()

	next, cmd := step(t, m, press(key))
	found := make(chan T, 1)
	go collect(cmd, found)

	var want T
	select {
	case got := <-found:
		return next, got
	case <-time.After(2 * time.Second):
		t.Fatalf("pressing %q never produced a %T", key, want)
		return next, want
	}
}

func TestInitLoadsEverythingTheFirstScreenNeeds(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Go 1.26 is out", "https://go.dev/blog/go1.26")
	f.Sub("Hacker News", "https://news.ycombinator.com/rss", "tech")
	f.Unread = []store.UnreadCount{{FeedID: f.Subs[0].FeedID, Unread: 3}}

	m := newModel(t.Context(), f, f.Identity, uiState{SortDir: sortDesc})
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init returned no command")
	}

	if len(f.Asked) != 0 {
		t.Fatalf("Init touched the store before its commands ran: %+v", f.Asked)
	}
}

func TestBookmarkKeyRoundTripsThroughTheStore(t *testing.T) {
	m, f := backed(t, "Go 1.26 is out")
	post, ok := m.currentPost()
	if !ok {
		t.Fatal("no post selected")
	}

	m, toggled := pressing[bookmarkToggledMsg](t, m, "b")
	if !toggled.bookmarked || toggled.postID != post.ID {
		t.Errorf("got %+v, want post %s bookmarked", toggled, post.ID)
	}
	if !f.Bookmarked[post.ID] {
		t.Error("the store does not have the bookmark")
	}

	m, _ = step(t, m, toggled)
	if !m.bookmarks[post.ID] {
		t.Error("the model's bookmark map was not updated")
	}

	m, toggled = pressing[bookmarkToggledMsg](t, m, "b")
	if toggled.bookmarked {
		t.Error("pressing b twice left the post bookmarked")
	}
	if f.Bookmarked[post.ID] {
		t.Error("the store still has the bookmark")
	}
}

func TestReloadKeyAsksTheStoreForPosts(t *testing.T) {
	m, f := backed(t, "Go 1.26 is out")

	pressing[postsLoadedMsg](t, m, "r")
	if len(f.Asked) == 0 {
		t.Fatal("reload never reached the store")
	}
	if got := f.Asked[0]; got.Limit != pageSize {
		t.Errorf("store was asked for %d posts, want the page size %d", got.Limit, pageSize)
	}
}

func TestFetchKeyReportsWhatTheRefreshDid(t *testing.T) {
	m, f := backed(t, "Go 1.26 is out")
	f.Refreshed = store.RefreshResult{Feeds: 4, Saved: 7, Failed: 1, Unchanged: 2}

	_, got := pressing[scrapedMsg](t, m, "R")
	want := scrapedMsg{feeds: 4, saved: 7, failed: 1, unchanged: 2}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestStoreErrorsBecomeErrMsg(t *testing.T) {
	m, f := backed(t, "Go 1.26 is out")
	f.Err = errors.New("the store is down")

	pressing[errMsg](t, m, "r")
}

func TestFullTextKeySurfacesErrNoMoreText(t *testing.T) {
	m, f := backed(t, "Teaser only")
	post, ok := m.currentPost()
	if !ok {
		t.Fatal("no post selected")
	}
	f.FullTexts[post.ID] = "the whole article, far longer than the teaser"

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	_, full := pressing[fullTextMsg](t, m, "f")
	if full.body != f.FullTexts[post.ID] {
		t.Errorf("body was %q", full.body)
	}
}
