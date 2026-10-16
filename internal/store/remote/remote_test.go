package remote

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"
)

func serving(t *testing.T, status int, payload wire.Error) *Store {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload.Error != "" {
			_ = json.NewEncoder(w).Encode(payload)
		}
	}))
	t.Cleanup(srv.Close)

	return New(srv.URL, "gat_test")
}

func TestServerErrorsBecomeStoreSentinels(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		payload wire.Error
		want    error
	}{
		{"nedostaje token", http.StatusUnauthorized, wire.Error{Code: wire.CodeUnauthorized, Error: "missing bearer token"}, store.ErrUnauthorized},
		{"token nije validan", http.StatusUnauthorized, wire.Error{Code: wire.CodeUnauthorized, Error: "invalid token"}, store.ErrUnauthorized},
		{"401 bez tela", http.StatusUnauthorized, wire.Error{}, store.ErrUnauthorized},
		{"403", http.StatusForbidden, wire.Error{}, store.ErrUnauthorized},
		{"402 je limit plana", http.StatusPaymentRequired, wire.Error{Code: "plan_limit", Error: "free plan allows 50 feeds"}, store.ErrPlanLimit},
		{"feed ne postoji", http.StatusNotFound, wire.Error{Code: wire.CodeFeedNotFound, Error: "feed not found"}, store.ErrFeedNotFound},
		{"ne prati feed", http.StatusConflict, wire.Error{Code: wire.CodeNotFollowed, Error: "you do not follow that feed"}, store.ErrNotFollowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := serving(t, tt.status, tt.payload).Subscriptions(t.Context())
			if err == nil {
				t.Fatalf("status %d produced no error", tt.status)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("errors.Is(%v, %v) = false; the TUI matches on the sentinel, not the status", err, tt.want)
			}
		})
	}
}

func TestAnOrdinaryServerFailureMatchesNoSentinel(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadRequest, http.StatusBadGateway} {
		_, err := serving(t, status, wire.Error{Code: wire.CodeInternal, Error: "internal error"}).Subscriptions(t.Context())
		if err == nil {
			t.Fatalf("status %d produced no error", status)
		}
		for _, unwanted := range []error{store.ErrUnauthorized, store.ErrPlanLimit, store.ErrUnreachable, store.ErrFeedNotFound} {
			if errors.Is(err, unwanted) {
				t.Errorf("status %d matched %v, so the TUI would show the wrong instruction", status, unwanted)
			}
		}
	}
}

func TestAServerErrorKeepsItsMessageForTheUser(t *testing.T) {
	_, err := serving(t, http.StatusPaymentRequired, wire.Error{Code: "plan_limit", Error: "free plan allows 50 feeds"}).Subscriptions(t.Context())
	if err == nil {
		t.Fatal("no error")
	}
	if got, want := err.Error(), "free plan allows 50 feeds"; got != want {
		t.Errorf("message = %q, want the server's own sentence %q", got, want)
	}
}

func TestAnUnreachableServerIsDistinguishableFromAnHTTPError(t *testing.T) {
	st := New("http://127.0.0.1:1", "gat_test")

	_, err := st.Subscriptions(t.Context())
	if err == nil {
		t.Fatal("dialling a closed port produced no error")
	}
	if !errors.Is(err, store.ErrUnreachable) {
		t.Errorf("errors.Is(%v, store.ErrUnreachable) = false", err)
	}
	for _, unwanted := range []error{store.ErrUnauthorized, store.ErrPlanLimit} {
		if errors.Is(err, unwanted) {
			t.Errorf("a dial failure also matched %v", unwanted)
		}
	}
}

func TestASuccessfulCallMatchesNoSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subscriptions":[]}`))
	}))
	t.Cleanup(srv.Close)

	if _, err := New(srv.URL, "gat_test").Subscriptions(t.Context()); err != nil {
		t.Fatalf("a 200 produced an error: %v", err)
	}
}

func TestRefreshIsUnsupportedRatherThanAFailedRequest(t *testing.T) {
	st := New("http://127.0.0.1:1", "gat_test")

	_, err := st.Refresh(t.Context())
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("Refresh error = %v, want errors.ErrUnsupported so the TUI can hide the key", err)
	}
	if errors.Is(err, store.ErrUnreachable) {
		t.Error("Refresh reported the server as unreachable; it never makes a request")
	}
}
