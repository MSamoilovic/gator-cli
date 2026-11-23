package feeds

import (
	"fmt"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

var pacific = time.FixedZone("PDT", -7*3600)

func feedAt(published time.Time) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>West</title><link>https://west.test</link>
<item><title>Late</title><link>https://west.test/late</link><pubDate>%s</pubDate></item>
</channel></rss>`, published.Format(time.RFC1123Z))
}

func TestParsePubDateReturnsTheInstantInUTC(t *testing.T) {
	got := ParsePubDate("Mon, 02 Jan 2006 15:04:05 -0700")
	if got.Time.Location() != time.UTC || got.Time.Hour() != 22 {
		t.Errorf("ParsePubDate = %v, want 2006-01-02 22:04:05 UTC", got.Time)
	}
}

func TestAPostIsStoredAtTheInstantItWasPublished(t *testing.T) {
	db := testdb.Open(t)
	published := time.Now().Add(-30 * time.Minute).Truncate(time.Second)
	srv := feedServer(t, feedAt(published.In(pacific)))

	scrapeOnce(t, db, "alice", srv.URL, Policy{})

	var stored time.Time
	db.QueryRow(t, &stored, "SELECT published_at FROM posts WHERE url = 'https://west.test/late'")
	if !stored.Equal(published) {
		t.Errorf("published_at = %v, want %v: a -07:00 feed was stored on its own wall clock", stored, published.UTC())
	}
}

func TestScrapeAndPruneAgreeOnAPostFromAWesternOffset(t *testing.T) {
	saved := time.Local
	time.Local = time.FixedZone("CEST", 2*3600)
	t.Cleanup(func() { time.Local = saved })

	db := testdb.Open(t)
	p := Policy{Retention: 72 * time.Hour, KeepPerFeed: 0}
	published := time.Now().Add(-71 * time.Hour)
	srv := feedServer(t, feedAt(published.In(pacific)))

	if res := scrapeOnce(t, db, "alice", srv.URL, p); res.Saved != 1 {
		t.Fatalf("saved %d, skipped %d: a post 71h old is inside a 72h window wherever it was written", res.Saved, res.Skipped)
	}
	if _, err := Prune(t.Context(), db.Queries, p); err != nil {
		t.Fatal(err)
	}
	if n := db.Count(t, "SELECT count(*) FROM posts"); n != 1 {
		t.Errorf("prune left %d posts, want the 71h-old one kept", n)
	}
}
