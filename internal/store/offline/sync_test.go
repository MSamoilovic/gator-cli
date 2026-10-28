package offline

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"

	"github.com/google/uuid"
)

func TestSyncSendsWhatWasQueuedAndReportsHowMuch(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	fake.Err = store.ErrUnreachable
	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Bookmark(t.Context(), one.ID); err != nil {
		t.Fatal(err)
	}

	fake.Err = nil
	res, err := st.Sync(t.Context())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if res.Sent != 2 {
		t.Errorf("sent = %d, want the 2 changes made offline", res.Sent)
	}
	if res.Pending != 0 {
		t.Errorf("pending = %d, want nothing left over", res.Pending)
	}
	if res.Feeds != 1 || res.Posts != 1 {
		t.Errorf("cached %d posts from %d feeds, want 1 and 1", res.Posts, res.Feeds)
	}
	if queued(t, dir) != nil {
		t.Error("the queue file outlived the changes it held")
	}

	read, err := fake.ReadIDs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	saved, err := fake.BookmarkedIDs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != 1 || len(saved) != 1 {
		t.Errorf("server has %d read and %d saved, want one of each", len(read), len(saved))
	}
}

func TestSyncFillsTheCacheThatBrowsingAloneWouldNotHave(t *testing.T) {
	dir := t.TempDir()
	feed := uuid.New()
	var posts []store.Post
	for i := range 5 {
		posts = append(posts, post(string(rune('a'+i)), wallClock(time.Now().Add(-time.Duration(i)*time.Hour)), feed))
	}

	fake := storetest.New("marko", posts...)
	fake.Subs = []store.Subscription{{FeedID: feed, FeedName: "Example", FeedUrl: feedURL}}
	st := Wrap(fake, server, dir)

	if _, err := st.Posts(t.Context(), store.PostQuery{Limit: 2}); err != nil {
		t.Fatal(err)
	}
	res, err := st.Sync(t.Context())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Posts != len(posts) {
		t.Errorf("cached %d posts, want all %d — a cache filled by use alone holds only what you opened",
			res.Posts, len(posts))
	}

	fake.Err = store.ErrUnreachable
	offlinePosts, err := st.Posts(t.Context(), store.PostQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(offlinePosts) != len(posts) {
		t.Errorf("offline list has %d posts, want the %d the sync fetched", len(offlinePosts), len(posts))
	}
	if _, err := st.Subscriptions(t.Context()); err != nil {
		t.Errorf("subscriptions offline: %v — sync did not cache them", err)
	}
}

func TestSyncOfflineSaysSoAndKeepsTheQueue(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	fake.Err = store.ErrUnreachable
	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatal(err)
	}

	res, err := st.Sync(t.Context())
	if !errors.Is(err, store.ErrUnreachable) {
		t.Fatalf("Sync offline = %v, want %v", err, store.ErrUnreachable)
	}
	if res.Sent != 0 {
		t.Errorf("sent = %d, want nothing claimed sent", res.Sent)
	}
	if res.Pending != 1 {
		t.Errorf("pending = %d, want the one change still waiting", res.Pending)
	}
	if len(queued(t, dir)) != 1 {
		t.Error("the queued change was lost by a sync that could not reach the server")
	}
}

func TestLostWritesSurviveARestartAndAreClearedOnceReported(t *testing.T) {
	dir := t.TempDir()
	fake, st := subscribed(t, dir)
	fake.Err = store.ErrUnreachable

	for range maxQueued + 1 {
		if err := st.SetRead(t.Context(), uuid.New(), true); err != nil {
			t.Fatal(err)
		}
	}
	if got := st.State().Dropped; got != 1 {
		t.Fatalf("dropped = %d, want 1", got)
	}

	again := Wrap(fake, server, dir)
	if got := again.State().Dropped; got != 1 {
		t.Errorf("dropped = %d after a restart, want 1 — the loss happened in the process that is now gone, "+
			"so an in-memory count can never be reported", got)
	}

	fake.Err = nil
	res, err := again.Sync(t.Context())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Dropped != 1 {
		t.Errorf("sync reported %d lost, want 1", res.Dropped)
	}
	if got := again.State().Dropped; got != 0 {
		t.Errorf("dropped = %d after being reported, want it cleared", got)
	}

	third := Wrap(fake, server, dir)
	if got := third.State().Dropped; got != 0 {
		t.Errorf("dropped = %d after a restart, want the cleared count to have reached the disk", got)
	}
	if _, err := os.Stat(filepath.Join(dir, QueueFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Error("the queue file survived a sync that emptied it")
	}
}
