package tui

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func testUser() database.User {
	return database.User{ID: uuid.Nil, Name: "marko"}
}

func testPost(title string) database.Post {
	return database.Post{
		ID:          uuid.New(),
		Title:       title,
		Url:         "https://example.com/" + title,
		Description: sql.NullString{String: "<p>Telo posta " + title + "</p>", Valid: true},
		PublishedAt: sql.NullTime{Time: time.Date(2026, 7, 27, 18, 55, 0, 0, time.UTC), Valid: true},
	}
}

// step primeni jednu poruku i vrati novi model, da testovi ne rade
// type assertion na svakom koraku.
func step(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	return got, cmd
}

// ready vrati model dimenzionisan na 80x24 sa ucitanim postovima.
func ready(t *testing.T, posts ...database.Post) model {
	t.Helper()
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: posts})
	return m
}

func TestWindowSizeSetsViewportHeight(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 80, Height: 24})

	if got, want := m.viewport.Width, 80; got != want {
		t.Errorf("viewport width = %d, want %d", got, want)
	}
	if got, want := m.viewport.Height, 24-detailChromeHeight; got != want {
		t.Errorf("viewport height = %d, want %d", got, want)
	}
}

func TestWindowSizeTinyTerminal(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), tea.WindowSizeMsg{Width: 10, Height: 2})

	if m.viewport.Height < 1 {
		t.Errorf("viewport height = %d, want at least 1", m.viewport.Height)
	}
}

func TestPostsLoadedFillsList(t *testing.T) {
	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	if m.loading {
		t.Error("loading still true after postsLoadedMsg")
	}
	if got, want := len(m.list.Items()), 2; got != want {
		t.Fatalf("list items = %d, want %d", got, want)
	}
	if got, want := m.list.Items()[0].(postItem).post.Title, "Prvi"; got != want {
		t.Errorf("first item = %q, want %q", got, want)
	}
}

func TestErrMsgShownInView(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}), errMsg{errors.New("baza pukla")})

	if m.loading {
		t.Error("loading still true after errMsg")
	}
	if got := m.View(); !strings.Contains(got, "baza pukla") {
		t.Errorf("view does not show error:\n%s", got)
	}
}

func TestLoadingView(t *testing.T) {
	m := newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc})

	if got := m.View(); !strings.Contains(got, "Loading") {
		t.Errorf("initial view = %q, want a loading message", got)
	}
}

func TestEnterOpensDetailAndEscCloses(t *testing.T) {
	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenDetail {
		t.Fatal("enter did not open detail")
	}
	if got, want := m.selected.Title, "Prvi"; got != want {
		t.Errorf("selected = %q, want %q", got, want)
	}

	view := m.View()
	for _, want := range []string{"Prvi", "Telo posta Prvi", "Published: 2026-07-27 18:55"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q:\n%s", want, view)
		}
	}
	if got, want := strings.Count(view, "\n")+1, 24; got != want {
		t.Errorf("detail view = %d lines, want %d", got, want)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen == screenDetail {
		t.Error("esc did not close detail")
	}
}

func TestEnterOnEmptyListDoesNothing(t *testing.T) {
	m := ready(t)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen == screenDetail {
		t.Error("detail opened with no posts")
	}
}

func TestArrowsMoveListSelectionNotDetail(t *testing.T) {
	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got, want := m.selected.Title, "Drugi"; got != want {
		t.Errorf("selected after down = %q, want %q", got, want)
	}
}

func TestDetailKeysDoNotMoveList(t *testing.T) {
	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	before := m.list.Index()

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	if got := m.list.Index(); got != before {
		t.Errorf("list index moved while in detail: %d, want %d", got, before)
	}
	if m.screen != screenDetail {
		t.Error("detail closed unexpectedly")
	}
}

func TestQuitKeys(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"q", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := ready(t, testPost("Prvi"))

			_, cmd := step(t, m, tc.msg)
			if cmd == nil {
				t.Fatal("no command returned")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("%s did not quit", tc.name)
			}
		})
	}
}

func TestCtrlCQuitsFromDetail(t *testing.T) {
	m := ready(t, testPost("Prvi"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	_, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("no command returned")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c did not quit from detail")
	}
}

// Dok je filter aktivan "q" je obican karakter, ne izlaz iz aplikacije.
func TestQDoesNotQuitWhileFiltering(t *testing.T) {
	m := ready(t, testPost("Prvi"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q quit the app while filtering")
		}
	}
	if got, want := m.list.FilterValue(), "q"; got != want {
		t.Errorf("filter value = %q, want %q", got, want)
	}
}

func TestResizeWhileInDetailRewrapsBody(t *testing.T) {
	post := testPost("Prvi")
	post.Description = sql.NullString{String: strings.Repeat("rec ", 200), Valid: true}
	m := ready(t, post)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 30, Height: 20})

	for _, line := range strings.Split(m.View(), "\n") {
		if lipgloss.Width(line) > 30 {
			t.Fatalf("line wider than terminal after resize: %q", line)
		}
	}
}
