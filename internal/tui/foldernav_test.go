package tui

import (
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
)

func typing(t *testing.T, m model, keys string) model {
	t.Helper()
	for _, r := range keys {
		m, _ = step(t, m, press(string(r)))
	}
	return m
}

func manyFolders(t *testing.T, names ...string) model {
	t.Helper()
	rows := make([]database.GetFeedFollowsForUserRow, 0, len(names))
	for _, n := range names {
		rows = append(rows, testFeedIn("Feed "+n, n))
	}
	return withFeeds(t, ready(t, testPost("Prvi")), rows...)
}

func TestFolderByNumber(t *testing.T) {
	three := []database.GetFeedFollowsForUserRow{
		testFeedIn("XKCD", "Comics"),
		testFeedIn("Ars", "Tech"),
		testFeedIn("BBC", ""),
	}

	twelve := make([]database.GetFeedFollowsForUserRow, 0, 12)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		twelve = append(twelve, testFeedIn("Feed "+n, n))
	}

	tests := []struct {
		name string
		rows []database.GetFeedFollowsForUserRow
		n    int
		want string
	}{
		{"prvi folder", three, 1, "Comics"},
		{"drugi folder", three, 2, "Tech"},
		{"nekategorisano je zadnje", three, 3, rootFolder},
		{"iznad broja foldera", three, 4, ""},
		{"nula nije folder", three, 0, ""},
		{"negativno", three, -1, ""},
		{"deveti postoji", twelve, 9, "i"},
		{"deseti je van prečica", twelve, 10, ""},
		{"bez foldera nema skoka", []database.GetFeedFollowsForUserRow{testFeedIn("BBC", "")}, 1, ""},
		{"bez feedova", nil, 1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := folderByNumber(tt.rows, tt.n)
			if tt.want == "" {
				if ok {
					t.Errorf("folderByNumber(%d) = %q, want no folder", tt.n, got)
				}
				return
			}
			if !ok || got != tt.want {
				t.Errorf("folderByNumber(%d) = %q (ok=%v), want %q", tt.n, got, ok, tt.want)
			}
		})
	}
}

func TestFolderNumbersStopAtNine(t *testing.T) {
	m := manyFolders(t, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k")

	titles := panelTitles(m)[1:]
	if got, want := len(titles), 11; got != want {
		t.Fatalf("panel has %d folders, want %d", got, want)
	}

	for i, title := range titles[:maxFolderKeys] {
		if !strings.HasPrefix(title, string(rune('1'+i))+" "+folderClosed) {
			t.Errorf("folder %d = %q, want a %d prefix", i+1, title, i+1)
		}
	}
	for _, title := range titles[maxFolderKeys:] {
		if !strings.HasPrefix(title, folderClosed) {
			t.Errorf("folder past nine = %q, want it unnumbered and unindented", title)
		}
	}
}

func TestDigitJumpsToFolderFromThePostList(t *testing.T) {
	m := manyFolders(t, "Comics", "Go", "Tech")
	if m.focus != focusPosts {
		t.Fatal("test starts in the wrong pane")
	}

	m = typing(t, m, "2")

	if m.focus != focusFeeds {
		t.Error("jumping to a folder did not move focus to the feed pane")
	}
	if !m.expanded["Go"] {
		t.Error("jumping to a folder left it closed")
	}
	fol, ok := m.feedList.SelectedItem().(folderItem)
	if !ok || fol.name != "Go" {
		t.Fatalf("selection = %v, want the Go folder", m.feedList.SelectedItem())
	}
	if !strings.Contains(strings.Join(panelTitles(m), "\n"), "Feed Go") {
		t.Error("the folder opened without bringing its feeds in")
	}
}

func TestDigitForAMissingFolderDoesNothing(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")

	m = typing(t, m, "7")

	if m.focus != focusPosts {
		t.Error("a digit with no folder behind it moved focus anyway")
	}
	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want nothing opened", m.expanded)
	}
}

