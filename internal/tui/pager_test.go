package tui

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestPagerComesFromTheEnvironmentAndFallsBack(t *testing.T) {
	t.Setenv("PAGER", "less -F -R")
	name, args := pagerCommand()
	if name != "less" {
		t.Errorf("pager = %q, want the program from $PAGER", name)
	}
	if len(args) != 2 || args[0] != "-F" || args[1] != "-R" {
		t.Errorf("args = %v, want the flags from $PAGER kept", args)
	}

	t.Setenv("PAGER", "")
	if name, args := pagerCommand(); name == "" || len(args) != 0 {
		t.Errorf("fallback = %q %v, want a bare program name", name, args)
	}
}

func TestTheArticleReachesThePagerWholeAndWrapped(t *testing.T) {
	post := database.Post{
		Title:       "Never send slides",
		Url:         "https://martinfowler.com/articles/slides.html",
		Description: sql.NullString{String: "<p>" + strings.Repeat("word ", 400) + "</p>", Valid: true},
		PublishedAt: sql.NullTime{Time: time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC), Valid: true},
	}

	got := articleText(post, "Martin Fowler", 80)
	for _, want := range []string{"Never send slides", "Martin Fowler", post.Url, "2026-09-08 10:00"} {
		if !strings.Contains(got, want) {
			t.Errorf("article missing %q:\n%s", want, got[:200])
		}
	}
	if strings.Contains(got, "<p>") {
		t.Error("the pager got raw HTML")
	}
	if n := strings.Count(got, "word"); n != 400 {
		t.Errorf("article carries %d of 400 words — the pager exists for the long ones", n)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line is %d columns wide, want it wrapped for the pager: %q", w, line)
		}
	}
}

func TestThePagerPrefersTheFetchedFullText(t *testing.T) {
	post := database.Post{
		Title:       "Teaser",
		Url:         "https://example.com/a",
		Description: sql.NullString{String: "the feed's first paragraph", Valid: true},
		FullText:    "the whole article as fetched",
	}

	got := articleText(post, "Example", 80)
	if !strings.Contains(got, "the whole article as fetched") {
		t.Errorf("article = %q, want the cached full text", got)
	}
	if strings.Contains(got, "the feed's first paragraph") {
		t.Error("the pager got the teaser although the full text was cached")
	}
}

func TestPagerKeyOpensFromTheDetailScreen(t *testing.T) {
	m := loaded(t, fullPage("a"))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	next, cmd := step(t, m, press("e"))
	if cmd == nil {
		t.Fatal("e did not start the pager")
	}
	if next.status != "" {
		t.Errorf("status = %q, want the pager to take over the screen silently", next.status)
	}
}

func TestThePagerIsReachableFromThePaletteToo(t *testing.T) {
	m := loaded(t, fullPage("a"))

	var found bool
	for _, it := range m.paletteActions() {
		if strings.Contains(it.label, "pager") {
			found = true
			if it.hint != "e" {
				t.Errorf("palette hint = %q, want e", it.hint)
			}
		}
	}
	if !found {
		t.Error("the palette does not offer the pager, so it is unreachable from the list")
	}
}

func TestPagerWithoutAPostIsANoOp(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}, ""),
		tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{})

	if _, cmd := step(t, m, press("e")); cmd != nil {
		t.Error("e started a pager with no post selected")
	}
}
