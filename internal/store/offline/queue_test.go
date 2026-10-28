package offline

import (
	"context"
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

const feedURL = "https://example.test/feed.xml"

func subscribed(t *testing.T, dir string, posts ...store.Post) (*storetest.Fake, *Store) {
	t.Helper()

	fake, st := filled(t, dir, posts...)
	feed := uuid.New()
	if len(posts) > 0 {
		feed = posts[0].FeedID
	}
	fake.Subs = []store.Subscription{{FeedID: feed, FeedName: "Example", FeedUrl: feedURL}}
	fake.Unread = []store.UnreadCount{{FeedID: feed, Unread: int64(len(posts))}}
	if _, err := st.Subscriptions(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UnreadCounts(t.Context()); err != nil {
		t.Fatal(err)
	}
	return fake, st
}

func queued(t *testing.T, dir string) []change {
	t.Helper()

	path := filepath.Join(dir, QueueFileName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return loadQueue(path).pending
}

func TestWritesMadeOfflineReachTheServerOnTheNextGoodCall(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	fake.Err = store.ErrUnreachable

	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatalf("SetRead offline: %v", err)
	}
	created, err := st.Bookmark(t.Context(), one.ID)
	if err != nil {
		t.Fatalf("Bookmark offline: %v", err)
	}
	if !created {
		t.Error("Bookmark offline reported nothing new")
	}
	if _, err := st.Categorize(t.Context(), feedURL, "News"); err != nil {
		t.Fatalf("Categorize offline: %v", err)
	}

	if n := st.State().Pending; n != 3 {
		t.Fatalf("pending = %d, want 3", n)
	}
	if n := len(queued(t, dir)); n != 3 {
		t.Fatalf("queue file holds %d changes, want 3", n)
	}

	read, _ := st.ReadIDs(t.Context())
	saved, _ := st.BookmarkedIDs(t.Context())
	subs, _ := st.Subscriptions(t.Context())
	switch {
	case len(read) != 1 || read[0] != one.ID:
		t.Errorf("cached read ids = %v, want %v", read, one.ID)
	case len(saved) != 1 || saved[0] != one.ID:
		t.Errorf("cached bookmark ids = %v, want %v", saved, one.ID)
	case len(subs) != 1 || subs[0].Category != "News":
		t.Errorf("cached folder = %q, want News", subs[0].Category)
	}

	fake.Err = nil
	if _, err := st.Posts(t.Context(), store.PostQuery{}); err != nil {
		t.Fatal(err)
	}

	if n := st.State().Pending; n != 0 {
		t.Errorf("pending after coming back = %d, want 0", n)
	}
	if _, err := os.Stat(filepath.Join(dir, QueueFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the queue file survived an empty queue: %v", err)
	}
	if !fake.Read[one.ID] {
		t.Error("the server never got the read")
	}
	if !fake.Bookmarked[one.ID] {
		t.Error("the server never got the bookmark")
	}
	if fake.Subs[0].Category != "News" {
		t.Errorf("the server's folder = %q, want News", fake.Subs[0].Category)
	}
}

func TestAChangeTheServerRefusesIsDroppedAndTheRestStillGoes(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	fake.Err = store.ErrUnreachable
	if _, err := st.Categorize(t.Context(), feedURL, "News"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatal(err)
	}

	fake.Err = nil
	fake.Subs = nil

	if _, err := st.Posts(t.Context(), store.PostQuery{}); err != nil {
		t.Fatal(err)
	}

	if n := st.State().Pending; n != 0 {
		t.Errorf("pending = %d, want 0 — a refused change must not wedge the queue", n)
	}
	if !fake.Read[one.ID] {
		t.Error("the change behind the refused one never went")
	}
}

type failingWrites struct {
	store.Store
	allow int
	sent  int
}

func (f *failingWrites) SetRead(ctx context.Context, postID uuid.UUID, read bool) error {
	f.sent++
	if f.sent > f.allow {
		return fmt.Errorf("the connection dropped: %w", store.ErrUnreachable)
	}
	return f.Store.SetRead(ctx, postID, read)
}

func TestTheFirstNetworkErrorStopsThePassAndKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	fake := storetest.New("marko")
	inner := &failingWrites{Store: fake}
	st := Wrap(inner, server, dir)

	if _, err := st.Me(t.Context()); err != nil {
		t.Fatal(err)
	}

	fake.Err = store.ErrUnreachable
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		if err := st.SetRead(t.Context(), id, true); err != nil {
			t.Fatal(err)
		}
	}

	fake.Err = nil
	inner.sent, inner.allow = 0, 1

	if _, err := st.Me(t.Context()); err != nil {
		t.Fatal(err)
	}

	state := st.State()
	if state.Pending != 2 {
		t.Errorf("pending = %d, want 2 — the pass must stop, not drop", state.Pending)
	}
	if state.Online {
		t.Error("state says online after the pass hit an unreachable server")
	}
	if n := len(queued(t, dir)); n != 2 {
		t.Errorf("queue file holds %d changes, want 2", n)
	}
	if !fake.Read[ids[0]] || fake.Read[ids[1]] {
		t.Errorf("the server got %v, want only the first change", fake.Read)
	}
}

func TestTheSameChangeReplayedTwiceLeavesOneState(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	twice := change{Op: opRead, PostID: one.ID, Value: true, At: time.Now()}
	q := loadQueue(filepath.Join(dir, QueueFileName))
	for range 2 {
		if _, err := q.add(twice); err != nil {
			t.Fatal(err)
		}
	}

	again := Wrap(fake, server, dir)
	if n := again.State().Pending; n != 2 {
		t.Fatalf("pending = %d, want the 2 changes a crash left behind", n)
	}
	if _, err := again.Me(t.Context()); err != nil {
		t.Fatal(err)
	}

	if n := again.State().Pending; n != 0 {
		t.Errorf("pending = %d, want 0", n)
	}
	read, err := st.ReadIDs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 1 {
		t.Errorf("read ids = %v, want exactly one", read)
	}
}

func TestTheQueueDropsTheOldestWhenItIsFullAndSaysSo(t *testing.T) {
	dir := t.TempDir()
	fake, st := subscribed(t, dir)
	fake.Err = store.ErrUnreachable

	first := uuid.New()
	if err := st.SetRead(t.Context(), first, true); err != nil {
		t.Fatal(err)
	}
	for range maxQueued {
		if err := st.SetRead(t.Context(), uuid.New(), true); err != nil {
			t.Fatal(err)
		}
	}

	state := st.State()
	if state.Pending != maxQueued {
		t.Errorf("pending = %d, want the cap %d", state.Pending, maxQueued)
	}
	if state.Dropped != 1 {
		t.Errorf("dropped = %d, want 1 — a silent loss is worse than an admitted one", state.Dropped)
	}
	for _, c := range queued(t, dir) {
		if c.PostID == first {
			t.Fatal("the oldest change survived the cap")
		}
	}
}

func TestATornQueueLineIsSkippedAndTheRestSurvives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, QueueFileName)
	good := `{"op":"read","post_id":"` + uuid.New().String() + `","value":true,"at":"2026-10-27T10:00:00Z"}`
	if err := os.WriteFile(path, []byte(good+"\n{\"op\":\"rea"), 0600); err != nil {
		t.Fatal(err)
	}

	q := loadQueue(path)
	if len(q.pending) != 1 {
		t.Fatalf("loaded %d changes, want 1 — a torn last line is what append-only buys", len(q.pending))
	}
	if q.pending[0].Op != opRead {
		t.Errorf("loaded op = %q, want %q", q.pending[0].Op, opRead)
	}
}

