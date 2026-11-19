package remote

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

func TestA429BecomesErrRateLimitedWithTheServersWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(wire.Error{Code: wire.CodeRateLimited, Error: "too many requests; try again in 7s"})
	}))
	t.Cleanup(srv.Close)

	_, err := New(srv.URL, "gat_test").Subscriptions(t.Context())
	if !errors.Is(err, store.ErrRateLimited) {
		t.Fatalf("errors.Is(%v, ErrRateLimited) = false", err)
	}
	if wait, ok := store.RetryAfter(err); !ok || wait != 7*time.Second {
		t.Errorf("RetryAfter = %v, %v; want 7s from the header", wait, ok)
	}
	for _, unwanted := range []error{store.ErrUnauthorized, store.ErrUnreachable, store.ErrPlanLimit} {
		if errors.Is(err, unwanted) {
			t.Errorf("a 429 also matched %v", unwanted)
		}
	}
}

func TestMarkingMoreThanABatchReadIsSplitIntoBatchesTheServerTakes(t *testing.T) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req wire.MarkReadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		sizes = append(sizes, len(req.PostIDs))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	ids := make([]uuid.UUID, 2*wire.MaxReadBatch+1)
	for i := range ids {
		ids[i] = uuid.New()
	}
	if err := New(srv.URL, "gat_test").SetAllRead(t.Context(), ids); err != nil {
		t.Fatal(err)
	}

	want := []int{wire.MaxReadBatch, wire.MaxReadBatch, 1}
	if len(sizes) != len(want) || sizes[0] != want[0] || sizes[1] != want[1] || sizes[2] != want[2] {
		t.Errorf("batches = %v, want %v", sizes, want)
	}
}

func TestAddingFeedsWaitsOutA429InsteadOfFailingTheImport(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wire.AddFeedResponse{Feed: wire.Feed{Name: "Example"}, Created: true})
	}))
	t.Cleanup(srv.Close)

	start := time.Now()
	results := New(srv.URL, "gat_test").AddFeeds(t.Context(), []feeds.Entry{{URL: "https://example.com/feed.xml"}}, nil)
	if err := results[0].Err; err != nil {
		t.Fatalf("AddFeeds = %v, want the second attempt to succeed", err)
	}
	if calls.Load() != 2 {
		t.Errorf("server saw %d calls, want the refused one and the retry", calls.Load())
	}
	if waited := time.Since(start); waited < time.Second {
		t.Errorf("retried after %v, want at least the 1s the server asked for", waited)
	}
}
