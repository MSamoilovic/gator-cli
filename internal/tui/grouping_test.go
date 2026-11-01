package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func postIn(feed database.GetFeedFollowsForUserRow, title string, age time.Duration) database.Post {
	p := testPost(title)
	p.FeedID = feed.FeedID
	p.PublishedAt.Time = time.Now().Add(-age)
	p.PublishedAt.Valid = true
	return p
}

func groupedReader(t *testing.T) (model, database.GetFeedFollowsForUserRow, database.GetFeedFollowsForUserRow) {
	t.Helper()

	go1, sport := testFeed("Go Blog"), testFeed("Sport Desk")
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortFeed}, ""),
		tea.WindowSizeMsg{Width: 80, Height: 24})
	m = withFeeds(t, m, go1, sport)
	m, _ = step(t, m, postsLoadedMsg{posts: []database.Post{
		postIn(go1, "Generics revisited", time.Hour),
		postIn(go1, "Errors are values", 2*time.Hour),
		postIn(sport, "Match report", 3*time.Hour),
	}})
	return m, go1, sport
}

func TestGroupingPutsAHeaderBeforeEachFeed(t *testing.T) {
	m, _, _ := groupedReader(t)

	var labels []string
	for _, it := range m.list.Items() {
		if h, ok := it.(feedHeader); ok {
			labels = append(labels, h.label())
		}
	}
	if want := []string{"Go Blog", "Sport Desk"}; strings.Join(labels, ",") != strings.Join(want, ",") {
		t.Errorf("headers = %v, want %v — one per feed, in the order the posts arrive", labels, want)
	}
	if got, want := len(m.list.Items()), 5; got != want {
		t.Errorf("list has %d rows, want %d (3 posts + 2 headers)", got, want)
	}
}

func TestTheUngroupedListHasNoHeadersAtAll(t *testing.T) {
	m := ready(t, testPost("Prvi"), testPost("Drugi"))

	for _, it := range m.list.Items() {
		if _, ok := it.(feedHeader); ok {
			t.Fatal("a header appeared while sorting by date, where every row is a post")
		}
	}
}

func TestTheCursorNeverRestsOnAHeader(t *testing.T) {
	m, _, _ := groupedReader(t)

	if _, ok := m.list.SelectedItem().(postItem); !ok {
		t.Fatal("the list opened with a header selected, so enter would do nothing")
	}

	for step_ := range 8 {
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
		if _, ok := m.list.SelectedItem().(postItem); !ok {
			t.Fatalf("after %d presses of down the cursor sits on a header", step_+1)
		}
	}
	for step_ := range 8 {
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
		if _, ok := m.list.SelectedItem().(postItem); !ok {
			t.Fatalf("after %d presses of up the cursor sits on a header", step_+1)
		}
	}
}

