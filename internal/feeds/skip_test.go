package feeds

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

func dated(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

func agedFeed(items int, newest time.Duration, step time.Duration) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<rss version=\"2.0\"><channel><title>Aged</title><link>https://aged.test</link>\n")
	for i := range items {
		when := time.Now().Add(-newest - time.Duration(i)*step)
		fmt.Fprintf(&b, "  <item><title>Item %02d</title><link>https://aged.test/%d</link><pubDate>%s</pubDate></item>\n",
			i, i, when.Format(time.RFC1123Z))
	}
	b.WriteString("</channel></rss>")
	return b.String()
}

func scrapeOnce(t *testing.T, db *testdb.DB, name, url string, p Policy) Result {
	t.Helper()

	feed, err := db.GetFeedByUrl(t.Context(), url)
	if err != nil {
		user := db.User(t, name)
		feed = db.Feed(t, user.ID, name, url)
	}

	res := Scrape(t.Context(), db.Queries, feed, p)
	if res.Err != nil {
		t.Fatalf("Scrape: %v", res.Err)
	}
	return res
}

func TestScrapeStoresNothingItWouldAlreadyKeepTwice(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, agedFeed(30, 0, 12*time.Hour))

	first := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())
	if first.Items != 30 {
		t.Fatalf("the feed served %d items, want 30", first.Items)
	}
	if first.Skipped != 0 {
		t.Errorf("the first scrape skipped %d items, want 0 — the feed held nothing yet, so nothing was prunable", first.Skipped)
	}
	if first.Saved != 30 {
		t.Errorf("the first scrape saved %d of 30", first.Saved)
	}

	if _, err := Prune(t.Context(), db.Queries, DefaultPolicy()); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	left := db.Count(t, "SELECT count(*) FROM posts")
	if left >= 30 {
		t.Fatalf("%d posts survived the prune, want fewer than 30 so there is churn to stop", left)
	}

	second := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())
	if second.Saved != 0 {
		t.Errorf("the second scrape re-inserted %d posts the prune had just deleted — this is the churn K1 exists to stop", second.Saved)
	}
	if second.Skipped == 0 {
		t.Error("the second scrape skipped nothing, so it must have tried every insert again")
	}
	if got := db.Count(t, "SELECT count(*) FROM posts"); got != left {
		t.Errorf("post count moved from %d to %d across a scrape that should have changed nothing", left, got)
	}
}

func TestScrapeStillFillsAFeedWhoseEveryItemIsPastRetention(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, agedFeed(6, 30*24*time.Hour, 24*time.Hour))

	res := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())

	if res.Skipped != 0 {
		t.Errorf("skipped %d of %d items from a feed that publishes slower than the retention window; "+
			"that is the empty-feed bug keep_per_feed was added to fix", res.Skipped, res.Items)
	}
	if got := db.Count(t, "SELECT count(*) FROM posts"); got != 6 {
		t.Errorf("%d posts stored, want 6 — a monthly blog must not come out empty", got)
	}
}

func TestScrapeKeepsFillingUpToTheKeepWindow(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, agedFeed(14, 30*24*time.Hour, 24*time.Hour))

	res := scrapeOnce(t, db, "alice", srv.URL, Policy{Retention: DefaultRetention, KeepPerFeed: 10})

	if res.Saved != 14 {
		t.Errorf("saved %d of 14 — the cutoff is recomputed once per round, so a first fill must take everything", res.Saved)
	}

	if _, err := Prune(t.Context(), db.Queries, Policy{Retention: DefaultRetention, KeepPerFeed: 10}); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if got := db.Count(t, "SELECT count(*) FROM posts"); got != 10 {
		t.Fatalf("%d posts after prune, want the newest 10", got)
	}

	again := scrapeOnce(t, db, "alice", srv.URL, Policy{Retention: DefaultRetention, KeepPerFeed: 10})
	if again.Saved != 0 {
		t.Errorf("re-inserted %d of the four posts prune dropped", again.Saved)
	}
	if again.Skipped != 4 {
		t.Errorf("skipped %d items, want exactly the 4 that sit below the keep window", again.Skipped)
	}
}

func TestScrapeNeverSkipsAnythingInsideTheRetentionWindow(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, agedFeed(20, 0, time.Hour))

	res := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())
	if res.Saved != 20 || res.Skipped != 0 {
		t.Fatalf("saved=%d skipped=%d, want 20 and 0", res.Saved, res.Skipped)
	}

	again := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())
	if again.Skipped != 0 {
		t.Errorf("skipped %d items that are all newer than the retention window", again.Skipped)
	}
}

