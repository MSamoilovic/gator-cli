package tui

import (
	"strings"
	"testing"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func testFeedIn(name, category string) database.GetFeedFollowsForUserRow {
	f := testFeed(name)
	f.Category = category
	return f
}

// panelTitles su naslovi redova u feed panelu, onako kako se i crtaju.
func panelTitles(m model) []string {
	items := m.feedList.Items()
	titles := make([]string, len(items))
	for i, it := range items {
		switch v := it.(type) {
		case folderItem:
			titles[i] = v.Title()
		case feedItem:
			titles[i] = v.Title()
		}
	}
	return titles
}

func TestPanelStaysFlatWithoutCategories(t *testing.T) {
	// Jedno jedino "(uncategorized)" zaglavlje iznad svega je cista buka.
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeed("BBC"), testFeed("CBR"))

	for _, it := range m.feedList.Items() {
		if fol, ok := it.(folderItem); ok {
			t.Fatalf("uncategorised feeds produced a %q header", fol.name)
		}
	}
	if got, want := len(m.feedList.Items()), 3; got != want {
		t.Errorf("panel has %d rows, want %d", got, want)
	}
}

func TestPanelGroupsUnderHeaders(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")),
		testFeedIn("BBC", ""),
		testFeedIn("XKCD", "Comics"),
		testFeedIn("Ars", "Tech"),
	)

	got := panelTitles(m)
	want := []string{
		allFeedsLabel,
		folderOpen + " Comics",
		feedIndent + "XKCD",
		folderOpen + " Tech",
		feedIndent + "Ars",
		folderOpen + " " + rootFolder,
		feedIndent + "BBC",
	}

	if len(got) != len(want) {
		t.Fatalf("panel =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEnterOnHeaderCollapsesTheFolder(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")),
		testFeedIn("XKCD", "Comics"),
		testFeedIn("Ars", "Tech"),
	)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})   // fokus na feedove
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown})  // na "Comics"
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // sklopi

	if !m.collapsed["Comics"] {
		t.Fatal("enter on a header did not collapse the folder")
	}
	for _, title := range panelTitles(m) {
		if strings.Contains(title, "XKCD") {
			t.Error("a collapsed folder still shows its feeds")
		}
	}
	if !strings.HasPrefix(panelTitles(m)[1], folderClosed) {
		t.Errorf("header did not switch to the closed arrow: %q", panelTitles(m)[1])
	}

	// Fokus ostaje na istom zaglavlju, jer se broj redova ispod promenio.
	if fol, ok := m.feedList.SelectedItem().(folderItem); !ok || fol.name != "Comics" {
		t.Errorf("selection moved off the folder that was toggled: %v", m.feedList.SelectedItem())
	}

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // rasklopi
	if m.collapsed["Comics"] {
		t.Error("enter did not expand the folder again")
	}
	if !strings.Contains(strings.Join(panelTitles(m), "\n"), "XKCD") {
		t.Error("expanding did not bring the feeds back")
	}
}

func TestCollapsingOneFolderLeavesTheOthers(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")),
		testFeedIn("XKCD", "Comics"),
		testFeedIn("Ars", "Tech"),
	)
	m, _ = m.toggleFolder("Comics")

	joined := strings.Join(panelTitles(m), "\n")
	if strings.Contains(joined, "XKCD") {
		t.Error("Comics is collapsed but still shows XKCD")
	}
	if !strings.Contains(joined, "Ars") {
		t.Error("collapsing Comics also hid Tech")
	}
}

func TestFolderHeaderCountsUnreadAcrossItsFeeds(t *testing.T) {
	a := testFeedIn("Ars", "Tech")
	b := testFeedIn("Mono", "Tech")
	m := withFeeds(t, ready(t, testPost("Prvi")), a, b)

	m.unread[a.FeedID] = 3
	m.unread[b.FeedID] = 4

	for _, it := range m.feedList.Items() {
		if fol, ok := it.(folderItem); ok && fol.name == "Tech" {
			if got := fol.unreadTotal(); got != 7 {
				t.Errorf("folder unread = %d, want 7", got)
			}
			if !strings.Contains(fol.Title(), "(7)") {
				t.Errorf("header = %q, want it to show (7)", fol.Title())
			}
			return
		}
	}
	t.Fatal("no Tech header in the panel")
}

