package feeds

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func entries(names ...string) []Entry {
	out := make([]Entry, len(names))
	for i, n := range names {
		out[i] = Entry{Name: n, URL: "https://example.com/" + n}
	}
	return out
}

func TestAddAllKeepsInputOrder(t *testing.T) {
	in := entries("prvi", "drugi", "treci")

	got := addAll(t.Context(), in, 8, nil, func(_ context.Context, e Entry) AddResult {
		if e.Name == "prvi" {
			time.Sleep(20 * time.Millisecond)
		}
		return AddResult{Entry: e}
	})

	if len(got) != len(in) {
		t.Fatalf("got %d results, want %d", len(got), len(in))
	}
	for i, r := range got {
		if r.Entry.Name != in[i].Name {
			t.Errorf("result %d = %q, want %q", i, r.Entry.Name, in[i].Name)
		}
	}
}

func TestAddAllIsolatesFailures(t *testing.T) {
	boom := errors.New("not a usable RSS feed")

	got := addAll(t.Context(), entries("ok1", "puca", "ok2"), 8, nil, func(_ context.Context, e Entry) AddResult {
		if e.Name == "puca" {
			return AddResult{Entry: e, Err: boom}
		}
		return AddResult{Entry: e, Created: true}
	})

	if !errors.Is(got[1].Err, boom) {
		t.Errorf("failing entry: err = %v, want %v", got[1].Err, boom)
	}
	for _, i := range []int{0, 2} {
		if got[i].Err != nil {
			t.Errorf("entry %d failed because of its neighbour: %v", i, got[i].Err)
		}
		if !got[i].Created {
			t.Errorf("entry %d was not created", i)
		}
	}
}

func TestAddAllRespectsLimit(t *testing.T) {
	const limit = 3

	var inFlight, peak atomic.Int32

	addAll(t.Context(), entries("a", "b", "c", "d", "e", "f", "g", "h"), limit, nil,
		func(_ context.Context, e Entry) AddResult {
			n := inFlight.Add(1)
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			inFlight.Add(-1)
			return AddResult{Entry: e}
		})

	if got := peak.Load(); got > limit {
		t.Errorf("%d entries ran at once, limit is %d", got, limit)
	}
	if peak.Load() < 2 {
		t.Error("nothing ran in parallel, the limit test proves nothing")
	}
}

func TestAddAllSerializesCallback(t *testing.T) {
	calls := 0
	in := entries("a", "b", "c", "d", "e", "f")

	addAll(t.Context(), in, 8, func(AddResult) { calls++ }, func(_ context.Context, e Entry) AddResult {
		return AddResult{Entry: e}
	})

	if calls != len(in) {
		t.Errorf("onResult called %d times, want %d", calls, len(in))
	}
}

func TestAddAllHandlesEmptyAndBadLimit(t *testing.T) {
	if got := addAll(t.Context(), nil, 8, nil, nil); len(got) != 0 {
		t.Errorf("empty input returned %d results", len(got))
	}

	got := addAll(t.Context(), entries("a", "b"), 0, nil, func(_ context.Context, e Entry) AddResult {
		return AddResult{Entry: e}
	})
	if len(got) != 2 {
		t.Errorf("limit 0 returned %d results, want 2", len(got))
	}
}
