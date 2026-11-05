package tui

import (
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/catalog"
	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func openPicker(t *testing.T, feeds ...database.GetFeedFollowsForUserRow) model {
	t.Helper()

	m := withFeeds(t, ready(t, testPost("Prvi")), feeds...)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, cmd := step(t, m, press("c"))
	if cmd != nil {
		m, _ = step(t, m, cmd())
	}

	if m.screen != screenCatalog {
		t.Fatal("c did not open the catalog")
	}
	return m
}

func firstCategory(t *testing.T) catalog.Category {
	t.Helper()
	cats, err := catalog.Categories()
	if err != nil {
		t.Fatal(err)
	}
	return cats[0]
}

func TestCatalogOpensWithEveryCategory(t *testing.T) {
	m := openPicker(t)

	cats, err := catalog.Categories()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(m.catalogList.Items()), len(cats); got != want {
		t.Fatalf("catalog items = %d, want %d", got, want)
	}

	view := m.View()
	if !strings.Contains(view, catalogTitle) {
		t.Errorf("catalog view has no title:\n%s", view)
	}
	if !strings.Contains(view, cats[0].Label) {
		t.Errorf("catalog view missing %q:\n%s", cats[0].Label, view)
	}
}

func TestSpaceTogglesCategory(t *testing.T) {
	m := openPicker(t)
	first := firstCategory(t)

	if strings.Contains(m.View(), "[x]") {
		t.Fatal("something was picked before space was pressed")
	}

	m, _ = step(t, m, press(" "))
	for _, f := range first.Feeds {
		if !m.picked[f.URL] {
			t.Errorf("space on a category did not pick %q", f.URL)
		}
	}
	if !strings.Contains(m.View(), "[x] "+first.Label) {
		t.Errorf("picked category has no [x] marker:\n%s", m.View())
	}

	m, _ = step(t, m, press(" "))
	if len(m.picked) != 0 {
		t.Errorf("second space left %d feeds picked", len(m.picked))
	}
}

func TestAddWithNothingPickedStaysInCatalog(t *testing.T) {
	m := openPicker(t)

	m, _ = step(t, m, press("a"))

	if m.screen != screenCatalog {
		t.Error("empty confirmation left the catalog")
	}
	if !strings.Contains(m.status, "Nothing picked") {
		t.Errorf("status = %q, want a hint about picking", m.status)
	}
}

func TestAddAddsPickedFeedsAndLeavesCatalog(t *testing.T) {
	m := openPicker(t)

	m, _ = step(t, m, press(" "))
	m, cmd := step(t, m, press("a"))

	if m.screen != screenList {
		t.Error("confirming did not return to the list")
	}
	if cmd == nil {
		t.Fatal("confirming issued no command")
	}
	if !strings.Contains(m.status, "Adding") {
		t.Errorf("status = %q, want an 'Adding …' message", m.status)
	}
}

func TestEscapeLeavesCatalogWithoutAdding(t *testing.T) {
	m := openPicker(t)

	m, _ = step(t, m, press(" "))
	m, cmd := step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.screen != screenList {
		t.Error("esc did not leave the catalog")
	}
	if cmd != nil {
		if _, adding := cmd().(catalogAddedMsg); adding {
			t.Error("esc added feeds anyway")
		}
	}
}

func TestReopeningCatalogClearsPicks(t *testing.T) {
	m := openPicker(t)
	first := firstCategory(t)

	m, _ = step(t, m, press(" "))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	m, cmd := step(t, m, press("c"))
	if cmd != nil {
		m, _ = step(t, m, cmd())
	}

	if m.picked[first.Feeds[0].URL] {
		t.Error("reopening the catalog kept the previous picks")
	}
}

func TestCatalogCountsFeedsYouAlreadyFollow(t *testing.T) {
	first := firstCategory(t)

	followed := testFeed(first.Feeds[0].Name)
	followed.FeedUrl = first.Feeds[0].URL

	m := openPicker(t, followed)

	item, ok := m.catalogList.Items()[0].(catalogItem)
	if !ok {
		t.Fatal("first catalog item is not a catalogItem")
	}
	if _, got, _ := item.counts(); got != 1 {
		t.Errorf("followed count = %d, want 1", got)
	}
	if !strings.Contains(m.View(), "1 followed") {
		t.Errorf("view does not report the followed feed:\n%s", m.View())
	}
}

func TestCatalogViewFitsTerminal(t *testing.T) {
	m := openPicker(t)

	view := m.View()
	if got, want := strings.Count(view, "\n")+1, 24; got != want {
		t.Errorf("catalog view = %d lines, want %d", got, want)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line wider than terminal: %d columns: %q", w, line)
		}
	}

	for _, want := range []string{"space pick", "esc back"} {
		if !strings.Contains(view, want) {
			t.Errorf("catalog help missing %q:\n%s", want, view)
		}
	}
}

