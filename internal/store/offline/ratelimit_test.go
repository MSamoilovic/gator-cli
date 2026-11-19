package offline

import (
	"errors"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"

	"github.com/google/uuid"
)

func limited() error {
	return &store.RateLimitError{RetryAfter: 30 * time.Second}
}

func TestARateLimitedWriteWaitsInTheQueueInsteadOfFailing(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)

	fake.Err = limited()
	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatalf("SetRead under 429 = %v, want it queued so the reader's mark stays true", err)
	}
	if _, err := st.Bookmark(t.Context(), one.ID); err != nil {
		t.Fatalf("Bookmark under 429 = %v, want it queued", err)
	}

	if got := len(queued(t, dir)); got != 2 {
		t.Fatalf("queued %d changes, want the 2 the server refused for now", got)
	}
	if !st.State().Online {
		t.Error("a 429 marked the server offline; it answered, it only asked to wait")
	}

	fake.Err = nil
	res, err := st.Sync(t.Context())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Sent != 2 {
		t.Errorf("sent = %d, want both postponed changes", res.Sent)
	}
	read, _ := fake.ReadIDs(t.Context())
	saved, _ := fake.BookmarkedIDs(t.Context())
	if len(read) != 1 || len(saved) != 1 {
		t.Errorf("server has %d read and %d saved, want one of each", len(read), len(saved))
	}
}

func TestARateLimitedReplayStopsAndLosesNothing(t *testing.T) {
	dir := t.TempDir()
	one := post("one", wallClock(time.Now()), uuid.New())
	two := post("two", wallClock(time.Now()), one.FeedID)
	fake, st := subscribed(t, dir, one, two)

	fake.Err = store.ErrUnreachable
	for _, p := range []store.Post{one, two} {
		if err := st.SetRead(t.Context(), p.ID, true); err != nil {
			t.Fatal(err)
		}
	}

	fake.Err = limited()
	res, err := st.Sync(t.Context())
	if !errors.Is(err, store.ErrRateLimited) {
		t.Fatalf("Sync under 429 = %v, want %v", err, store.ErrRateLimited)
	}
	if res.Sent != 0 || res.Dropped != 0 {
		t.Errorf("sent %d and dropped %d, want neither: a 429 is a pause, not a refusal", res.Sent, res.Dropped)
	}
	if got := len(queued(t, dir)); got != 2 {
		t.Fatalf("queue holds %d after a 429, want both changes kept", got)
	}

	fake.Err = nil
	if res, err := st.Sync(t.Context()); err != nil || res.Sent != 2 {
		t.Errorf("Sync after the wait = %+v, %v; want both changes sent", res, err)
	}
}
