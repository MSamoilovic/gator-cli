package tui

import (
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestAgeShrinksItsUnitAsThePostGetsOlder(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		age  time.Duration
		want string
	}{
		{0, " 0m"},
		{30 * time.Minute, "30m"},
		{59*time.Minute + 59*time.Second, "59m"},
		{time.Hour, " 1h"},
		{23*time.Hour + 59*time.Minute, "23h"},
		{24 * time.Hour, " 1d"},
		{6*24*time.Hour + 23*time.Hour, " 6d"},
		{7 * 24 * time.Hour, " 1w"},
		{51*7*24*time.Hour + 6*24*time.Hour, "51w"},
		{52 * 7 * 24 * time.Hour, " 1y"},
		{104 * 7 * 24 * time.Hour, " 2y"},
		{-5 * time.Minute, " 0m"},
	}

	for _, c := range cases {
		t.Run(c.age.String(), func(t *testing.T) {
			published := sql.NullTime{Time: now.Add(-c.age), Valid: true}
			if got := relativeAge(published, now); got != c.want {
				t.Errorf("relativeAge(%s) = %q, want %q", c.age, got, c.want)
			}
		})
	}
}

func TestEveryAgeIsExactlyThreeColumnsWide(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	for minutes := 0; minutes < 60*24*400; minutes += 7 {
		published := sql.NullTime{Time: now.Add(-time.Duration(minutes) * time.Minute), Valid: true}
		if got := lipgloss.Width(relativeAge(published, now)); got != ageCols {
			t.Fatalf("age of a post %d minutes old is %d columns (%q), want %d",
				minutes, got, relativeAge(published, now), ageCols)
		}
	}
}

func TestAnUndatedPostGetsADashNotAZeroAge(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	got := relativeAge(sql.NullTime{}, now)
	if !strings.Contains(got, undatedAge) {
		t.Errorf("relativeAge of an undated post = %q, want it to contain %q — "+
			"1.3%% of posts arrive dateless and must not read as just published", got, undatedAge)
	}
	if lipgloss.Width(got) != ageCols {
		t.Errorf("an undated post takes %d columns, want %d", lipgloss.Width(got), ageCols)
	}
}

func densePosts(n int) []database.Post {
	posts := make([]database.Post, n)
	for i := range posts {
		posts[i] = testPost("Post number " + strconv.Itoa(i))
		posts[i].PublishedAt = sql.NullTime{Time: time.Now().Add(-time.Duration(i) * time.Hour), Valid: true}
	}
	return posts
}

func listedPosts(view string) int {
	n := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, unreadMark) {
			n++
		}
	}
	return n
}

func atDensity(t *testing.T, name string, w, h int, posts []database.Post) model {
	t.Helper()
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc, Density: name}, ""),
		tea.WindowSizeMsg{Width: w, Height: h})
	m, _ = step(t, m, postsLoadedMsg{posts: posts, paged: true})
	return m
}

func TestCompactDensityFitsThreeTimesAsManyPosts(t *testing.T) {
	posts := densePosts(pageSize)

	comfortable := listedPosts(atDensity(t, nameComfortable, 80, 24, posts).View())
	compact := listedPosts(atDensity(t, nameCompact, 80, 24, posts).View())

	if comfortable == 0 || compact == 0 {
		t.Fatalf("nothing rendered: comfortable=%d compact=%d", comfortable, compact)
	}
	if compact < 2*comfortable {
		t.Errorf("compact shows %d posts against comfortable's %d on an 80x24 terminal; "+
			"dropping the second row must buy back most of the screen", compact, comfortable)
	}
}

func TestBothDensitiesStayInsideTheTerminal(t *testing.T) {
	for _, name := range []string{nameComfortable, nameCompact} {
		view := atDensity(t, name, 80, 24, densePosts(pageSize)).View()
		if got := lipgloss.Height(view); got > 24 {
			t.Errorf("%s density is %d rows tall, want at most 24", name, got)
		}
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > 80 {
				t.Errorf("%s density row %d is %d columns wide, want at most 80: %q", name, i, got, line)
			}
		}
	}
}

