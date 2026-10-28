package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/offline"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"

	"github.com/google/uuid"
)

type syncingFake struct {
	*storetest.Fake
	res store.SyncResult
	err error
}

func (f *syncingFake) State() store.SyncState { return store.SyncState{Pending: f.res.Pending} }

func (f *syncingFake) Sync(context.Context) (store.SyncResult, error) { return f.res, f.err }

func syncState(f *syncingFake) *state {
	return &state{
		Store: f,
		Cfg:   &config.Config{CurrentUserName: f.Identity.Name, ServerURL: "https://reader.example.test"},
	}
}

func TestSyncReportsWhatItSentAndWhatItCached(t *testing.T) {
	f := &syncingFake{Fake: storetest.New("marko"), res: store.SyncResult{Sent: 3, Posts: 200, Feeds: 142}}

	out, err := capture(t, func() error {
		return handlerSync(context.Background(), syncState(f), command{Name: "sync"})
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	for _, want := range []string{"Sent 3", "https://reader.example.test", "200 posts", "142 feeds"} {
		if !strings.Contains(out, want) {
			t.Errorf("sync printed %q, want it to mention %q", out, want)
		}
	}
}

func TestSyncNamesTheChangesTheQueueLost(t *testing.T) {
	f := &syncingFake{Fake: storetest.New("marko"), res: store.SyncResult{Sent: 1, Dropped: 2, Posts: 5, Feeds: 1}}

	out, err := capture(t, func() error {
		return handlerSync(context.Background(), syncState(f), command{Name: "sync"})
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !strings.Contains(out, "Lost 2") {
		t.Errorf("sync printed %q, want the overflowed changes admitted", out)
	}
}

func TestAFailedSyncStillSaysWhatGotThrough(t *testing.T) {
	f := &syncingFake{
		Fake: storetest.New("marko"),
		res:  store.SyncResult{Sent: 2, Pending: 3},
		err:  store.ErrUnreachable,
	}

	out, err := capture(t, func() error {
		return handlerSync(context.Background(), syncState(f), command{Name: "sync"})
	})
	if !errors.Is(err, store.ErrUnreachable) {
		t.Fatalf("sync = %v, want %v", err, store.ErrUnreachable)
	}
	for _, want := range []string{"Sent 2", "3 change(s) still waiting"} {
		if !strings.Contains(out, want) {
			t.Errorf("sync printed %q, want %q — a failed pass still moved some of the queue", out, want)
		}
	}
}

func TestSyncRefusesAStoreThatCannotSync(t *testing.T) {
	f := storetest.New("marko")

	_, err := capture(t, func() error {
		return handlerSync(context.Background(), fakeState(f), command{Name: "sync"})
	})
	if err == nil || !strings.Contains(err.Error(), "nothing to sync") {
		t.Errorf("sync against a store that keeps no cache = %v, want it to say so", err)
	}
}

func TestALocalInstallIsTurnedAwayBeforeSyncRuns(t *testing.T) {
	f := storetest.New("marko")
	cmds := defaultCommands()

	out, err := capture(t, func() error {
		return cmds.run(context.Background(), fakeState(f), command{Name: "sync"})
	})
	if err == nil || !strings.Contains(err.Error(), "sync is for a server") {
		t.Errorf("sync on a local install = %v, want the mode guard to refuse it before the handler runs", err)
	}
	if out != "" {
		t.Errorf("sync printed %q before being refused", out)
	}
}

func TestSyncCountsWhatItSentAlthoughSigningInWouldHaveFlushedItFirst(t *testing.T) {
	dir := t.TempDir()
	const server = "https://reader.example.test"

	fake := storetest.New("marko")
	st := offline.Wrap(fake, server, dir)

	fake.Err = store.ErrUnreachable
	if err := st.SetRead(context.Background(), uuid.New(), true); err != nil {
		t.Fatalf("SetRead offline: %v", err)
	}
	fake.Err = nil

	s := &state{Store: st, Cfg: &config.Config{CurrentUserName: "marko", ServerURL: server}}
	cmds := defaultCommands()
	out, err := capture(t, func() error {
		return cmds.run(context.Background(), s, command{Name: "sync"})
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !strings.Contains(out, "Sent 1") {
		t.Errorf("sync printed %q, want it to count the queued change — middlewareLoggedIn calls Me "+
			"before the handler, and that call alone empties the queue", out)
	}
}
