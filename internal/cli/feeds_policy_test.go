package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"net/http"
	"net/http/httptest"
)

func TestAggScrapesAndPrunesUnderOneRetentionPolicy(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<rss version=\"2.0\"><channel><title>Spread</title><link>https://spread.test</link>\n")
	for i := range 15 {
		when := time.Now().Add(-time.Hour - time.Duration(i)*12*time.Hour)
		fmt.Fprintf(&b, "  <item><title>Item %02d</title><link>https://spread.test/%d</link><pubDate>%s</pubDate></item>\n",
			i, i, when.Format(time.RFC1123Z))
	}
	b.WriteString("</channel></rss>")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, b.String())
	}))
	t.Cleanup(srv.Close)

	db := testdb.Open(t)
	user := db.User(t, "alice")
	feed := db.Feed(t, user.ID, "Spread", srv.URL)
	db.Follow(t, user.ID, feed.ID)

	if _, err := capture(t, func() error {
		scrapeFeeds(t.Context(), &state{Db: db.Queries})
		return nil
	}); err != nil {
		t.Fatalf("scrapeFeeds: %v", err)
	}

	got := db.Count(t, "SELECT count(*) FROM posts")
	want := int(feeds.DefaultKeepPerFeed)
	if got != want {
		t.Errorf("%d posts left after one agg tick, want %d. The feed spans 7 days in 12-hour steps, so a "+
			"scrape and a prune running under different retention policies leave a different number "+
			"behind — this is the one invariant that keeps them from drifting apart", got, want)
	}
}
