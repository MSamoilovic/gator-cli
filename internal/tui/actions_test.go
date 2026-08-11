package tui

import (
	"errors"
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func press(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestBookmarksLoadedMarksItems(t *testing.T) {
	post := testPost("Prvi")
	m := ready(t, post)

	if got := m.list.Items()[0].(postItem).Title(); strings.Contains(got, "★") {
		t.Fatalf("title starred before bookmarks loaded: %q", got)
	}

	m, _ = step(t, m, bookmarksLoadedMsg{postIDs: []uuid.UUID{post.ID}})

	if got := m.list.Items()[0].(postItem).Title(); !strings.Contains(got, "★") {
		t.Errorf("title = %q, want a star marker", got)
	}
}

func TestBookmarksLoadedBeforePosts(t *testing.T) {
	post := testPost("Prvi")

	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, bookmarksLoadedMsg{postIDs: []uuid.UUID{post.ID}})
	m, _ = step(t, m, postsLoadedMsg{posts: []database.Post{post}})

	if got := m.list.Items()[0].(postItem).Title(); !strings.Contains(got, "★") {
		t.Errorf("title = %q, want a star marker", got)
	}
}

func TestBookmarkToggleUpdatesStateAndStatus(t *testing.T) {
	post := testPost("Prvi")
	m := ready(t, post)

	m, _ = step(t, m, bookmarkToggledMsg{postID: post.ID, bookmarked: true})
	if !m.bookmarks[post.ID] {
		t.Fatal("post not marked as bookmarked")
	}
	if !strings.Contains(m.View(), "Bookmarked") {
		t.Errorf("status not shown:\n%s", m.View())
	}
	if got := m.list.Items()[0].(postItem).Title(); !strings.Contains(got, "★") {
		t.Errorf("title = %q, want a star marker", got)
	}

	m, _ = step(t, m, bookmarkToggledMsg{postID: post.ID, bookmarked: false})
	if m.bookmarks[post.ID] {
		t.Error("post still marked as bookmarked")
	}
	if got := m.list.Items()[0].(postItem).Title(); strings.Contains(got, "★") {
		t.Errorf("title = %q, want no star", got)
	}
}

func TestStatusExpiresOnlyForCurrentToken(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, openedMsg{url: "https://example.com"})
	stale := m.statusToken

	m, _ = step(t, m, bookmarkToggledMsg{postID: uuid.New(), bookmarked: true})
	m, _ = step(t, m, statusExpiredMsg{token: stale})

	if m.status == "" {
		t.Fatal("stale expiry cleared a newer status")
	}

	m, _ = step(t, m, statusExpiredMsg{token: m.statusToken})
	if m.status != "" {
		t.Errorf("status = %q, want empty after matching expiry", m.status)
	}
}

func TestStatusFallsBackToKeyHints(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	if got := m.View(); !strings.Contains(got, "b mark") {
		t.Errorf("list view missing key hints:\n%s", got)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.View(); !strings.Contains(got, "esc back") {
		t.Errorf("detail view missing key hints:\n%s", got)
	}
}

func TestActionErrorGoesToStatusNotErrorScreen(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, errMsg{errors.New("xdg-open pukao")})

	if m.err != nil {
		t.Error("action error replaced the whole view")
	}
	if !strings.Contains(m.View(), "xdg-open pukao") {
		t.Errorf("error not shown in status:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "Prvi") {
		t.Error("post list disappeared after an action error")
	}
}

func TestLoadErrorStillShowsErrorScreen(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})

	m, _ = step(t, m, errMsg{errors.New("baza pukla")})

	if m.err == nil {
		t.Fatal("load error did not set err")
	}
	if !strings.Contains(m.View(), "baza pukla") {
		t.Errorf("error screen missing message:\n%s", m.View())
	}
}

func TestActionsOnEmptyListDoNothing(t *testing.T) {
	m := ready(t)

	for _, k := range []string{"o", "b"} {
		next, cmd := step(t, m, press(k))
		if cmd != nil {
			t.Errorf("%q returned a command with no posts", k)
		}
		if next.status != "" {
			t.Errorf("%q set a status with no posts: %q", k, next.status)
		}
	}
}

func TestActionKeysWorkInDetail(t *testing.T) {
	post := testPost("Prvi")
	m := ready(t, post)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenDetail {
		t.Fatal("detail did not open")
	}

	next, cmd := step(t, m, press("b"))
	if cmd == nil {
		t.Error("b did not trigger a command in detail")
	}
	if next.screen != screenDetail {
		t.Error("b closed the detail view")
	}

	next, cmd = step(t, m, press("o"))
	if cmd == nil {
		t.Error("o did not trigger a command in detail")
	}
	if next.screen != screenDetail {
		t.Error("o closed the detail view")
	}
}

func TestActionKeysIgnoredWhileFiltering(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, press("/"))
	m, _ = step(t, m, press("b"))
	m, _ = step(t, m, press("o"))

	if got, want := m.list.FilterValue(), "bo"; got != want {
		t.Errorf("filter value = %q, want %q", got, want)
	}
	if m.status != "" {
		t.Errorf("action fired while filtering: %q", m.status)
	}
}
