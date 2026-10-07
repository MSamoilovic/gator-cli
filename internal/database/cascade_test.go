package database_test

import (
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

func TestDeletingAUserKeepsTheFeedsTheyAdded(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")
	feed := db.Feed(t, alice.ID, "Hacker News", "https://hn.test/rss")
	db.Post(t, feed.ID, "Post A", "https://hn.test/a", time.Now())

	db.Exec(t, "DELETE FROM users WHERE id = $1", alice.ID)

	if n := db.Count(t, "SELECT count(*) FROM feeds WHERE id = $1", feed.ID); n != 1 {
		t.Errorf("feeds left after deleting its adder = %d, want 1", n)
	}
	if n := db.Count(t, "SELECT count(*) FROM feeds WHERE id = $1 AND user_id IS NULL", feed.ID); n != 1 {
		t.Errorf("the surviving feed does not have user_id NULL")
	}
	if n := db.Count(t, "SELECT count(*) FROM posts WHERE feed_id = $1", feed.ID); n != 1 {
		t.Errorf("posts left = %d, want 1", n)
	}
}

func TestDeletingAUserLeavesEveryoneElseAlone(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")
	bob := db.User(t, "bob")

	feed := db.Feed(t, alice.ID, "Hacker News", "https://hn.test/rss")
	post := db.Post(t, feed.ID, "Post A", "https://hn.test/a", time.Now())

	db.Follow(t, alice.ID, feed.ID)
	db.Follow(t, bob.ID, feed.ID)
	db.Bookmark(t, bob.ID, post.ID)
	db.MarkRead(t, bob.ID, post.ID)

	db.Exec(t, "DELETE FROM users WHERE id = $1", alice.ID)

	for _, c := range []struct {
		what  string
		query string
	}{
		{"follows", "SELECT count(*) FROM feed_follows WHERE user_id = $1"},
		{"bookmarks", "SELECT count(*) FROM bookmarks WHERE user_id = $1"},
		{"reads", "SELECT count(*) FROM post_reads WHERE user_id = $1"},
	} {
		if n := db.Count(t, c.query, bob.ID); n != 1 {
			t.Errorf("bob's %s after alice was deleted = %d, want 1", c.what, n)
		}
	}
}

func TestDeletingAUserTakesTheirOwnRowsWithIt(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")
	feed := db.Feed(t, alice.ID, "Hacker News", "https://hn.test/rss")
	post := db.Post(t, feed.ID, "Post A", "https://hn.test/a", time.Now())

	db.Follow(t, alice.ID, feed.ID)
	db.Bookmark(t, alice.ID, post.ID)
	db.MarkRead(t, alice.ID, post.ID)

	db.Exec(t, "DELETE FROM users WHERE id = $1", alice.ID)

	for _, c := range []struct {
		what  string
		query string
	}{
		{"follows", "SELECT count(*) FROM feed_follows WHERE user_id = $1"},
		{"bookmarks", "SELECT count(*) FROM bookmarks WHERE user_id = $1"},
		{"reads", "SELECT count(*) FROM post_reads WHERE user_id = $1"},
	} {
		if n := db.Count(t, c.query, alice.ID); n != 0 {
			t.Errorf("alice's %s after she was deleted = %d, want 0", c.what, n)
		}
	}
}

func TestFeedOwnerForeignKeyIsNotCascading(t *testing.T) {
	db := testdb.Open(t)

	var action string
	db.QueryRow(t, &action, `
		SELECT confdeltype FROM pg_constraint
		WHERE conrelid = 'feeds'::regclass AND contype = 'f'
		  AND conkey = ARRAY[(
		    SELECT attnum FROM pg_attribute
		    WHERE attrelid = 'feeds'::regclass AND attname = 'user_id'
		  )]::smallint[]`)

	if action != "n" {
		t.Errorf("feeds.user_id ON DELETE is %q, want %q (SET NULL) — a cascade here wipes other users' data", action, "n")
	}
}

func TestDeletingAFeedTakesItsPosts(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")
	feed := db.Feed(t, alice.ID, "Hacker News", "https://hn.test/rss")
	db.Post(t, feed.ID, "Post A", "https://hn.test/a", time.Now())

	db.Exec(t, "DELETE FROM feeds WHERE id = $1", feed.ID)

	if n := db.Count(t, "SELECT count(*) FROM posts WHERE feed_id = $1", feed.ID); n != 0 {
		t.Errorf("posts left after their feed was deleted = %d, want 0", n)
	}
}
