package database_test

import (
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

var noFeed = uuid.MustParse("00000000-0000-0000-0000-000000000000")

func allPosts(userID uuid.UUID) database.GetPostsForUserFilteredParams {
	return database.GetPostsForUserFilteredParams{
		UserID:    userID,
		FeedID:    noFeed,
		SortDir:   "desc",
		PostLimit: 100,
	}
}

func titles(posts []database.Post) []string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = p.Title
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type fixture struct {
	db           *testdb.DB
	alice, bob   database.User
	goFeed       database.Feed
	sportFeed    database.Feed
	oldGo, newGo database.Post
	sportPost    database.Post
}

func seed(t *testing.T) fixture {
	t.Helper()
	db := testdb.Open(t)

	f := fixture{db: db}
	f.alice = db.User(t, "alice")
	f.bob = db.User(t, "bob")

	f.goFeed = db.Feed(t, f.alice.ID, "Go Blog", "https://go.test/rss")
	f.sportFeed = db.Feed(t, f.alice.ID, "Sport Desk", "https://sport.test/rss")

	f.oldGo = db.Post(t, f.goFeed.ID, "Older Go post", "https://go.test/1", time.Now().Add(-72*time.Hour))
	f.newGo = db.Post(t, f.goFeed.ID, "Newer Go post", "https://go.test/2", time.Now().Add(-1*time.Hour))
	f.sportPost = db.Post(t, f.sportFeed.ID, "Match report", "https://sport.test/1", time.Now().Add(-2*time.Hour))

	db.Follow(t, f.alice.ID, f.goFeed.ID)
	db.Follow(t, f.alice.ID, f.sportFeed.ID)

	return f
}

func TestPostsOnlyComeFromFeedsTheUserFollows(t *testing.T) {
	f := seed(t)

	posts, err := f.db.GetPostsForUserFiltered(t.Context(), allPosts(f.bob.ID))
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("bob follows nothing but got %d posts: %v", len(posts), titles(posts))
	}

	posts, err = f.db.GetPostsForUserFiltered(t.Context(), allPosts(f.alice.ID))
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	if len(posts) != 3 {
		t.Errorf("alice follows both feeds but got %d posts: %v", len(posts), titles(posts))
	}
}

func TestPostsSortNewestFirstByDefaultAndOldestFirstOnAsc(t *testing.T) {
	f := seed(t)

	desc, err := f.db.GetPostsForUserFiltered(t.Context(), allPosts(f.alice.ID))
	if err != nil {
		t.Fatalf("desc: %v", err)
	}
	want := []string{"Newer Go post", "Match report", "Older Go post"}
	if got := titles(desc); !equal(got, want) {
		t.Errorf("desc order = %v, want %v", got, want)
	}

	p := allPosts(f.alice.ID)
	p.SortDir = "asc"
	asc, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("asc: %v", err)
	}
	want = []string{"Older Go post", "Match report", "Newer Go post"}
	if got := titles(asc); !equal(got, want) {
		t.Errorf("asc order = %v, want %v", got, want)
	}
}

func TestPostsFilterByFeedName(t *testing.T) {
	f := seed(t)

	p := allPosts(f.alice.ID)
	p.FeedName = "go blog"

	posts, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts for a case-insensitive partial feed name, want 2: %v", len(posts), titles(posts))
	}
	for _, post := range posts {
		if post.FeedID != f.goFeed.ID {
			t.Errorf("post %q came from another feed", post.Title)
		}
	}
}

func TestPostsFilterByFeedID(t *testing.T) {
	f := seed(t)

	p := allPosts(f.alice.ID)
	p.FeedID = f.sportFeed.ID

	posts, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	if got := titles(posts); !equal(got, []string{"Match report"}) {
		t.Errorf("feed_id filter returned %v, want [Match report]", got)
	}
}

func TestPostsUnreadOnlyHidesWhatWasRead(t *testing.T) {
	f := seed(t)
	f.db.MarkRead(t, f.alice.ID, f.newGo.ID)

	p := allPosts(f.alice.ID)
	p.UnreadOnly = true

	posts, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	for _, post := range posts {
		if post.ID == f.newGo.ID {
			t.Error("a post marked read is still listed under unread_only")
		}
	}
	if len(posts) != 2 {
		t.Errorf("got %d unread posts, want 2: %v", len(posts), titles(posts))
	}
}

func TestReadingAPostDoesNotHideItFromAnotherUser(t *testing.T) {
	f := seed(t)
	f.db.Follow(t, f.bob.ID, f.goFeed.ID)
	f.db.MarkRead(t, f.alice.ID, f.newGo.ID)

	p := allPosts(f.bob.ID)
	p.UnreadOnly = true

	posts, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("bob sees %d unread posts, want 2 — alice's read marker leaked: %v", len(posts), titles(posts))
	}
}