func TestTheQueueFileIsOnlyReadableByItsOwner(t *testing.T) {
	dir := t.TempDir()
	fake, st := subscribed(t, dir)
	fake.Err = store.ErrUnreachable

	if err := st.SetRead(t.Context(), uuid.New(), true); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, QueueFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("queue file mode = %v, want -rw-------", perm)
	}
}

func TestMarkingReadOfflineKeepsTheUnreadCountersHonest(t *testing.T) {
	dir := t.TempDir()
	feed := uuid.New()
	one := post("one", wallClock(time.Now()), feed)
	two := post("two", wallClock(time.Now().Add(-time.Hour)), feed)
	fake, st := subscribed(t, dir, one, two)

	fake.Err = store.ErrUnreachable

	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatal(err)
	}
	counts, err := st.UnreadCounts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].Unread != 1 {
		t.Fatalf("unread after reading one of two = %v, want 1", counts)
	}

	if err := st.SetRead(t.Context(), one.ID, false); err != nil {
		t.Fatal(err)
	}
	counts, _ = st.UnreadCounts(t.Context())
	if counts[0].Unread != 2 {
		t.Errorf("unread after marking it back = %d, want 2", counts[0].Unread)
	}

	if err := st.SetAllRead(t.Context(), []uuid.UUID{one.ID, two.ID}); err != nil {
		t.Fatal(err)
	}
	counts, _ = st.UnreadCounts(t.Context())
	if counts[0].Unread != 0 {
		t.Errorf("unread after marking the page read = %d, want 0", counts[0].Unread)
	}
}