func TestFolderHeaderMarksBrokenFeeds(t *testing.T) {
	// Pokvaren feed u sklopljenom folderu bi inace bio nevidljiv.
	broken := testFeedIn("Ars", "Tech")
	broken.FeedFailures = 4
	m := withFeeds(t, ready(t, testPost("Prvi")), broken)

	for _, it := range m.feedList.Items() {
		if fol, ok := it.(folderItem); ok && fol.name == "Tech" {
			if fol.broken != 1 {
				t.Errorf("folder broken count = %d, want 1", fol.broken)
			}
			if !strings.Contains(fol.Title(), brokenMark) {
				t.Errorf("header = %q, want it to carry %q", fol.Title(), brokenMark)
			}
			return
		}
	}
	t.Fatal("no Tech header in the panel")
}

func TestUnfollowOnHeaderIsRefused(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")), testFeedIn("XKCD", "Comics"))

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown}) // na zaglavlje
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	if m.confirming {
		t.Fatal("unfollow asked to confirm on a folder header")
	}
	if !strings.Contains(m.status, "feed") {
		t.Errorf("status = %q, want it to say to pick a feed", m.status)
	}
}

func TestStoredFeedIsRevealedInsideACollapsedFolder(t *testing.T) {
	// Zapamcen izbor ne sme da ostane sakriven iza sklopljenog zaglavlja.
	feed := testFeedIn("Ars", "Tech")

	m := ready(t, testPost("Prvi"))
	m.feedID = feed.FeedID
	m.collapsed["Tech"] = true
	m = withFeeds(t, m, feed)

	if m.collapsed["Tech"] {
		t.Error("the folder holding the stored feed stayed collapsed")
	}

	found := false
	for _, it := range m.feedList.Items() {
		if fi, ok := it.(feedItem); ok && fi.id == feed.FeedID {
			found = true
		}
	}
	if !found {
		t.Error("the stored feed is not in the panel")
	}
}

func TestCatalogSeesFeedsInCollapsedFolders(t *testing.T) {
	// followedURLs cita m.feeds, ne listu — inace bi katalog za sklopljene
	// foldere tvrdio da se ti feedovi ne prate.
	feed := testFeedIn("Ars", "Tech")
	feed.FeedUrl = "https://arstechnica.test/rss"

	m := withFeeds(t, ready(t, testPost("Prvi")), feed)
	m, _ = m.toggleFolder("Tech")

	if !m.followedURLs()[feed.FeedUrl] {
		t.Error("a feed inside a collapsed folder is reported as not followed")
	}
}

func TestCollapsedFoldersSurviveARestart(t *testing.T) {
	m := withFeeds(t, ready(t, testPost("Prvi")),
		testFeedIn("XKCD", "Comics"),
		testFeedIn("Ars", "Tech"),
	)
	m, _ = m.toggleFolder("Tech")

	saved := m.snapshot()
	if len(saved.Collapsed) != 1 || saved.Collapsed[0] != "Tech" {
		t.Fatalf("snapshot collapsed = %v, want [Tech]", saved.Collapsed)
	}

	restored := collapsedSet(saved.Collapsed)
	if !restored["Tech"] || restored["Comics"] {
		t.Errorf("restored collapsed = %v", restored)
	}
}

func TestSelectingAFeedInsideAFolderStillFilters(t *testing.T) {
	feed := testFeedIn("Ars", "Tech")
	m := withFeeds(t, ready(t, testPost("Prvi")), feed)

	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyTab})
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown}) // zaglavlje Tech
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyDown}) // feed Ars
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.feedID != feed.FeedID {
		t.Errorf("selected feed = %v, want %v", m.feedID, feed.FeedID)
	}
	if m.feedName != "Ars" {
		t.Errorf("feed name = %q, want Ars", m.feedName)
	}
	if m.focus != focusPosts {
		t.Error("selecting a feed did not move focus back to the posts")
	}
}

func TestUnknownFeedIDDoesNotExpandAnything(t *testing.T) {
	m := ready(t, testPost("Prvi"))
	m.feedID = uuid.New()
	m.collapsed["Tech"] = true
	m = withFeeds(t, m, testFeedIn("Ars", "Tech"))

	if !m.collapsed["Tech"] {
		t.Error("a stored feed that is no longer followed expanded a folder anyway")
	}
}
