package cli

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"
)

const smuggled = "\x1b]52;c;cm0gLXJmIH4=\x07\x1b[2J\u009b6n‮"

func TestNoCommandPrintsAFeedsControlCharacters(t *testing.T) {
	f := storetest.New("marko")
	sub := f.Sub("Evil feed"+smuggled, "https://evil.example/feed", "Folder"+smuggled)
	post := f.Post("Free money"+smuggled, "https://evil.example/a"+smuggled)
	f.AllPosts[0].Description = sql.NullString{String: "<p>body" + smuggled + "</p>", Valid: true}
	f.AllPosts[0].FullText = "whole" + smuggled
	f.FeedStats = []store.FeedStat{{FeedName: sub.FeedName, FeedUrl: sub.FeedUrl, RecentCount: 3}}
	s := fakeState(f)
	ctx := context.Background()

	runs := map[string]func() error{
		"browse": func() error {
			return handlerBrowse(ctx, s, command{Name: "browse", Args: []string{"--no-tui"}}, f.Identity)
		},
		"following": func() error { return handlerFollowing(ctx, s, command{Name: "following"}, f.Identity) },
		"stats":     func() error { return handlerStats(ctx, s, command{Name: "stats"}, f.Identity) },
		"article": func() error {
			return handlerArticle(ctx, s, command{Name: "article", Args: []string{post.Url}}, f.Identity)
		},
	}
	for name, run := range runs {
		out, err := capture(t, run)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Contains(out, "\x1b") || strings.ContainsAny(out, "\x07\u009b‮") {
			t.Errorf("%s printed a control character:\n%q", name, out)
		}
		if !strings.Contains(out, "]52;c;") {
			t.Errorf("%s printed none of the feed's text, so it proves nothing:\n%q", name, out)
		}
	}
}
