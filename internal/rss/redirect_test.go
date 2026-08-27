package rss

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func feedServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleFeed))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func redirectTo(t *testing.T, target string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPermanentRedirectReportsTheNewURL(t *testing.T) {
	feed := feedServer(t)
	old := redirectTo(t, feed.URL+"/feed.xml", http.StatusMovedPermanently)

	_, next, err := FetchFeed(t.Context(), Source{URL: old.URL})
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if want := feed.URL + "/feed.xml"; next.URL != want {
		t.Errorf("URL = %q, want %q", next.URL, want)
	}
}

func TestPermanentRedirect308(t *testing.T) {
	feed := feedServer(t)
	old := redirectTo(t, feed.URL+"/feed.xml", http.StatusPermanentRedirect)

	_, next, err := FetchFeed(t.Context(), Source{URL: old.URL})
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if want := feed.URL + "/feed.xml"; next.URL != want {
		t.Errorf("URL = %q, want %q", next.URL, want)
	}
}

func TestTemporaryRedirectKeepsTheOldURL(t *testing.T) {
	feed := feedServer(t)

	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusSeeOther} {
		old := redirectTo(t, feed.URL+"/feed.xml", status)

		_, next, err := FetchFeed(t.Context(), Source{URL: old.URL})
		if err != nil {
			t.Fatalf("FetchFeed after %d: %v", status, err)
		}
		if next.URL != old.URL {
			t.Errorf("status %d moved the feed to %q; a temporary redirect must not", status, next.URL)
		}
	}
}

func TestOneTemporaryHopSpoilsTheWholeChain(t *testing.T) {
	feed := feedServer(t)
	middle := redirectTo(t, feed.URL+"/feed.xml", http.StatusFound)
	first := redirectTo(t, middle.URL, http.StatusMovedPermanently)

	_, next, err := FetchFeed(t.Context(), Source{URL: first.URL})
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if next.URL != first.URL {
		t.Errorf("URL = %q, want the original: a chain is only permanent if every hop is", next.URL)
	}
}

func TestNoRedirectLeavesTheURLAlone(t *testing.T) {
	srv := feedServer(t)

	_, next, err := FetchFeed(t.Context(), Source{URL: srv.URL})
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if next.URL != srv.URL {
		t.Errorf("URL = %q, want %q", next.URL, srv.URL)
	}
}

func TestPermanentRedirectSurvivesNotModified(t *testing.T) {
	unchanged := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(unchanged.Close)
	old := redirectTo(t, unchanged.URL+"/feed.xml", http.StatusMovedPermanently)

	_, next, err := FetchFeed(t.Context(), Source{URL: old.URL, ETag: `"abc"`})
	if err != ErrNotModified {
		t.Fatalf("err = %v, want ErrNotModified", err)
	}
	if want := unchanged.URL + "/feed.xml"; next.URL != want {
		t.Errorf("URL = %q, want %q: a feed that moved and then said 304 has still moved", next.URL, want)
	}
	if next.ETag != `"abc"` {
		t.Errorf("ETag = %q, want the one that was sent to be kept", next.ETag)
	}
}

func TestValidatorsSurviveARedirect(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"moved"`)
		w.Write([]byte(sampleFeed))
	}))
	t.Cleanup(feed.Close)
	old := redirectTo(t, feed.URL+"/feed.xml", http.StatusMovedPermanently)

	_, next, err := FetchFeed(t.Context(), Source{URL: old.URL})
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if next.ETag != `"moved"` {
		t.Errorf("ETag = %q, want the one the new location sent", next.ETag)
	}
	if next.URL == old.URL {
		t.Error("URL was not updated")
	}
}

func TestRedirectLoopIsAnError(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path+"/on", http.StatusMovedPermanently)
	}))
	t.Cleanup(srv.Close)

	_, next, err := FetchFeed(t.Context(), Source{URL: srv.URL})
	if err == nil {
		t.Fatal("expected an error for an endless redirect chain, got nil")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Errorf("error = %q, want it to mention redirects", err)
	}
	if next.URL != srv.URL {
		t.Errorf("URL = %q, want the original left alone on failure", next.URL)
	}
}

func TestFailedFetchLeavesTheSourceAlone(t *testing.T) {
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(gone.Close)
	old := redirectTo(t, gone.URL+"/feed.xml", http.StatusMovedPermanently)

	before := Source{URL: old.URL, ETag: `"keep"`, LastModified: "Mon, 01 Jan 2026 00:00:00 GMT"}
	_, next, err := FetchFeed(t.Context(), before)
	if err == nil {
		t.Fatal("expected an error for 404, got nil")
	}
	if next != before {
		t.Errorf("source = %+v, want it untouched on failure: %+v", next, before)
	}
}
