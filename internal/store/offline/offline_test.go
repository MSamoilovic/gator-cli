package offline

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"

	"github.com/google/uuid"
)

const server = "https://gator.example.test"

func post(title string, published time.Time, feedID uuid.UUID) store.Post {
	return store.Post{
		ID:          uuid.New(),
		CreatedAt:   published,
		Title:       title,
		Url:         "https://example.test/" + title,
		Description: sql.NullString{String: "body of " + title, Valid: true},
		PublishedAt: sql.NullTime{Time: published, Valid: true},
		FeedID:      feedID,
	}
}

func filled(t *testing.T, dir string, posts ...store.Post) (*storetest.Fake, *Store) {
	t.Helper()

	fake := storetest.New("marko", posts...)
	st := Wrap(fake, server, dir)
	if _, err := st.Me(t.Context()); err != nil {
		t.Fatalf("Me online: %v", err)
	}
	if _, err := st.Posts(t.Context(), store.PostQuery{}); err != nil {
		t.Fatalf("Posts online: %v", err)
	}
	return fake, st
}

func TestReadsFallBackToTheCacheWhenTheServerIsGone(t *testing.T) {
	dir := t.TempDir()
	feed := uuid.New()
	fake, st := filled(t, dir, post("one", time.Now(), feed), post("two", time.Now().Add(-time.Hour), feed))
	fake.Subs = []store.Subscription{{FeedID: feed, FeedName: "Example"}}
	if _, err := st.Subscriptions(t.Context()); err != nil {
		t.Fatal(err)
	}

	fake.Err = store.ErrUnreachable

	posts, err := st.Posts(t.Context(), store.PostQuery{})
	if err != nil {
		t.Fatalf("Posts offline: %v", err)
	}
	if len(posts) != 2 || posts[0].Title != "one" {
		t.Fatalf("Posts offline = %d posts, first %q; want 2, one", len(posts), posts[0].Title)
	}

	subs, err := st.Subscriptions(t.Context())
	if err != nil || len(subs) != 1 || subs[0].FeedName != "Example" {
		t.Fatalf("Subscriptions offline = %v, %v; want Example", subs, err)
	}
}

func TestASecondViewAddsToTheCacheInsteadOfReplacingIt(t *testing.T) {
	sport, tech := uuid.New(), uuid.New()
	now := time.Now()
	fresh := post("fresh", wallClock(now.Add(-time.Hour)), sport)
	other := post("other", wallClock(now.Add(-2*time.Hour)), tech)

	fake, st := filled(t, t.TempDir(), fresh, other)

	if _, err := st.Posts(t.Context(), store.PostQuery{FeedID: tech}); err != nil {
		t.Fatal(err)
	}

	fake.Err = store.ErrUnreachable
	posts, err := st.Posts(t.Context(), store.PostQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		var titles []string
		for _, p := range posts {
			titles = append(titles, p.Title)
		}
		t.Fatalf("cache after browsing one feed = %v, want both posts", titles)
	}
}

func TestMeOfflineAnswersFromTheCacheSoTheReaderStarts(t *testing.T) {
	fake, st := filled(t, t.TempDir())
	fake.Err = store.ErrUnreachable

	user, err := st.Me(t.Context())
	if err != nil {
		t.Fatalf("Me offline: %v", err)
	}
	if user.Name != "marko" {
		t.Errorf("Me offline = %q, want marko", user.Name)
	}
}

func TestAnEmptyCacheOfflineSaysItWasNeverSynced(t *testing.T) {
	fake := storetest.New("marko")
	fake.Err = store.ErrUnreachable
	st := Wrap(fake, server, t.TempDir())

	_, err := st.Me(t.Context())
	if !errors.Is(err, store.ErrNeverSynced) {
		t.Fatalf("Me with an empty cache = %v, want ErrNeverSynced", err)
	}
	if !errors.Is(err, store.ErrUnreachable) {
		t.Errorf("Me with an empty cache = %v, want it to stay ErrUnreachable too", err)
	}
}

func TestACacheWrittenForAnotherServerIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	filled(t, dir, post("one", time.Now(), uuid.New()))

	elsewhere := storetest.New("marko")
	elsewhere.Err = store.ErrUnreachable
	st := Wrap(elsewhere, "https://other.example.test", dir)

	if _, err := st.Me(t.Context()); !errors.Is(err, store.ErrNeverSynced) {
		t.Fatalf("Me against another server = %v, want ErrNeverSynced", err)
	}
	posts, err := st.Posts(t.Context(), store.PostQuery{})
	if err != nil || len(posts) != 0 {
		t.Errorf("Posts against another server = %d posts, %v; want none", len(posts), err)
	}
}

func TestTheCacheFileIsOnlyReadableByItsOwner(t *testing.T) {
	dir := t.TempDir()
	filled(t, dir, post("one", time.Now(), uuid.New()))

	info, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("cache file mode = %v, want -rw-------", perm)
	}
}

