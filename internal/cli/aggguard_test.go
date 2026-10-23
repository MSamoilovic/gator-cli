package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

func TestAggRefusesToFetchAPrivateAddress(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Internal</title>` +
			`<link>http://x</link><description>d</description></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	db := testdb.Open(t)
	user := db.User(t, "alice")
	feed := db.Feed(t, user.ID, "Internal", srv.URL)
	db.Follow(t, user.ID, feed.ID)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := capture(t, func() error {
			return handlerAgg(ctx, &state{Db: db.Queries}, command{Name: "agg", Args: []string{"1h"}})
		}); err != nil {
			t.Error(err)
		}
	}()

	var lastError string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		db.QueryRow(t, &lastError, "select last_error from feeds where id = $1", feed.ID)
		if lastError != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done

	if !strings.Contains(lastError, "public internet") {
		t.Errorf("last_error = %q, want the guard's refusal — a feed added through the server can "+
			"redirect inward after it was accepted, and agg is what fetches it next", lastError)
	}
	if hits != 0 {
		t.Errorf("agg fetched the private address %d times", hits)
	}
}