func TestCategorizingAFeedTheCacheNeverSawIsRefusedNotQueued(t *testing.T) {
	dir := t.TempDir()
	fake, st := subscribed(t, dir)
	fake.Err = store.ErrUnreachable

	_, err := st.Categorize(t.Context(), "https://stranger.test/feed.xml", "News")
	if !errors.Is(err, store.ErrNotFollowed) {
		t.Fatalf("Categorize of an unknown feed = %v, want ErrNotFollowed", err)
	}
	if n := st.State().Pending; n != 0 {
		t.Errorf("pending = %d, want nothing queued", n)
	}
}

func TestBookmarkingOfflineTwiceOnlySaysNewOnce(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)
	fake.Err = store.ErrUnreachable

	first, err := st.Bookmark(t.Context(), one.ID)
	if err != nil || !first {
		t.Fatalf("first Bookmark offline = %v, %v; want true", first, err)
	}
	second, err := st.Bookmark(t.Context(), one.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second {
		t.Error("the second Bookmark offline claimed a new save")
	}

	posts, err := st.BookmarkedPosts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Errorf("bookmarked posts = %d, want 1", len(posts))
	}

	if err := st.Unbookmark(t.Context(), one.ID); err != nil {
		t.Fatal(err)
	}
	posts, _ = st.BookmarkedPosts(t.Context())
	if len(posts) != 0 {
		t.Errorf("bookmarked posts after unbookmarking = %d, want 0", len(posts))
	}
}

func TestAnOfflineUnbookmarkReachesTheServerAsARemoval(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	if _, err := st.Bookmark(t.Context(), one.ID); err != nil {
		t.Fatal(err)
	}
	if !fake.Bookmarked[one.ID] {
		t.Fatal("the online bookmark never reached the server")
	}

	fake.Err = store.ErrUnreachable
	if err := st.Unbookmark(t.Context(), one.ID); err != nil {
		t.Fatalf("Unbookmark offline: %v", err)
	}

	fake.Err = nil
	if _, err := st.Me(t.Context()); err != nil {
		t.Fatal(err)
	}

	if st.State().Pending != 0 {
		t.Fatalf("pending = %d, want 0", st.State().Pending)
	}
	if fake.Bookmarked[one.ID] {
		t.Error("the replay saved the post again instead of removing it")
	}
}

func TestWhatNeedsTheNetworkIsRefusedAndNotQueued(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)
	fake.Err = store.ErrUnreachable

	if _, err := st.FullText(t.Context(), one); !errors.Is(err, store.ErrUnreachable) {
		t.Errorf("FullText offline = %v, want ErrUnreachable", err)
	}
	if _, _, err := st.AddFeed(t.Context(), "Example", feedURL); !errors.Is(err, store.ErrUnreachable) {
		t.Errorf("AddFeed offline = %v, want ErrUnreachable", err)
	}
	if _, err := st.Stats(t.Context(), time.Now()); !errors.Is(err, store.ErrUnreachable) {
		t.Errorf("Stats offline = %v, want ErrUnreachable", err)
	}
	if n := st.State().Pending; n != 0 {
		t.Errorf("pending = %d, want nothing queued for what needs the network", n)
	}
}