func TestAZeroPolicySkipsNothingAtAll(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, agedFeed(20, 30*24*time.Hour, 24*time.Hour))

	res := scrapeOnce(t, db, "alice", srv.URL, Policy{})
	if res.Skipped != 0 {
		t.Errorf("a zero Policy skipped %d items; the safe reading of 'no policy' is to store everything", res.Skipped)
	}
	if res.Saved != 20 {
		t.Errorf("saved %d of 20 under a zero Policy", res.Saved)
	}
}

func TestKeepPerFeedZeroSkipsEverythingPastRetention(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, agedFeed(8, 30*24*time.Hour, 24*time.Hour))

	res := scrapeOnce(t, db, "alice", srv.URL, Policy{Retention: DefaultRetention})
	if res.Skipped != 8 {
		t.Errorf("skipped %d of 8; with no keep window, age-only prune would delete all of them", res.Skipped)
	}
	if got := db.Count(t, "SELECT count(*) FROM posts"); got != 0 {
		t.Errorf("%d posts stored, want 0 — every one would be deleted on the same tick", got)
	}
}

func TestAnItemWithNoParsableDateIsNeverSkipped(t *testing.T) {
	db := testdb.Open(t)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<rss version=\"2.0\"><channel><title>Undated</title><link>https://undated.test</link>\n")
	for i := range int(DefaultKeepPerFeed) + 4 {
		when := time.Now().Add(-30*24*time.Hour - time.Duration(i)*24*time.Hour)
		fmt.Fprintf(&b, "  <item><title>Dated %02d</title><link>https://undated.test/d%d</link><pubDate>%s</pubDate></item>\n",
			i, i, when.Format(time.RFC1123Z))
	}
	b.WriteString("  <item><title>No date at all</title><link>https://undated.test/1</link></item>\n")
	b.WriteString("  <item><title>Unparsable</title><link>https://undated.test/2</link><pubDate>last Tuesday</pubDate></item>\n")
	b.WriteString("</channel></rss>")

	srv := feedServer(t, b.String())

	first := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())
	if first.Skipped != 0 {
		t.Fatalf("the first scrape skipped %d items, want 0", first.Skipped)
	}

	second := scrapeOnce(t, db, "alice", srv.URL, DefaultPolicy())
	if second.Skipped != 6 {
		t.Errorf("skipped %d items, want 6. The two undated items COALESCE to created_at, so they sit at "+
			"the top of the keep window and push six dated ones below it. Skipping 8 would mean the "+
			"undated pair was skipped too, and prune would never have deleted those", second.Skipped)
	}
	if got := db.Count(t, "SELECT count(*) FROM posts WHERE published_at IS NULL"); got != 2 {
		t.Errorf("%d undated posts stored, want 2", got)
	}
}

func TestWorthlessNeedsAgeAndACutoffTogether(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	old := dated(now.Add(-10 * 24 * time.Hour))
	fresh := dated(now.Add(-time.Hour))

	cases := []struct {
		name string
		w    prunableWindow
		give time.Time
		want bool
	}{
		{"staro i ispod praga", prunableWindow{retention: DefaultRetention, hasCutoff: true, cutoff: now.Add(-24 * time.Hour)}, old.Time, true},
		{"staro ali iznad praga", prunableWindow{retention: DefaultRetention, hasCutoff: true, cutoff: now.Add(-30 * 24 * time.Hour)}, old.Time, false},
		{"staro i tacno na pragu", prunableWindow{retention: DefaultRetention, hasCutoff: true, cutoff: now.Add(-10 * 24 * time.Hour)}, old.Time, false},
		{"svezo i ispod praga", prunableWindow{retention: DefaultRetention, hasCutoff: true, cutoff: now}, fresh.Time, false},
		{"nema praga", prunableWindow{retention: DefaultRetention}, old.Time, false},
		{"nema retencije", prunableWindow{hasCutoff: true, cutoff: now}, old.Time, false},
		{"prag ne postoji ali se nista ne cuva", prunableWindow{retention: DefaultRetention, keepsNothing: true}, old.Time, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.w.worthless(dated(c.give), now); got != c.want {
				t.Errorf("worthless() = %v, want %v", got, c.want)
			}
		})
	}
}
