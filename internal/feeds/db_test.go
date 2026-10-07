package feeds

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

const liveFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>The Go Blog</title>
  <link>https://go.test</link>
  <item><title>First</title><link>https://go.test/1</link><pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate></item>
  <item><title>Second</title><link>https://go.test/2</link><pubDate>Tue, 03 Jan 2006 15:04:05 -0700</pubDate></item>
</channel></rss>`

func feedServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFollowIsIdempotent(t *testing.T) {
	db := testdb.Open(t)

	user := db.User(t, "alice")
	feed := db.Feed(t, user.ID, "Go Blog", "https://go.test/rss")

	if _, created, err := Follow(t.Context(), db.Queries, user.ID, feed.ID); err != nil || !created {
		t.Fatalf("first Follow: created=%v err=%v, want created=true", created, err)
	}
	if _, created, err := Follow(t.Context(), db.Queries, user.ID, feed.ID); err != nil || created {
		t.Fatalf("second Follow: created=%v err=%v, want created=false and no error", created, err)
	}

	if n := db.Count(t, "SELECT count(*) FROM feed_follows WHERE user_id = $1", user.ID); n != 1 {
		t.Errorf("following twice produced %d rows, want 1", n)
	}
}

func TestAddDerivesTheNameFromTheFeedTitle(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")
	srv := feedServer(t, liveFeed)

	feed, created, err := Add(t.Context(), db.Queries, user.ID, "", srv.URL)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !created {
		t.Error("Add reported the feed as already known")
	}
	if feed.Name != "The Go Blog" {
		t.Errorf("name = %q, want %q from <title>", feed.Name, "The Go Blog")
	}
}

func TestAddKeepsAnExplicitName(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")
	srv := feedServer(t, liveFeed)

	feed, _, err := Add(t.Context(), db.Queries, user.ID, "My Name", srv.URL)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if feed.Name != "My Name" {
		t.Errorf("name = %q, want the name that was passed in", feed.Name)
	}
}

func TestAddingAKnownFeedFollowsItInstead(t *testing.T) {
	db := testdb.Open(t)
	srv := feedServer(t, liveFeed)

	alice := db.User(t, "alice")
	bob := db.User(t, "bob")

	first, created, err := Add(t.Context(), db.Queries, alice.ID, "", srv.URL)
	if err != nil || !created {
		t.Fatalf("alice Add: created=%v err=%v", created, err)
	}

	second, created, err := Add(t.Context(), db.Queries, bob.ID, "", srv.URL)
	if err != nil {
		t.Fatalf("bob Add: %v", err)
	}
	if created {
		t.Error("bob adding a known URL reported it as a new feed")
	}
	if second.ID != first.ID {
		t.Error("bob got a second feed row for the same URL")
	}

	if n := db.Count(t, "SELECT count(*) FROM feeds WHERE url = $1", srv.URL); n != 1 {
		t.Errorf("%d feed rows for one URL, want 1", n)
	}
	if n := db.Count(t, "SELECT count(*) FROM feed_follows WHERE feed_id = $1", first.ID); n != 2 {
		t.Errorf("%d follows on the shared feed, want 2", n)
	}
}

func TestScrapeSavesPostsThenSkipsTheDuplicates(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")
	srv := feedServer(t, liveFeed)

	feed := db.Feed(t, user.ID, "Go Blog", srv.URL)

	first := Scrape(t.Context(), db.Queries, feed)
	if first.Err != nil {
		t.Fatalf("first Scrape: %v", first.Err)
	}
	if first.Items != 2 || first.Saved != 2 {
		t.Errorf("first Scrape: items=%d saved=%d, want 2 and 2", first.Items, first.Saved)
	}

	fresh, err := db.GetFeedByUrl(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("reloading feed: %v", err)
	}

	second := Scrape(t.Context(), db.Queries, fresh)
	if second.Err != nil {
		t.Fatalf("second Scrape: %v", second.Err)
	}
	if second.Saved != 0 {
		t.Errorf("second Scrape saved %d posts, want 0 — duplicates must be skipped", second.Saved)
	}

	if n := db.Count(t, "SELECT count(*) FROM posts WHERE feed_id = $1", feed.ID); n != 2 {
		t.Errorf("%d posts after scraping twice, want 2", n)
	}
}

func TestScrapeRecordsAndThenClearsFailure(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	var ok bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, liveFeed)
	}))
	t.Cleanup(srv.Close)

	feed := db.Feed(t, user.ID, "Flaky", srv.URL)

	if res := Scrape(t.Context(), db.Queries, feed); res.Err == nil {
		t.Fatal("Scrape of a 500 reported no error")
	}

	broken, err := db.GetFeedByUrl(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("reloading feed: %v", err)
	}
	if broken.FailureCount != 1 {
		t.Errorf("failure_count = %d, want 1", broken.FailureCount)
	}
	if broken.LastError == "" {
		t.Error("last_error is empty after a failed fetch")
	}

	ok = true
	if res := Scrape(t.Context(), db.Queries, broken); res.Err != nil {
		t.Fatalf("Scrape after recovery: %v", res.Err)
	}

	healthy, err := db.GetFeedByUrl(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("reloading feed: %v", err)
	}
	if healthy.FailureCount != 0 || healthy.LastError != "" {
		t.Errorf("after a good fetch: failure_count=%d last_error=%q, want 0 and empty",
			healthy.FailureCount, healthy.LastError)
	}
}

func TestScrapeStoresConditionalGetValidators(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		fmt.Fprint(w, liveFeed)
	}))
	t.Cleanup(srv.Close)

	feed := db.Feed(t, user.ID, "Go Blog", srv.URL)

	if res := Scrape(t.Context(), db.Queries, feed); res.Err != nil {
		t.Fatalf("first Scrape: %v", res.Err)
	}

	stored, err := db.GetFeedByUrl(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("reloading feed: %v", err)
	}
	if stored.Etag != `"v1"` {
		t.Fatalf("etag = %q, want %q", stored.Etag, `"v1"`)
	}

	second := Scrape(t.Context(), db.Queries, stored)
	if second.Err != nil {
		t.Fatalf("second Scrape: %v", second.Err)
	}
	if !second.NotModified {
		t.Error("the second Scrape did not report NotModified despite sending the stored ETag")
	}
}

func TestPruneDeletesOnlyPostsPastTheRetention(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")
	feed := db.Feed(t, user.ID, "Go Blog", "https://go.test/rss")

	db.Post(t, feed.ID, "Ancient", "https://go.test/old", time.Now().Add(-100*time.Hour))
	db.Post(t, feed.ID, "Recent", "https://go.test/new", time.Now().Add(-1*time.Hour))

	n, err := Prune(t.Context(), db.Queries, DefaultRetention)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("Prune deleted %d posts, want 1", n)
	}

	if db.Count(t, "SELECT count(*) FROM posts WHERE title = 'Recent'") != 1 {
		t.Error("Prune deleted a post inside the retention window")
	}
	if db.Count(t, "SELECT count(*) FROM posts WHERE title = 'Ancient'") != 0 {
		t.Error("Prune left a post older than the retention window")
	}
}

func TestScrapeAllPicksUpEveryFeed(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")
	srv := feedServer(t, liveFeed)

	db.Feed(t, user.ID, "One", srv.URL+"/one")
	db.Feed(t, user.ID, "Two", srv.URL+"/two")

	var seen int
	results, err := ScrapeAll(t.Context(), db.Queries, func(Result) { seen++ })
	if err != nil {
		t.Fatalf("ScrapeAll: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ScrapeAll returned %d results, want 2", len(results))
	}
	if seen != 2 {
		t.Errorf("onResult fired %d times, want 2", seen)
	}
}