func TestPostsSinceDropsOlderPosts(t *testing.T) {
	f := seed(t)

	p := allPosts(f.alice.ID)
	p.Since = time.Now().Add(-24 * time.Hour)

	posts, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("GetPostsForUserFiltered: %v", err)
	}
	for _, post := range posts {
		if post.ID == f.oldGo.ID {
			t.Error("a post from 72h ago survived a 24h window")
		}
	}
	if len(posts) != 2 {
		t.Errorf("got %d posts in the 24h window, want 2: %v", len(posts), titles(posts))
	}
}

func TestPostsLimitAndOffsetPage(t *testing.T) {
	f := seed(t)

	p := allPosts(f.alice.ID)
	p.PostLimit = 2
	first, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if got := titles(first); !equal(got, []string{"Newer Go post", "Match report"}) {
		t.Errorf("page 1 = %v", got)
	}

	p.PostOffset = 2
	second, err := f.db.GetPostsForUserFiltered(t.Context(), p)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if got := titles(second); !equal(got, []string{"Older Go post"}) {
		t.Errorf("page 2 = %v", got)
	}
}

func TestSearchMatchesTitleAndBodyForFollowedFeedsOnly(t *testing.T) {
	f := seed(t)

	found, err := f.db.SearchPostsForUser(t.Context(), database.SearchPostsForUserParams{
		UserID:    f.alice.ID,
		Query:     "match report",
		PostLimit: 50,
	})
	if err != nil {
		t.Fatalf("SearchPostsForUser: %v", err)
	}
	if got := titles(found); !equal(got, []string{"Match report"}) {
		t.Errorf("search returned %v, want [Match report]", got)
	}

	none, err := f.db.SearchPostsForUser(t.Context(), database.SearchPostsForUserParams{
		UserID:    f.bob.ID,
		Query:     "match report",
		PostLimit: 50,
	})
	if err != nil {
		t.Fatalf("SearchPostsForUser: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("bob follows nothing but search returned %v", titles(none))
	}
}

func TestUnreadCountsArePerUser(t *testing.T) {
	f := seed(t)
	f.db.Follow(t, f.bob.ID, f.goFeed.ID)
	f.db.MarkRead(t, f.alice.ID, f.newGo.ID)

	counts := func(userID uuid.UUID) map[string]int64 {
		rows, err := f.db.GetUnreadCountsForUser(t.Context(), userID)
		if err != nil {
			t.Fatalf("GetUnreadCountsForUser: %v", err)
		}
		out := make(map[string]int64, len(rows))
		for _, r := range rows {
			out[r.FeedID.String()] = r.Unread
		}
		return out
	}

	if got := counts(f.alice.ID)[f.goFeed.ID.String()]; got != 1 {
		t.Errorf("alice has %d unread in the Go feed, want 1", got)
	}
	if got := counts(f.bob.ID)[f.goFeed.ID.String()]; got != 2 {
		t.Errorf("bob has %d unread in the Go feed, want 2 — alice's read marker leaked", got)
	}
}

func TestBookmarkAndReadAreIdempotent(t *testing.T) {
	f := seed(t)

	for i := 0; i < 2; i++ {
		f.db.Bookmark(t, f.alice.ID, f.newGo.ID)
		f.db.MarkRead(t, f.alice.ID, f.newGo.ID)
	}

	if n := f.db.Count(t, "SELECT count(*) FROM bookmarks WHERE user_id = $1", f.alice.ID); n != 1 {
		t.Errorf("bookmarking twice produced %d rows, want 1", n)
	}
	if n := f.db.Count(t, "SELECT count(*) FROM post_reads WHERE user_id = $1", f.alice.ID); n != 1 {
		t.Errorf("marking read twice produced %d rows, want 1", n)
	}
}

func TestTruncateAllEmptiesEveryTable(t *testing.T) {
	f := seed(t)
	f.db.Bookmark(t, f.alice.ID, f.newGo.ID)
	f.db.MarkRead(t, f.alice.ID, f.newGo.ID)

	before, err := f.db.CountAllRows(t.Context())
	if err != nil {
		t.Fatalf("CountAllRows: %v", err)
	}
	if before.Users == 0 || before.Feeds == 0 || before.Posts == 0 {
		t.Fatalf("fixture did not populate the database: %+v", before)
	}

	if err := f.db.TruncateAll(t.Context()); err != nil {
		t.Fatalf("TruncateAll: %v", err)
	}

	after, err := f.db.CountAllRows(t.Context())
	if err != nil {
		t.Fatalf("CountAllRows: %v", err)
	}
	if after != (database.CountAllRowsRow{}) {
		t.Errorf("TruncateAll left rows behind: %+v", after)
	}
}
