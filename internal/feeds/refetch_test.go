package feeds

import (
	"fmt"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func remembering(t *testing.T, db *testdb.DB, user uuid.UUID, name string, posts int, follow bool) uuid.UUID {
	t.Helper()

	feed := db.Feed(t, user, name, "https://"+name+".test/rss")
	db.Exec(t, `UPDATE feeds SET etag = 'W/"tag"', last_modified = 'Mon, 01 Jan 2026 00:00:00 GMT' WHERE id = $1`, feed.ID)
	if follow {
		db.Follow(t, user, feed.ID)
	}
	for i := range posts {
		db.Post(t, feed.ID, fmt.Sprintf("%s p%d", name, i), fmt.Sprintf("https://%s.test/%d", name, i),
			time.Now().Add(-time.Duration(500+i)*time.Hour))
	}
	return feed.ID
}

func remembers(t *testing.T, db *testdb.DB, id uuid.UUID) bool {
	t.Helper()
	return db.Count(t, "SELECT count(*) FROM feeds WHERE id = $1 AND (etag <> '' OR last_modified <> '')", id) == 1
}

func TestForgetClearsOnlyTheThinFollowedFeeds(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	thin := remembering(t, db, user.ID, "thin", 3, true)
	full := remembering(t, db, user.ID, "full", 20, true)
	exactly := remembering(t, db, user.ID, "exactly", int(DefaultKeepPerFeed), true)
	unfollowed := remembering(t, db, user.ID, "unfollowed", 1, false)

	n, err := Forget(t.Context(), db.Queries, DefaultKeepPerFeed, false)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if n != 1 {
		t.Errorf("Forget cleared %d feeds, want 1", n)
	}

	if remembers(t, db, thin) {
		t.Error("the thin feed still remembers its validators, so the next fetch is still a 304")
	}
	if !remembers(t, db, full) {
		t.Error("a feed with plenty of posts lost its validators, which costs a full body every round")
	}
	if !remembers(t, db, exactly) {
		t.Errorf("a feed holding exactly %d posts was treated as thin; the threshold is strict", DefaultKeepPerFeed)
	}
	if !remembers(t, db, unfollowed) {
		t.Error("an unfollowed feed was cleared; nothing fetches it, so clearing it buys nothing")
	}
}

func TestForgetAllIgnoresThePostCount(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	thin := remembering(t, db, user.ID, "thin", 2, true)
	full := remembering(t, db, user.ID, "full", 40, true)
	unfollowed := remembering(t, db, user.ID, "unfollowed", 40, false)

	n, err := Forget(t.Context(), db.Queries, 0, true)
	if err != nil {
		t.Fatalf("Forget(below=0, all=true) was refused, but the threshold rule does not apply to all: %v", err)
	}
	if n != 2 {
		t.Errorf("Forget(all) cleared %d feeds, want 2", n)
	}
	if remembers(t, db, thin) || remembers(t, db, full) {
		t.Error("Forget(all) left a followed feed remembering its validators")
	}
	if !remembers(t, db, unfollowed) {
		t.Error("Forget(all) cleared an unfollowed feed too")
	}
}

func TestForgetSkipsFeedsWithNothingToClear(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	feed := db.Feed(t, user.ID, "clean", "https://clean.test/rss")
	db.Follow(t, user.ID, feed.ID)

	n, err := Forget(t.Context(), db.Queries, DefaultKeepPerFeed, true)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if n != 0 {
		t.Errorf("Forget reported %d cleared feeds, want 0 — a feed with empty validators needs no write, and the count is what the CLI prints", n)
	}
}

func TestForgetRefusesANonPositiveThresholdUnlessAllIsMeant(t *testing.T) {
	for _, below := range []int32{0, -1} {
		if _, err := Forget(t.Context(), nil, below, false); err == nil {
			t.Errorf("Forget(below=%d) returned no error, want one before it touches the database", below)
		}
	}
}

func TestPruneCannotLeaveAFollowedFeedBelowTheKeepWindow(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	held := []int{0, 1, 5, 9, 10, 11, 40}
	for _, n := range held {
		feed := db.Feed(t, user.ID, fmt.Sprintf("feed%d", n), fmt.Sprintf("https://f%d.test/rss", n))
		db.Follow(t, user.ID, feed.ID)
		for i := range n {
			db.Post(t, feed.ID, fmt.Sprintf("f%d p%02d", n, i), fmt.Sprintf("https://f%d.test/%d", n, i),
				time.Now().Add(-time.Duration(500+i)*time.Hour))
		}
	}

	if _, err := Prune(t.Context(), db.Queries, DefaultRetention, DefaultKeepPerFeed); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	for _, n := range held {
		got := db.Count(t, "SELECT count(*) FROM posts p JOIN feeds f ON f.id = p.feed_id WHERE f.name = $1", fmt.Sprintf("feed%d", n))
		if want := min(n, int(DefaultKeepPerFeed)); got != want {
			t.Errorf("a feed holding %d posts kept %d, want %d", n, got, want)
		}
		if n < int(DefaultKeepPerFeed) && got != n {
			t.Errorf("prune took %d of %d posts from a feed already below the keep window — that is how a feed gets "+
				"trapped behind its own conditional GET, and it must stay impossible", n-got, n)
		}
	}
}

func TestAgeOnlyPruneIsTheOneWayLeftToTrapAFeed(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")
	feed := remembering(t, db, user.ID, "dave", 12, true)

	if _, err := Prune(t.Context(), db.Queries, DefaultRetention, 0); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if got := db.Count(t, "SELECT count(*) FROM posts"); got != 0 {
		t.Fatalf("%d posts left, want 0 — keep-per-feed 0 is age-only retention", got)
	}
	if !remembers(t, db, feed) {
		t.Fatal("age-only prune cleared the validators by itself; this test exists to show that it does not")
	}

	n, err := Forget(t.Context(), db.Queries, DefaultKeepPerFeed, false)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if n != 1 || remembers(t, db, feed) {
		t.Errorf("Forget cleared %d feeds and the feed still remembers: %v — refetch is the repair for this state", n, remembers(t, db, feed))
	}
}