func TestEveryPostIsStillReachableWithTheArrows(t *testing.T) {
	m, _, _ := groupedReader(t)

	seen := map[uuid.UUID]bool{}
	for range 10 {
		if item, ok := m.list.SelectedItem().(postItem); ok {
			seen[item.post.ID] = true
		}
		m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if got, want := len(seen), 3; got != want {
		t.Errorf("reached %d of %d posts; a header is swallowing one", got, want)
	}
}

func TestOpeningFromAGroupedListOpensThePostUnderTheCursor(t *testing.T) {
	m, _, _ := groupedReader(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenDetail {
		t.Fatalf("screen = %v after enter, want the detail screen", m.screen)
	}
	if got, want := m.selected.Title, "Generics revisited"; got != want {
		t.Errorf("opened %q, want %q", got, want)
	}
}

func TestSteppingThroughPostsInTheDetailSkipsHeaders(t *testing.T) {
	m, _, _ := groupedReader(t)
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	var titles []string
	for range 2 {
		m, _ = step(t, m, press("n"))
		titles = append(titles, m.selected.Title)
	}
	if want := "Errors are values,Match report"; strings.Join(titles, ",") != want {
		t.Errorf("n walked %v, want %q — it must step over the header between the feeds",
			titles, want)
	}
}

func TestGroupingAsksTheStoreForADigestRatherThanEverything(t *testing.T) {
	plain := listFilter{sortDir: sortDesc}.params()
	if plain.perFeed != 0 {
		t.Errorf("perFeed = %d while sorting by date, want 0 — that would hide posts", plain.perFeed)
	}

	grouped := listFilter{sortDir: sortFeed}.params()
	if grouped.perFeed != digestPerFeed {
		t.Errorf("perFeed = %d, want %d", grouped.perFeed, digestPerFeed)
	}
	if grouped.sortDir != sortDesc {
		t.Errorf("sortDir = %q, want %q — the server only accepts asc or desc, so \"feed\" "+
			"must never reach the wire", grouped.sortDir, sortDesc)
	}
}

func TestTheCountDoesNotCallAHeaderAPost(t *testing.T) {
	m, _, _ := groupedReader(t)

	view := m.View()
	if strings.Contains(view, "5 posts") {
		t.Error("the status bar counts the two headers as posts; there are three")
	}
	if !strings.Contains(view, "5 rows") {
		t.Errorf("view = %q, want the count to name what it is really counting", view)
	}
}

func TestTheHeaderFillsItsRowSoThePageDoesNotShiftUp(t *testing.T) {
	m, _, _ := groupedReader(t)

	for _, d := range []density{densityComfortable, densityCompact} {
		del := postDelegate(d)
		var b strings.Builder
		del.Render(&b, m.list, 0, feedHeader{feedID: uuid.New(), feedNames: m.feedNames})

		if got := strings.Count(b.String(), "\n") + 1; got != del.Height() {
			t.Errorf("%v density: a header renders %d lines but the delegate claims %d, so the "+
				"list's pagination is off by one per group", d, got, del.Height())
		}
	}
}

func paneRow(t *testing.T, m model, label string) int {
	t.Helper()
	for i, it := range m.feedList.Items() {
		if v, ok := it.(viewItem); ok && v.label == label {
			return i
		}
	}
	t.Fatalf("no %q row in the feed pane", label)
	return 0
}

func pickView(t *testing.T, m model, label string) model {
	t.Helper()
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m.feedList.Select(paneRow(t, m, label))
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func TestTheFeedPaneSwitchesBetweenTheTwoViews(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	m = pickView(t, m, byFeedLabel)
	if got, want := m.filter.sortDir, sortFeed; got != want {
		t.Errorf("sortDir = %q, want %q after picking %q", got, want, byFeedLabel)
	}

	m = pickView(t, m, timelineLabel)
	if got, want := m.filter.sortDir, sortDesc; got != want {
		t.Errorf("sortDir = %q, want %q after picking %q", got, want, timelineLabel)
	}
}

func TestEitherViewShowsEveryFeedAgain(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, ready(t, testPost("Prvi")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.filter.feedID != feed.FeedID {
		t.Fatal("the fixture did not select a feed, so this proves nothing")
	}

	m = pickView(t, m, byFeedLabel)
	if m.filter.feedID != uuid.Nil {
		t.Error("picking a view kept one feed's filter, so it is not showing every feed")
	}
	if m.focus != focusPosts {
		t.Error("focus stayed in the pane after picking a view")
	}
}

func TestPickingTimelineKeepsAnOldestFirstChoice(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))
	m, _ = step(t, m, press("S"))
	if m.filter.sortDir != sortAsc {
		t.Fatalf("sortDir = %q, want the fixture in oldest-first", m.filter.sortDir)
	}

	m = pickView(t, m, timelineLabel)
	if got, want := m.filter.sortDir, sortAsc; got != want {
		t.Errorf("sortDir = %q, want %q — Timeline must not silently undo oldest-first", got, want)
	}
}

func TestTheActiveViewIsMarkedWhicheverWayItWasChosen(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC Sport"))

	active := func(label string) bool {
		return m.feedList.Items()[paneRow(t, m, label)].(viewItem).active
	}

	if !active(timelineLabel) || active(byFeedLabel) {
		t.Fatal("the pane does not start with Timeline marked")
	}

	m, _ = step(t, m, press("S"))
	m, _ = step(t, m, press("S"))
	if !active(byFeedLabel) || active(timelineLabel) {
		t.Error("S changed the list but the pane still marks the old view")
	}

	m = pickView(t, m, timelineLabel)
	if !active(timelineLabel) || active(byFeedLabel) {
		t.Error("picking in the pane did not move the marker")
	}
}

func TestTheActiveMarkIsExactlyAsWideAsTheIndent(t *testing.T) {
	on := viewItem{label: "x", active: true}.Title()
	off := viewItem{label: "x", active: false}.Title()

	if lipgloss.Width(on) != lipgloss.Width(off) {
		t.Errorf("active row is %d columns and inactive is %d, so the two rows do not line up",
			lipgloss.Width(on), lipgloss.Width(off))
	}
}
