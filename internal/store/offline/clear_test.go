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

func withQueuedWrite(t *testing.T, dir string) {
	t.Helper()

	one := post("one", wallClock(time.Now()), uuid.New())
	fake, st := subscribed(t, dir, one)
	fake.Err = store.ErrUnreachable
	if err := st.SetRead(t.Context(), one.ID, true); err != nil {
		t.Fatalf("SetRead offline: %v", err)
	}
	if n := len(queued(t, dir)); n != 1 {
		t.Fatalf("queued %d changes, want 1", n)
	}
}

func exists(t *testing.T, dir, name string) bool {
	t.Helper()

	_, err := os.Stat(filepath.Join(dir, name))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func TestACacheWrittenForAnotherUserIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	filled(t, dir, post("one", time.Now(), uuid.New()))

	bob := storetest.New("bob")
	bob.Err = store.ErrUnreachable
	st := Wrap(bob, server, "bob", dir)

	if _, err := st.Me(t.Context()); !errors.Is(err, store.ErrNeverSynced) {
		t.Fatalf("Me as another user = %v, want ErrNeverSynced", err)
	}
	posts, err := st.Posts(t.Context(), store.PostQuery{})
	if err != nil || len(posts) != 0 {
		t.Errorf("Posts as another user = %d posts, %v; want none", len(posts), err)
	}
}

func TestClearRemovesBothFilesAndCountsWhatWasWaiting(t *testing.T) {
	dir := t.TempDir()
	withQueuedWrite(t, dir)

	lost, err := Clear(dir)
	if err != nil || lost != 1 {
		t.Fatalf("Clear = %d, %v; want 1 lost change", lost, err)
	}
	for _, name := range []string{FileName, QueueFileName} {
		if exists(t, dir, name) {
			t.Errorf("%s survived Clear", name)
		}
	}

	if lost, err := Clear(dir); err != nil || lost != 0 {
		t.Errorf("Clear on nothing = %d, %v; want a quiet 0", lost, err)
	}
}

func TestOnlyTheSameUserOnTheSameServerKeepsTheOfflineData(t *testing.T) {
	cases := []struct {
		name, server, user string
		kept               bool
	}{
		{"same user and server", server, "marko", true},
		{"another user", server, "bob", false},
		{"another server", "https://other.example.test", "marko", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			withQueuedWrite(t, dir)

			lost, err := ClearUnlessOwnedBy(dir, c.server, c.user)
			if err != nil {
				t.Fatal(err)
			}
			if c.kept {
				if lost != 0 || !exists(t, dir, FileName) || len(queued(t, dir)) != 1 {
					t.Errorf("lost %d and cleared the files of the account that owns them", lost)
				}
				return
			}
			if lost != 1 || exists(t, dir, FileName) || exists(t, dir, QueueFileName) {
				t.Errorf("lost = %d, cache left = %v, queue left = %v; want 1 and both gone",
					lost, exists(t, dir, FileName), exists(t, dir, QueueFileName))
			}
		})
	}
}

func TestAQueueWithoutAnOwnedCacheIsCleared(t *testing.T) {
	dir := t.TempDir()
	withQueuedWrite(t, dir)
	if err := os.Remove(filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}

	if lost, err := ClearUnlessOwnedBy(dir, server, "marko"); err != nil || lost != 1 {
		t.Errorf("ClearUnlessOwnedBy = %d, %v; want the unowned queue cleared", lost, err)
	}
}