func TestDigitDoesNothingWhenTheFeedPaneIsHidden(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")
	m, _ = step(t, m, tea.WindowSizeMsg{Width: minWidthForFeeds - 1, Height: 24})

	m = typing(t, m, "1")

	if m.focus != focusPosts {
		t.Error("focus moved into a pane that is not on screen")
	}
	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want nothing opened", m.expanded)
	}
}

func TestUnfoldAllThenFoldAllKeepsTheCursorOnAFolder(t *testing.T) {
	m := manyFolders(t, "Comics", "Go", "Tech")

	m = typing(t, m, "-")
	if got, want := len(m.expanded), 3; got != want {
		t.Fatalf("expanded %d folders, want %d", got, want)
	}
	if !strings.Contains(strings.Join(panelTitles(m), "\n"), "Feed Tech") {
		t.Error("unfolding all did not bring every feed in")
	}

	m = typing(t, m, "3")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if fi, ok := m.feedList.SelectedItem().(feedItem); !ok || fi.name != "Feed Tech" {
		t.Fatalf("selection = %v, want the feed inside Tech", m.feedList.SelectedItem())
	}

	m = typing(t, m, "0")

	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want every folder closed", m.expanded)
	}
	fol, ok := m.feedList.SelectedItem().(folderItem)
	if !ok || fol.name != "Tech" {
		t.Errorf("selection = %v, want the Tech header that held the feed", m.feedList.SelectedItem())
	}
}

func TestBracketStepsBetweenFolderHeaders(t *testing.T) {
	m := manyFolders(t, "Comics", "Go", "Tech")
	m = typing(t, m, "-")
	m = typing(t, m, "1")

	m = typing(t, m, "]")
	if fol, ok := m.feedList.SelectedItem().(folderItem); !ok || fol.name != "Go" {
		t.Fatalf("] = %v, want it to skip the feed row and land on Go", m.feedList.SelectedItem())
	}

	m = typing(t, m, "[")
	if fol, ok := m.feedList.SelectedItem().(folderItem); !ok || fol.name != "Comics" {
		t.Fatalf("[ = %v, want Comics", m.feedList.SelectedItem())
	}

	m = typing(t, m, "[")
	if fol, ok := m.feedList.SelectedItem().(folderItem); !ok || fol.name != "Comics" {
		t.Errorf("[ past the first folder = %v, want it to stay on Comics", m.feedList.SelectedItem())
	}

	m = typing(t, m, "]]]]")
	if fol, ok := m.feedList.SelectedItem().(folderItem); !ok || fol.name != "Tech" {
		t.Errorf("] past the last folder = %v, want it to stay on Tech", m.feedList.SelectedItem())
	}
}

func TestBracketsDoNotStepOntoFeedsOrAllFeeds(t *testing.T) {
	m := manyFolders(t, "Comics")
	m = typing(t, m, "1")

	m = typing(t, m, "[")
	if fol, ok := m.feedList.SelectedItem().(folderItem); !ok || fol.name != "Comics" {
		t.Errorf("[ = %v, want it to stay rather than land on %q", m.feedList.SelectedItem(), allFeedsLabel)
	}
}

func TestFolderDigitsStayOutOfTheSearchPrompt(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")

	m = typing(t, m, "s")
	m = typing(t, m, "90-")

	if m.input != inputSearch {
		t.Fatal("search prompt closed")
	}
	if got, want := m.prompt.Value(), "90-"; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}
	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want the digits to have gone to the prompt", m.expanded)
	}
}

func TestFolderDigitsStayOutOfTheListFilter(t *testing.T) {
	m := manyFolders(t, "Comics", "Go")

	m = typing(t, m, "/")
	m = typing(t, m, "1")

	if m.focus != focusPosts {
		t.Error("a digit typed into the list filter moved focus to the feed pane")
	}
	if len(m.expanded) != 0 {
		t.Errorf("expanded = %v, want the digit to have gone to the filter", m.expanded)
	}
}
