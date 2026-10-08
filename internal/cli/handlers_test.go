package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"
)

func capture(t *testing.T, run func() error) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	saved := os.Stdout
	os.Stdout = w
	runErr := run()
	os.Stdout = saved
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured output: %v", err)
	}
	return string(out), runErr
}

func fakeState(f *storetest.Fake) *state {
	return &state{Store: f, Cfg: &config.Config{CurrentUserName: f.Identity.Name}}
}

func TestBookmarkSaysAlreadyBookmarkedTheSecondTime(t *testing.T) {
	f := storetest.New("marko")
	post := f.Post("Go 1.26 is out", "https://go.dev/blog/go1.26")
	s := fakeState(f)
	cmd := command{Name: "bookmark", Args: []string{post.Url}}

	first, err := capture(t, func() error { return handlerBookmark(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("first bookmark: %v", err)
	}
	if !strings.Contains(first, `Bookmarked "Go 1.26 is out"`) {
		t.Errorf("first bookmark printed %q", first)
	}

	second, err := capture(t, func() error { return handlerBookmark(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("second bookmark: %v", err)
	}
	if !strings.Contains(second, "Already bookmarked") {
		t.Errorf("second bookmark printed %q, want the already-bookmarked notice", second)
	}
}

func TestBookmarkUnknownURLFails(t *testing.T) {
	f := storetest.New("marko")
	s := fakeState(f)
	cmd := command{Name: "bookmark", Args: []string{"https://example.com/nope"}}

	_, err := capture(t, func() error { return handlerBookmark(context.Background(), s, cmd, f.Identity) })
	if err == nil {
		t.Fatal("bookmarking an unknown url succeeded")
	}
	if !strings.Contains(err.Error(), "post not found") {
		t.Errorf("error was %q, want it to say the post was not found", err)
	}
}

func TestCategorizeTellsYouToFollowFirst(t *testing.T) {
	f := storetest.New("marko")
	s := fakeState(f)
	url := "https://news.ycombinator.com/rss"
	cmd := command{Name: "categorize", Args: []string{url, "tech"}}

	_, err := capture(t, func() error { return handlerCategorize(context.Background(), s, cmd, f.Identity) })
	if err == nil {
		t.Fatal("categorizing a feed we do not follow succeeded")
	}
	if !strings.Contains(err.Error(), "gator follow "+url) {
		t.Errorf("error was %q, want the follow hint with the url", err)
	}
}

func TestCategorizeMovesAFollowedFeed(t *testing.T) {
	f := storetest.New("marko")
	sub := f.Sub("Hacker News", "https://news.ycombinator.com/rss", "")
	s := fakeState(f)
	cmd := command{Name: "categorize", Args: []string{sub.FeedUrl, "tech"}}

	out, err := capture(t, func() error { return handlerCategorize(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("categorize: %v", err)
	}
	if !strings.Contains(out, "Moved Hacker News to tech") {
		t.Errorf("printed %q", out)
	}
	if f.Subs[0].Category != "tech" {
		t.Errorf("category is %q, want tech", f.Subs[0].Category)
	}
}

func TestCategorizeToEmptyMovesToRoot(t *testing.T) {
	f := storetest.New("marko")
	sub := f.Sub("Hacker News", "https://news.ycombinator.com/rss", "tech")
	s := fakeState(f)
	cmd := command{Name: "categorize", Args: []string{sub.FeedUrl, "  "}}

	out, err := capture(t, func() error { return handlerCategorize(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("categorize: %v", err)
	}
	if !strings.Contains(out, "Moved Hacker News to the root") {
		t.Errorf("printed %q", out)
	}
	if f.Subs[0].Category != "" {
		t.Errorf("category is %q, want it empty", f.Subs[0].Category)
	}
}

func TestFollowingGroupsByFolderWithUncategorizedLast(t *testing.T) {
	f := storetest.New("marko")
	f.Sub("Loose End", "https://loose.example/feed", "")
	f.Sub("Zed Blog", "https://zed.dev/blog/feed", "tools")
	f.Sub("Hacker News", "https://news.ycombinator.com/rss", "tech")
	f.Sub("Dan Luu", "https://danluu.com/atom.xml", "tech")
	s := fakeState(f)

	out, err := capture(t, func() error {
		return handlerFollowing(context.Background(), s, command{Name: "following"}, f.Identity)
	})
	if err != nil {
		t.Fatalf("following: %v", err)
	}

	order := []string{"tech", "tools", rootLabel}
	at := make([]int, len(order))
	for i, label := range order {
		at[i] = strings.Index(out, label)
		if at[i] < 0 {
			t.Fatalf("output is missing %q:\n%s", label, out)
		}
	}
	if !(at[0] < at[1] && at[1] < at[2]) {
		t.Errorf("folders are out of order, want tech, tools, then %s:\n%s", rootLabel, out)
	}

	if dan, hn := strings.Index(out, "Dan Luu"), strings.Index(out, "Hacker News"); dan > hn {
		t.Errorf("feeds inside a folder are not sorted by name:\n%s", out)
	}
	if !strings.Contains(out, "tech") || !strings.Contains(out, "(2)") {
		t.Errorf("tech should report two feeds:\n%s", out)
	}
}

func TestSearchSaysWhenNothingMatches(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Go 1.26 is out", "https://go.dev/blog/go1.26")
	s := fakeState(f)

	out, err := capture(t, func() error {
		return handlerSearch(context.Background(), s, command{Name: "search", Args: []string{"rust"}}, f.Identity)
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, `No posts found matching "rust"`) {
		t.Errorf("printed %q", out)
	}
}

func TestSearchPassesTheQueryAndLimitToTheStore(t *testing.T) {
	f := storetest.New("marko")
	f.Post("Go 1.26 is out", "https://go.dev/blog/go1.26")
	s := fakeState(f)
	cmd := command{Name: "search", Args: []string{"--limit", "5", "go", "1.26"}}

	out, err := capture(t, func() error { return handlerSearch(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(out, "Go 1.26 is out") {
		t.Errorf("printed %q", out)
	}

	if len(f.Asked) != 1 {
		t.Fatalf("store saw %d queries, want 1", len(f.Asked))
	}
	if got := f.Asked[0]; got.Query != "go 1.26" || got.Limit != 5 {
		t.Errorf("store was asked %+v, want query %q and limit 5", got, "go 1.26")
	}
}

func TestBrowseRejectsBadFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"sort", []string{"--no-tui", "--sort", "sideways"}, "invalid sort"},
		{"limit", []string{"--no-tui", "--limit", "0"}, "invalid limit"},
		{"page", []string{"--no-tui", "--page", "0"}, "invalid page"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := storetest.New("marko")
			s := fakeState(f)
			cmd := command{Name: "browse", Args: c.args}

			_, err := capture(t, func() error { return handlerBrowse(context.Background(), s, cmd, f.Identity) })
			if err == nil {
				t.Fatalf("browse %v succeeded", c.args)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error was %q, want it to contain %q", err, c.want)
			}
			if len(f.Asked) != 0 {
				t.Errorf("browse hit the store before validating its flags: %+v", f.Asked)
			}
		})
	}
}

func TestBrowsePagesWithLimitAndPage(t *testing.T) {
	f := storetest.New("marko")
	for _, title := range []string{"first", "second", "third", "fourth"} {
		f.Post(title, "https://example.com/"+title)
	}
	s := fakeState(f)
	cmd := command{Name: "browse", Args: []string{"--no-tui", "--limit", "2", "--page", "2"}}

	out, err := capture(t, func() error { return handlerBrowse(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("browse: %v", err)
	}

	if len(f.Asked) != 1 {
		t.Fatalf("store saw %d queries, want 1", len(f.Asked))
	}
	if got := f.Asked[0]; got.Limit != 2 || got.Offset != 2 {
		t.Errorf("store was asked limit %d offset %d, want 2 and 2", got.Limit, got.Offset)
	}
	if !strings.Contains(out, "third") || !strings.Contains(out, "fourth") {
		t.Errorf("page 2 printed %q", out)
	}
	if strings.Contains(out, "first") {
		t.Errorf("page 2 printed page 1's posts:\n%s", out)
	}
}

func TestUnfollowReportsTheFeedItRemoved(t *testing.T) {
	f := storetest.New("marko")
	sub := f.Sub("Zed Blog", "https://zed.dev/blog/feed", "tools")
	s := fakeState(f)
	cmd := command{Name: "unfollow", Args: []string{sub.FeedUrl}}

	out, err := capture(t, func() error { return handlerUnfollow(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("unfollow: %v", err)
	}
	if !strings.Contains(out, "Unfollowed Zed Blog") {
		t.Errorf("printed %q", out)
	}
	if len(f.Subs) != 0 {
		t.Errorf("still following %d feeds", len(f.Subs))
	}
}

func TestAddFeedDistinguishesNewFromAlreadyKnown(t *testing.T) {
	f := storetest.New("marko")
	s := fakeState(f)
	cmd := command{Name: "addfeed", Args: []string{"Zed Blog", "https://zed.dev/blog/feed"}}

	first, err := capture(t, func() error { return handlerAddFeed(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("first addfeed: %v", err)
	}
	if !strings.Contains(first, `Added feed "Zed Blog"`) {
		t.Errorf("first addfeed printed %q", first)
	}

	second, err := capture(t, func() error { return handlerAddFeed(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("second addfeed: %v", err)
	}
	if !strings.Contains(second, "Following existing feed") {
		t.Errorf("second addfeed printed %q", second)
	}
}

func TestArticlePrintsTheStoredTextWithoutRefetching(t *testing.T) {
	f := storetest.New("marko")
	post := f.Post("Go 1.26 is out", "https://go.dev/blog/go1.26")
	f.AllPosts[0].FullText = "the whole article"
	s := fakeState(f)
	cmd := command{Name: "article", Args: []string{post.Url}}

	out, err := capture(t, func() error { return handlerArticle(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("article: %v", err)
	}
	if !strings.Contains(out, "the whole article") {
		t.Errorf("printed %q", out)
	}
}

func TestArticleSurfacesErrNoMoreText(t *testing.T) {
	f := storetest.New("marko")
	post := f.Post("Teaser only", "https://example.com/teaser")
	s := fakeState(f)
	cmd := command{Name: "article", Args: []string{post.Url}}

	_, err := capture(t, func() error { return handlerArticle(context.Background(), s, cmd, f.Identity) })
	if err == nil {
		t.Fatal("article succeeded with nothing to fetch")
	}
	if !errors.Is(err, store.ErrNoMoreText) {
		t.Errorf("error was %q, want it to wrap store.ErrNoMoreText", err)
	}
}

func TestFollowNamesTheFeedEvenWhenAlreadyFollowed(t *testing.T) {
	f := storetest.New("marko")
	sub := f.Sub("ESPN NBA", "https://www.espn.com/espn/rss/nba/news", "Basketball")
	s := fakeState(f)
	cmd := command{Name: "follow", Args: []string{sub.FeedUrl}}

	out, err := capture(t, func() error { return handlerFollow(context.Background(), s, cmd, f.Identity) })
	if err != nil {
		t.Fatalf("follow: %v", err)
	}
	if !strings.Contains(out, "Already following ESPN NBA") {
		t.Errorf("printed %q, want the feed name — a follow that hit ON CONFLICT DO NOTHING returns no row", out)
	}
}

func TestFollowAndUnfollowPassTheNotFoundErrorThrough(t *testing.T) {
	cases := []struct {
		name    string
		handler func(context.Context, *state, command, store.User) error
	}{
		{"follow", handlerFollow},
		{"unfollow", handlerUnfollow},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := storetest.New("marko")
			s := fakeState(f)
			cmd := command{Name: c.name, Args: []string{"https://example.com/nope"}}

			_, err := capture(t, func() error { return c.handler(context.Background(), s, cmd, f.Identity) })
			if !errors.Is(err, store.ErrFeedNotFound) {
				t.Fatalf("error was %v, want it to wrap store.ErrFeedNotFound", err)
			}
			if strings.Contains(err.Error(), "error "+c.name+"ing feed") {
				t.Errorf("error was %q — a missing feed should not get the action prefix on top", err)
			}
		})
	}
}
