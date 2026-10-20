package tui

import (
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

func feedRow(name string, failures int32, indent bool, unread int) feedItem {
	id := uuid.New()
	return feedItem{
		id:       id,
		name:     name,
		failures: failures,
		indent:   indent,
		unread:   map[uuid.UUID]int{id: unread},
	}
}

func nameColumn(t *testing.T, item feedItem) int {
	t.Helper()
	title := item.Title()
	at := strings.Index(title, item.name)
	if at < 0 {
		t.Fatalf("Title() = %q does not contain the feed name %q", title, item.name)
	}
	return lipgloss.Width(title[:at])
}

func TestAFailingFeedKeepsItsNameInTheSameColumn(t *testing.T) {
	healthy := feedRow("Escape Pod", 0, true, 10)
	failing := feedRow("Escape Pod", 3, true, 10)

	if got, want := nameColumn(t, failing), nameColumn(t, healthy); got != want {
		t.Errorf("a flagged feed starts its name at column %d, a healthy one at %d — the warning must "+
			"take the indent, not push the name out of the column of names", got, want)
	}
	if !strings.HasPrefix(failing.Title(), brokenMark) {
		t.Errorf("Title() = %q, want the warning in the gutter", failing.Title())
	}
	if !strings.HasPrefix(healthy.Title(), feedIndent) {
		t.Errorf("Title() = %q, want a nested feed indented", healthy.Title())
	}
}

func TestFlaggingAFeedCostsItNoLabelWidth(t *testing.T) {
	budget := feedPanelWidth - panelGutter - 2

	for _, name := range []string{"Go", "Escape Pod", "Basketball Insiders", "The Hollywood Reporter"} {
		healthy := lipgloss.Width(feedRow(name, 0, true, 10).Title())
		failing := lipgloss.Width(feedRow(name, 2, true, 10).Title())

		if healthy != failing {
			t.Errorf("%q is %d columns healthy and %d flagged; measured on the real database that "+
				"took truncated labels from 24 to 39 of 142", name, healthy, failing)
		}
		_ = budget
	}
}

func TestATopLevelFeedHasNoIndentToBorrow(t *testing.T) {
	flat := feedRow("Escape Pod", 0, false, 10)
	if got, want := flat.Title(), "Escape Pod (10)"; got != want {
		t.Errorf("Title() = %q, want %q — an uncategorised list has no indent, and buying one would "+
			"cost every feed two columns (measured: 14 truncated labels become 24)", got, want)
	}

	flagged := feedRow("Escape Pod", 1, false, 10)
	if got, want := flagged.Title(), brokenMark+" Escape Pod (10)"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
}

func TestTheWarningGutterIsExactlyAsWideAsTheIndent(t *testing.T) {
	if got, want := lipgloss.Width(brokenMark+" "), lipgloss.Width(feedIndent); got != want {
		t.Fatalf("the warning gutter is %d columns and the indent is %d; the swap only works while "+
			"they are equal, and %q is one cell wide today", got, want, brokenMark)
	}
}

func TestTheFeedPaneDrawsEveryNestedNameInOneColumn(t *testing.T) {
	broken := database.GetFeedFollowsForUserRow{FeedID: uuid.New(), FeedName: "Escape Pod", Category: "Sci-fi", FeedFailures: 4}
	feeds := []database.GetFeedFollowsForUserRow{
		{FeedID: uuid.New(), FeedName: "Clarkesworld", Category: "Sci-fi"},
		broken,
		{FeedID: uuid.New(), FeedName: "Reactor", Category: "Sci-fi"},
	}

	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc, Expanded: []string{"Sci-fi"}}, ""),
		tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: densePosts(3)})
	m = withFeeds(t, m, feeds...)

	view := m.View()
	columns := map[string]int{}
	for _, line := range strings.Split(view, "\n") {
		for _, f := range feeds {
			at := strings.Index(line, f.FeedName)
			if at < 0 {
				continue
			}
			columns[f.FeedName] = lipgloss.Width(line[:at])
		}
	}

	if len(columns) != len(feeds) {
		t.Fatalf("found %d of %d feed names in the pane:\n%s", len(columns), len(feeds), view)
	}
	for name, got := range columns {
		if got != columns["Clarkesworld"] {
			t.Errorf("%q is drawn at column %d, %q at %d:\n%s", name, got, "Clarkesworld", columns["Clarkesworld"], view)
		}
	}
	if !strings.Contains(view, brokenMark+" "+broken.FeedName) {
		t.Errorf("the pane lost the warning on %q:\n%s", broken.FeedName, view)
	}
}

func TestAFolderStillCarriesAWarningForItsCollapsedFeeds(t *testing.T) {
	item := folderItem{
		name:     "Sci-fi",
		number:   1,
		broken:   1,
		expanded: map[string]bool{},
		unread:   map[uuid.UUID]int{},
	}
	if !strings.Contains(item.Title(), brokenMark) {
		t.Errorf("Title() = %q, want the warning — a failing feed inside a collapsed folder is "+
			"invisible otherwise", item.Title())
	}
}
