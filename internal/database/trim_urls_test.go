package database_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

func TestTheURLMigrationTrimsLinksAndDropsADuplicateItWouldCreate(t *testing.T) {
	db := testdb.Open(t)
	body, err := os.ReadFile("../../sql/schema/020_trim_post_urls.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, up, _ := strings.Cut(string(body), "-- +goose Up")
	up, _, _ = strings.Cut(up, "-- +goose Down")

	alice := db.User(t, "alice")
	feed := db.Feed(t, alice.ID, "CBS", "https://cbs.test/rss")
	db.Post(t, feed.ID, "padded", "\n        https://cbs.test/a\n\t", time.Now())
	db.Post(t, feed.ID, "clean twin", "https://cbs.test/b", time.Now())
	db.Post(t, feed.ID, "padded twin", "\n  https://cbs.test/b", time.Now())

	db.Exec(t, up)

	if n := db.Count(t, "SELECT count(*) FROM posts WHERE url = 'https://cbs.test/a'"); n != 1 {
		t.Errorf("the padded link was not trimmed: %d rows at https://cbs.test/a", n)
	}
	if n := db.Count(t, "SELECT count(*) FROM posts WHERE url = 'https://cbs.test/b'"); n != 1 {
		t.Errorf("https://cbs.test/b has %d rows, want the one clean row", n)
	}
	if n := db.Count(t, "SELECT count(*) FROM posts WHERE url ~ '^\\s|\\s$'"); n != 0 {
		t.Errorf("%d links still carry whitespace", n)
	}
}