func TestCatalogAddedMsgSummarises(t *testing.T) {
	cases := []struct {
		name string
		msg  catalogAddedMsg
		want string
	}{
		{"all good", catalogAddedMsg{added: 8}, "Following 8 feeds"},
		{"some failed", catalogAddedMsg{added: 6, failed: 2}, "2 unreachable"},
		{"all failed", catalogAddedMsg{failed: 3}, "No feed could be added"},
		{"already known", catalogAddedMsg{known: 4}, "Following 4 feeds"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := catalogSummary(tc.msg); !strings.Contains(got, tc.want) {
				t.Errorf("catalogSummary() = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

func TestCatalogAddedReloadsFeeds(t *testing.T) {
	m := openPicker(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	_, cmd := step(t, m, catalogAddedMsg{added: 3})
	if cmd == nil {
		t.Fatal("a successful add did not reload anything")
	}

	m2 := openPicker(t)
	m2, _ = step(t, m2, tea.KeyMsg{Type: tea.KeyEsc})
	m2, _ = step(t, m2, catalogAddedMsg{failed: 2})

	if !strings.Contains(m2.status, "No feed could be added") {
		t.Errorf("status = %q, want the failure summary", m2.status)
	}
}

func TestCatalogOpensAfterFeedsLoadWhenAskedTo(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}, ""), tea.WindowSizeMsg{Width: 80, Height: 24})
	m.openOnLoad = true

	if m.screen == screenCatalog {
		t.Fatal("catalog opened before the feeds arrived")
	}

	first := firstCategory(t)
	followed := testFeed(first.Feeds[0].Name)
	followed.FeedUrl = first.Feeds[0].URL
	m, _ = step(t, m, feedsLoadedMsg{feeds: []database.GetFeedFollowsForUserRow{followed}})

	if m.screen != screenCatalog {
		t.Fatal("catalog did not open once the feeds had loaded")
	}
	if m.openOnLoad {
		t.Error("openOnLoad was not cleared, the catalog will reopen on every feed reload")
	}

	item, ok := m.catalogList.Items()[0].(catalogItem)
	if !ok {
		t.Fatal("first catalog item is not a catalogItem")
	}
	if _, got, _ := item.counts(); got != 1 {
		t.Errorf("followed count = %d, want 1 — the catalog opened too early", got)
	}
}

func TestCatalogStaysClosedWithoutTheFlag(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	if m.screen != screenList {
		t.Error("plain gator tui opened straight into the catalog")
	}
}

func TestEmptyStatePointsAtCatalog(t *testing.T) {
	m := withFeeds(t, ready(t))

	if got := m.View(); !strings.Contains(got, "c to pick from the catalog") {
		t.Errorf("empty state does not mention the catalog:\n%s", got)
	}
}

func openFirstCategory(t *testing.T, feeds ...database.GetFeedFollowsForUserRow) model {
	t.Helper()

	m := openPicker(t, feeds...)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenCatalogFeeds {
		t.Fatal("enter did not open the category")
	}
	return m
}

func TestEnterOpensTheFeedsOfACategory(t *testing.T) {
	first := firstCategory(t)
	m := openFirstCategory(t)

	if got, want := len(m.catalogFeeds.Items()), len(first.Feeds); got != want {
		t.Fatalf("category shows %d feeds, want %d", got, want)
	}
	view := m.View()
	if !strings.Contains(view, first.Label) || !strings.Contains(view, first.Feeds[0].Name) {
		t.Errorf("category screen shows neither the category nor its first feed:\n%s", view)
	}
}

func TestASingleFeedCanBePickedOnItsOwn(t *testing.T) {
	first := firstCategory(t)
	m := openFirstCategory(t)

	m, _ = step(t, m, press(" "))
	if !m.picked[first.Feeds[0].URL] {
		t.Fatal("space did not pick the first feed")
	}
	if len(m.picked) != 1 {
		t.Errorf("picked %d feeds, want only the one under the cursor", len(m.picked))
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenCatalog {
		t.Fatal("esc did not return to the categories")
	}
	if len(first.Feeds) > 1 && !strings.Contains(m.View(), "[-] "+first.Label) {
		t.Errorf("a partly picked category has no [-] marker:\n%s", m.View())
	}
}

func TestAddFromACategoryAddsOnlyWhatWasPicked(t *testing.T) {
	m := openFirstCategory(t)

	m, _ = step(t, m, press(" "))
	m, cmd := step(t, m, press("a"))

	if m.screen != screenList {
		t.Error("adding did not return to the list")
	}
	if cmd == nil || !strings.Contains(m.status, "Adding 1 feeds") {
		t.Errorf("status = %q, want exactly one feed being added", m.status)
	}
}

func TestAFollowedFeedCannotBePickedAndIsLeftOutOfACategoryToggle(t *testing.T) {
	first := firstCategory(t)
	followed := testFeed(first.Feeds[0].Name)
	followed.FeedUrl = first.Feeds[0].URL

	m := openFirstCategory(t, followed)
	m, _ = step(t, m, press(" "))
	if m.picked[first.Feeds[0].URL] {
		t.Error("an already followed feed was picked")
	}
	if !strings.Contains(m.status, "Already following") {
		t.Errorf("status = %q, want a note that the feed is already followed", m.status)
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = step(t, m, press(" "))
	if m.picked[first.Feeds[0].URL] {
		t.Error("toggling the category picked a feed that is already followed")
	}
	if got, want := len(m.picked), len(first.Feeds)-1; got != want {
		t.Errorf("category toggle picked %d feeds, want %d", got, want)
	}
}

func TestCategoryScreenFitsTerminal(t *testing.T) {
	m := openFirstCategory(t)

	view := m.View()
	if got, want := strings.Count(view, "\n")+1, 24; got != want {
		t.Errorf("category view = %d lines, want %d", got, want)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line wider than terminal: %d columns: %q", w, line)
		}
	}
}
