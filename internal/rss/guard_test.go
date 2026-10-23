package rss_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/fetch"
	"github.com/MSamoilovic/gator-cli/internal/rss"
)

const feed = `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><link>http://x</link>` +
	`<description>d</description><item><title>I</title><link>http://x/1</link></item></channel></rss>`

func TestFetchFeedRefusesAPrivateAddressWhenGuarded(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Write([]byte(feed))
	}))
	defer srv.Close()

	_, _, err := rss.FetchFeed(fetch.PublicOnly(context.Background()), rss.Source{URL: srv.URL})
	if !errors.Is(err, fetch.ErrBlocked) {
		t.Fatalf("FetchFeed through the guard = %v, want fetch.ErrBlocked", err)
	}
	if hits != 0 {
		t.Errorf("the feed was fetched %d times — the address must be refused before the connection", hits)
	}

	if _, _, err := rss.FetchFeed(context.Background(), rss.Source{URL: srv.URL}); err != nil {
		t.Fatalf("FetchFeed without the guard = %v, want the feed — the CLI fetches what the user named", err)
	}
}
