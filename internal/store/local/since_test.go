package local_test

import (
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/local"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

func TestTheSinceBoundIsAnInstantWhateverZoneItArrivesIn(t *testing.T) {
	db := testdb.Open(t)
	alice := db.User(t, "alice")
	feed := db.Feed(t, alice.ID, "Feed", "https://feed.test/rss")
	db.Follow(t, alice.ID, feed.ID)
	db.Post(t, feed.ID, "fresh", "https://feed.test/fresh", time.Now().Add(-30*time.Minute).UTC())

	east := time.FixedZone("CEST", 2*3600)
	since := time.Now().Add(-time.Hour).In(east)

	posts, err := local.New(db.Queries, "alice").Posts(t.Context(), store.PostQuery{Since: since, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Errorf("a post 30 minutes old was left out of the last hour when the bound came in at +02:00: got %d posts", len(posts))
	}

	stats, err := local.New(db.Queries, "alice").Stats(t.Context(), since)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].RecentCount != 1 {
		t.Errorf("stats counted %+v in the last hour, want the one fresh post", stats)
	}
}