func TestTheCacheKeepsTheNewestPostsAndNoMoreThanTheCap(t *testing.T) {
	feed := uuid.New()
	var posts []store.Post
	for i := range maxPosts + 50 {
		published := wallClock(time.Now().Add(-time.Duration(i) * time.Minute))
		posts = append(posts, post(fmt.Sprintf("post-%03d", i), published, feed))
	}

	fake, st := filled(t, t.TempDir(), posts...)
	fake.Err = store.ErrUnreachable

	cached, err := st.Posts(t.Context(), store.PostQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != maxPosts {
		t.Fatalf("cached posts = %d, want %d", len(cached), maxPosts)
	}
	for i := 1; i < len(cached); i++ {
		if postTime(cached[i-1]).Before(postTime(cached[i])) {
			t.Fatalf("cached posts are not newest first at %d", i)
		}
	}
	if cached[0].Title != posts[0].Title {
		t.Errorf("newest cached post = %q, want %q", cached[0].Title, posts[0].Title)
	}
}

func TestOfflinePostsHonourTheQuery(t *testing.T) {
	inAZoneAheadOfUTC(t)

	now := time.Now()
	sport, tech := uuid.New(), uuid.New()
	fresh := post("fresh", wallClock(now.Add(-time.Hour)), sport)
	stale := post("stale", wallClock(now.Add(-72*time.Hour)), sport)
	other := post("other", wallClock(now.Add(-2*time.Hour)), tech)
	edge := post("edge", wallClock(now.Add(-25*time.Hour)), tech)

	dir := t.TempDir()
	fake, st := filled(t, dir, fresh, stale, other, edge)
	fake.Subs = []store.Subscription{{FeedID: sport, FeedName: "Sport"}, {FeedID: tech, FeedName: "Tech"}}
	if _, err := st.Subscriptions(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRead(t.Context(), stale.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadIDs(t.Context()); err != nil {
		t.Fatal(err)
	}
	fake.Err = store.ErrUnreachable

	titles := func(q store.PostQuery) []string {
		t.Helper()
		got, err := st.Posts(t.Context(), q)
		if err != nil {
			t.Fatalf("Posts offline: %v", err)
		}
		var out []string
		for _, p := range got {
			out = append(out, p.Title)
		}
		return out
	}

	cases := []struct {
		name string
		q    store.PostQuery
		want []string
	}{
		{"everything, newest first", store.PostQuery{}, []string{"fresh", "other", "edge", "stale"}},
		{"oldest first", store.PostQuery{SortDir: "asc"}, []string{"stale", "edge", "other", "fresh"}},
		{"one feed", store.PostQuery{FeedID: sport}, []string{"fresh", "stale"}},
		{"feed by name", store.PostQuery{FeedName: "tec"}, []string{"other", "edge"}},
		{"unread only", store.PostQuery{Read: store.ReadUnread}, []string{"fresh", "other", "edge"}},
		{"read only", store.PostQuery{Read: store.ReadOnly}, []string{"stale"}},
		{"last day", store.PostQuery{Since: now.Add(-24 * time.Hour)}, []string{"fresh", "other"}},
		{"body search", store.PostQuery{Query: "body of fre"}, []string{"fresh"}},
		{"second page", store.PostQuery{Limit: 2, Offset: 2}, []string{"edge", "stale"}},
		{"past the end", store.PostQuery{Limit: 2, Offset: 9}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := titles(c.q)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestTheStateFollowsWhetherTheLastCallReachedTheServer(t *testing.T) {
	fake, st := filled(t, t.TempDir(), post("one", time.Now(), uuid.New()))

	if state := st.State(); !state.Online || state.LastSync.IsZero() {
		t.Fatalf("State after a good call = %+v, want online with a sync time", state)
	}

	fake.Err = store.ErrUnreachable
	if _, err := st.Posts(t.Context(), store.PostQuery{}); err != nil {
		t.Fatal(err)
	}
	if state := st.State(); state.Online {
		t.Errorf("State after an unreachable call = %+v, want offline", state)
	}

	fake.Err = nil
	if _, err := st.Posts(t.Context(), store.PostQuery{}); err != nil {
		t.Fatal(err)
	}
	if state := st.State(); !state.Online {
		t.Errorf("State after the server came back = %+v, want online", state)
	}
}

func TestACancelledCallIsNotTakenForAnOfflineServer(t *testing.T) {
	fake, st := filled(t, t.TempDir(), post("one", time.Now(), uuid.New()))
	fake.Err = store.ErrUnreachable

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := st.Posts(ctx, store.PostQuery{}); !errors.Is(err, store.ErrUnreachable) {
		t.Fatalf("Posts with a cancelled context = %v, want the error through", err)
	}
	if state := st.State(); !state.Online {
		t.Errorf("State after a cancelled call = %+v, want it unchanged", state)
	}
}

func inAZoneAheadOfUTC(t *testing.T) {
	t.Helper()
	was := time.Local
	time.Local = time.FixedZone("TEST", 2*60*60)
	t.Cleanup(func() { time.Local = was })
}
