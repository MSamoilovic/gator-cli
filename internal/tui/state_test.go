package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := uiState{
		FeedID:     uuid.New().String(),
		FeedName:   "BBC Sport",
		SortDir:    sortAsc,
		UnreadOnly: true,
		SinceHours: 168,
		Collapsed:  []string{"Sport", "Tech"},
	}
	if err := want.save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := loadState(); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestStateIsWrittenPrivately(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := (uiState{SortDir: sortDesc}).save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	fi, err := os.Stat(filepath.Join(home, stateFileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got, want := fi.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

func TestMissingStateIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	got := loadState()

	if got.SortDir != sortDesc {
		t.Errorf("SortDir = %q, want the %q default", got.SortDir, sortDesc)
	}
	if got.FeedID != "" {
		t.Errorf("FeedID = %q, want empty", got.FeedID)
	}
}

func TestCorruptStateIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(filepath.Join(home, stateFileName), []byte("{ ovo nije json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := loadState(); got.SortDir != sortDesc {
		t.Errorf("corrupt state leaked through: %+v", got)
	}
}

func TestHandEditedStateIsSanitized(t *testing.T) {
	cases := []struct {
		name  string
		given uiState
		check func(*testing.T, uiState)
	}{
		{
			name:  "unknown sort",
			given: uiState{SortDir: "sideways"},
			check: func(t *testing.T, got uiState) {
				if got.SortDir != sortDesc {
					t.Errorf("SortDir = %q, want %q", got.SortDir, sortDesc)
				}
			},
		},
		{
			name:  "malformed feed id",
			given: uiState{SortDir: sortDesc, FeedID: "not-a-uuid", FeedName: "X"},
			check: func(t *testing.T, got uiState) {
				if got.FeedID != "" || got.FeedName != "" {
					t.Errorf("feed survived: %+v", got)
				}
			},
		},
		{
			name:  "time range outside the cycle",
			given: uiState{SortDir: sortDesc, SinceHours: 999},
			check: func(t *testing.T, got uiState) {
				if got.SinceHours != 0 {
					t.Errorf("SinceHours = %d, want 0", got.SinceHours)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.given.sanitized())
		})
	}
}

func TestSavedStateIsApplied(t *testing.T) {
	feed := testFeed("BBC Sport")
	saved := uiState{
		FeedID:     feed.FeedID.String(),
		FeedName:   "BBC Sport",
		SortDir:    sortAsc,
		UnreadOnly: true,
		SinceHours: 168,
	}

	m, _ := step(t, newModel(t.Context(), nil, testUser(), saved), tea.WindowSizeMsg{Width: 80, Height: 24})

	f := m.filter()
	if f.feedID != feed.FeedID {
		t.Error("saved feed was not applied")
	}
	if f.sortDir != sortAsc {
		t.Errorf("sortDir = %q, want %q", f.sortDir, sortAsc)
	}
	if !f.unreadOnly {
		t.Error("saved unread filter was not applied")
	}
	if f.since.IsZero() {
		t.Error("saved time range was not applied")
	}
	if got, want := m.list.Title, "BBC Sport · 7d"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestSnapshotCapturesCurrentView(t *testing.T) {
	feed := testFeed("BBC Sport")
	m := withFeeds(t, loaded(t, fullPage("a")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = step(t, m, press("S"))
	m, _ = step(t, m, press("U"))
	m, _ = step(t, m, press("t"))

	got := m.snapshot()
	if got.FeedID != feed.FeedID.String() {
		t.Errorf("FeedID = %q, want the selected feed", got.FeedID)
	}
	if got.FeedName != "BBC Sport" {
		t.Errorf("FeedName = %q, want %q", got.FeedName, "BBC Sport")
	}
	if got.SortDir != sortAsc {
		t.Errorf("SortDir = %q, want %q", got.SortDir, sortAsc)
	}
	if !got.UnreadOnly {
		t.Error("UnreadOnly was not captured")
	}
	if got.SinceHours != 24 {
		t.Errorf("SinceHours = %d, want 24", got.SinceHours)
	}
}

func TestSnapshotSkipsTransientViews(t *testing.T) {
	m := loaded(t, fullPage("a"))

	m, _ = step(t, m, press("s"))
	m = typeText(t, m, "golang")
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.snapshot(); got.FeedID != "" {
		t.Errorf("search view leaked into the snapshot: %+v", got)
	}

	m, _ = step(t, m, press("B"))
	if got := m.snapshot(); got.FeedID != "" {
		t.Errorf("bookmarks view leaked into the snapshot: %+v", got)
	}
}

func TestSnapshotWithNoFeedIsEmpty(t *testing.T) {
	m := loaded(t, fullPage("a"))

	got := m.snapshot()

	if got.FeedID != "" || got.FeedName != "" {
		t.Errorf("snapshot invented a feed: %+v", got)
	}
	if got.SinceHours != 0 {
		t.Errorf("SinceHours = %d, want 0", got.SinceHours)
	}
}

func TestUnfollowedFeedFallsBackToAll(t *testing.T) {
	gone := uuid.New()
	saved := uiState{FeedID: gone.String(), FeedName: "Obrisan", SortDir: sortDesc}

	m, _ := step(t, newModel(t.Context(), nil, testUser(), saved), tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.feedID != gone {
		t.Fatal("saved feed was not applied")
	}

	m, cmd := step(t, m, feedsLoadedMsg{feeds: nil})

	if m.feedID != uuid.Nil {
		t.Error("model kept a feed that is no longer followed")
	}
	if got, want := m.list.Title, postsTitle; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if cmd == nil {
		t.Error("falling back did not reload the unfiltered list")
	}
}

func TestStoredFeedIsSelectedInThePanel(t *testing.T) {
	first := testFeed("BBC Sport")
	second := testFeed("CBR")
	saved := uiState{FeedID: second.FeedID.String(), FeedName: "CBR", SortDir: sortDesc}

	m, _ := step(t, newModel(t.Context(), nil, testUser(), saved), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, feedsLoadedMsg{feeds: []database.GetFeedFollowsForUserRow{first, second}})

	if got, want := m.feedList.Index(), 2; got != want {
		t.Errorf("feed cursor = %d, want %d (All feeds, BBC Sport, CBR)", got, want)
	}
	if m.feedID != second.FeedID {
		t.Error("stored feed was dropped")
	}
}

func TestScrollPercentShownInDetail(t *testing.T) {
	post := testPost("Prvi")
	post.Description.String = "<p>" + strings.Repeat("rec ", 500) + "</p>"
	post.Description.Valid = true

	m := loaded(t, []database.Post{post})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !strings.Contains(m.View(), "0%") {
		t.Errorf("scroll indicator missing at the top:\n%s", m.View())
	}

	m.viewport.GotoBottom()
	if !strings.Contains(m.View(), "100%") {
		t.Errorf("scroll indicator missing at the bottom:\n%s", m.View())
	}
}

func TestDetailHeaderHeightIsStableWithScroll(t *testing.T) {
	post := testPost("Prvi")

	for _, pct := range []float64{0, 0.5, 1} {
		got := renderDetailHeader(post, 80, pct)
		if n := strings.Count(got, "\n"); n != detailChromeHeight-2 {
			t.Errorf("scroll %v: header = %d newlines, want %d", pct, n, detailChromeHeight-2)
		}
	}
}