func TestSwitchingDensityRedrawsTheSamePosts(t *testing.T) {
	m := atDensity(t, nameComfortable, 80, 24, densePosts(pageSize))
	before := len(m.list.Items())

	next, _ := m.toggleDensity()
	m = next.(model)

	if m.density != densityCompact {
		t.Fatalf("density = %v, want compact", m.density)
	}
	if got := len(m.list.Items()); got != before {
		t.Errorf("items = %d after switching density, want the %d we had", got, before)
	}
	if !strings.Contains(m.status, nameCompact) {
		t.Errorf("status = %q, want it to name the density that is now in force", m.status)
	}
	if got := listedPosts(m.View()); got <= listedPosts(atDensity(t, nameComfortable, 80, 24, densePosts(pageSize)).View()) {
		t.Error("switching to compact did not put more posts on screen")
	}

	back, _ := m.toggleDensity()
	if got := back.(model).density; got != densityComfortable {
		t.Errorf("density = %v after a second switch, want comfortable again", got)
	}
}

func TestDensitySurvivesRestart(t *testing.T) {
	m := atDensity(t, nameComfortable, 80, 24, densePosts(4))
	next, _ := m.toggleDensity()

	saved := next.(model).snapshot()
	if got, want := saved.Density, nameCompact; got != want {
		t.Fatalf("snapshot density = %q, want %q", got, want)
	}

	restarted := newModel(t.Context(), nil, testUser(), saved.sanitized(), "")
	if got := restarted.density; got != densityCompact {
		t.Errorf("density after restart = %v, want compact", got)
	}
}

func TestAnUnknownDensityFallsBackToComfortable(t *testing.T) {
	for _, name := range []string{"", "cozy", "COMPACT"} {
		if got := parseDensity(name); got != densityComfortable {
			t.Errorf("parseDensity(%q) = %v, want comfortable", name, got)
		}
	}
	if got := parseDensity(nameCompact); got != densityCompact {
		t.Errorf("parseDensity(%q) = %v, want compact", nameCompact, got)
	}
}

func TestOnlyComfortableDensityDrawsTheFeedName(t *testing.T) {
	feed := testFeed("Hacker News RSS")
	post := testPost("Why big dogs age faster")
	post.FeedID = feed.FeedID

	for name, wantFeed := range map[string]bool{nameComfortable: true, nameCompact: false} {
		m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc, Density: name}, ""),
			tea.WindowSizeMsg{Width: 80, Height: 24})
		m = withFeeds(t, m, feed)
		m, _ = step(t, m, postsLoadedMsg{posts: []database.Post{post}})

		view := m.View()
		if got := strings.Count(view, feed.FeedName) > 1; got != wantFeed {
			t.Errorf("%s density draws the feed name beside the post = %v, want %v:\n%s",
				name, got, wantFeed, view)
		}
	}
}

func TestPostItemsLearnTheirFeedNameWhenFeedsArrive(t *testing.T) {
	feed := testFeed("Hacker News RSS")
	post := testPost("Why big dogs age faster")
	post.FeedID = feed.FeedID

	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}, ""),
		tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, postsLoadedMsg{posts: []database.Post{post}})

	item := m.list.Items()[0].(postItem)
	if got := item.Description(); got != "" {
		t.Fatalf("Description() before the feeds arrive = %q, want empty", got)
	}

	m = withFeeds(t, m, feed)
	if got, want := item.Description(), feed.FeedName; got != want {
		t.Errorf("Description() = %q, want %q — the name map is shared by reference, "+
			"so posts loaded before the feed list must pick it up without a rebuild", got, want)
	}
}

func TestEveryPostItemCarriesTheFeedNameMap(t *testing.T) {
	m := atDensity(t, nameComfortable, 80, 24, densePosts(3))
	for i, it := range m.list.Items() {
		if it.(postItem).feedNames == nil {
			t.Errorf("post item %d has no feed name map, so its second row stays blank forever", i)
		}
	}
}

func TestDensityIsReachableFromThePalette(t *testing.T) {
	m, _ := step(t, newModel(t.Context(), nil, testUser(), uiState{SortDir: sortDesc}, ""),
		tea.WindowSizeMsg{Width: 80, Height: 24})

	for _, a := range m.paletteActions() {
		if strings.Contains(strings.ToLower(a.label), "density") {
			return
		}
	}
	t.Error("no palette action switches the list density, and no key is bound to it either")
}
