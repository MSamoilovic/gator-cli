package tui

import (
	"github.com/MSamoilovic/gator-cli/internal/store"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func TestOnlyTheFeedsSourceIsNotDerived(t *testing.T) {
	if sourceFeeds.derived() {
		t.Error("sourceFeeds reports itself as derived, so sorting and filtering would be refused")
	}
	for _, s := range []listSource{sourceBookmarks, sourceSearch} {
		if !s.derived() {
			t.Errorf("source %d is not derived, so paging and sorting would run against the wrong query", s)
		}
	}
}

func TestTheListHasExactlyOneSourceAtATime(t *testing.T) {
	m := loaded(t, fullPage("a"))

	if m.source != sourceFeeds {
		t.Fatalf("a fresh model starts on source %d, want sourceFeeds", m.source)
	}

	m, _ = step(t, m, press("B"))
	if m.source != sourceBookmarks {
		t.Errorf("after B the source is %d, want sourceBookmarks", m.source)
	}

	m, _ = step(t, m, press("H"))
	if m.source != sourceBookmarks {
		t.Errorf("H changed the source to %d — the read view is a filter now, not a source", m.source)
	}
	if m.filter.read != store.ReadAny {
		t.Errorf("H inside the bookmarks view set the read filter to %q, want it refused", m.filter.read)
	}

	m, _ = step(t, m, press("B"))
	if m.source != sourceFeeds {
		t.Errorf("a second B left the source at %d, want sourceFeeds", m.source)
	}

	m, _ = step(t, m, press("H"))
	if m.source != sourceFeeds || m.filter.read != store.ReadOnly {
		t.Errorf("H from the feeds view gave source %d and read %q, want sourceFeeds and read", m.source, m.filter.read)
	}
}

func TestPressingTheSameViewKeyTwiceReturnsToFeeds(t *testing.T) {
	for _, key := range []string{"B"} {
		m := loaded(t, fullPage("a"))

		m, _ = step(t, m, press(key))
		if m.source == sourceFeeds {
			t.Fatalf("%s did not leave the feeds source", key)
		}

		m, _ = step(t, m, press(key))
		if m.source != sourceFeeds {
			t.Errorf("%s twice left the source at %d, want sourceFeeds", key, m.source)
		}
	}
}

func TestSearchIsItsOwnSourceAndEscapeLeavesIt(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "golang")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.source != sourceSearch {
		t.Fatalf("after a search the source is %d, want sourceSearch", m.source)
	}
	if m.filter.query != "golang" {
		t.Errorf("query = %q, want %q", m.filter.query, "golang")
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.source != sourceFeeds {
		t.Errorf("esc left the source at %d, want sourceFeeds", m.source)
	}
	if m.filter.query != "" {
		t.Errorf("esc left the query as %q, want it cleared", m.filter.query)
	}
}

func TestSwitchingSourceKeepsTheFilter(t *testing.T) {
	feed := testFeed("BBC Sport")
	saved := uiState{
		FeedID:     feed.FeedID.String(),
		FeedName:   "BBC Sport",
		SortDir:    sortAsc,
		UnreadOnly: true,
		SinceHours: 168,
	}
	m, _ := step(t, newModel(t.Context(), nil, testUser(), saved), tea.WindowSizeMsg{Width: 80, Height: 24})

	before := m.filter

	m, _ = step(t, m, press("B"))
	m, _ = step(t, m, press("B"))

	if m.filter != before {
		t.Errorf("round trip through the bookmarks view changed the filter:\n got %+v\nwant %+v", m.filter, before)
	}
}

func TestParamsTurnsTheWindowIntoAnAbsoluteTime(t *testing.T) {
	f := listFilter{since: 7 * 24 * time.Hour}

	got := f.params()
	if got.since.IsZero() {
		t.Fatal("a 7d window produced a zero time, so the SQL filter would be skipped")
	}
	if d := time.Since(got.since); d < 6*24*time.Hour || d > 8*24*time.Hour {
		t.Errorf("since is %v ago, want about 7 days", d)
	}
}

func TestParamsLeavesNoWindowAlone(t *testing.T) {
	if got := (listFilter{}).params(); !got.since.IsZero() {
		t.Errorf("an empty window produced %v, want the zero time so SQL skips the condition", got.since)
	}
}

func TestParamsCarriesEveryFilterField(t *testing.T) {
	id := uuid.New()
	f := listFilter{feedID: id, sortDir: sortAsc, read: store.ReadUnread}

	got := f.params()
	if got.feedID != id {
		t.Errorf("feedID = %v, want %v", got.feedID, id)
	}
	if got.sortDir != sortAsc {
		t.Errorf("sortDir = %q, want %q", got.sortDir, sortAsc)
	}
	if got.read != store.ReadUnread {
		t.Errorf("read state = %q, want it carried to the query", got.read)
	}
}

func TestDerivedViewsStillRefuseSortingAndTimeRange(t *testing.T) {
	for _, key := range []string{"B"} {
		m := loaded(t, fullPage("a"))
		m, _ = step(t, m, press(key))

		before := m.filter
		m, _ = step(t, m, press("S"))
		m, _ = step(t, m, press("t"))

		if m.filter != before {
			t.Errorf("in the %s view sorting or the time range still changed the filter", key)
		}
	}
}
