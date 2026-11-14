package remote

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestARedirectNeverCarriesThePasswordOrTheTokenOnward(t *testing.T) {
	var leaked atomic.Bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte("hunter2")) || r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(elsewhere.Close)

	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusFound} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, elsewhere.URL+r.URL.Path, status)
		}))
		t.Cleanup(srv.Close)

		_, err := Login(t.Context(), srv.URL, "marko", "hunter2")
		if err == nil || !strings.Contains(err.Error(), elsewhere.URL) {
			t.Errorf("login answered %d: err = %v, want an error naming where the server pointed", status, err)
		}
		if _, err := New(srv.URL, "gat_secret").Subscriptions(t.Context()); err == nil {
			t.Errorf("subscriptions answered %d: no error", status)
		}
	}

	if leaked.Load() {
		t.Error("the redirect target received the password or the bearer token")
	}
}

func TestAnOversizedResponseIsANamedErrorNotABrokenJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("["))
		_, _ = w.Write(bytes.Repeat([]byte(" "), maxResponse))
		_, _ = w.Write([]byte("]"))
	}))
	t.Cleanup(srv.Close)

	_, err := New(srv.URL, "gat_test").Subscriptions(t.Context())
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Errorf("err = %v, want ErrResponseTooLarge", err)
	}
}
