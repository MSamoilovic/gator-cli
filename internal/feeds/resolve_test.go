package feeds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testFeedXML = `<?xml version="1.0"?>
<rss version="2.0"><channel><title>Julia Evans</title></channel></rss>`

func site(t *testing.T, page string, feeds map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := feeds[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(body))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveLeavesARealFeedAlone(t *testing.T) {
	srv := site(t, "", map[string]string{"/": testFeedXML})

	feed, url, err := resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if url != srv.URL {
		t.Errorf("url = %q, want the one that was given: %q", url, srv.URL)
	}
	if feed.Channel.Title != "Julia Evans" {
		t.Errorf("title = %q", feed.Channel.Title)
	}
}

func TestResolveFollowsWhatThePageAdvertises(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/atom+xml" href="/atom.xml">
	</head></html>`
	srv := site(t, page, map[string]string{"/atom.xml": testFeedXML})

	feed, url, err := resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := srv.URL + "/atom.xml"; url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
	if feed.Channel.Title != "Julia Evans" {
		t.Errorf("title = %q", feed.Channel.Title)
	}
}

func TestResolveTakesTheFirstAdvertisedFeed(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" title="Feed" href="/feed/">
		<link rel="alternate" type="application/rss+xml" title="Comments Feed" href="/comments/feed/">
	</head></html>`
	srv := site(t, page, map[string]string{
		"/feed/":          testFeedXML,
		"/comments/feed/": testFeedXML,
	})

	_, url, err := resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := srv.URL + "/feed/"; url != want {
		t.Errorf("url = %q, want the main feed %q", url, want)
	}
}

func TestResolveOnAPageWithNoFeed(t *testing.T) {
	srv := site(t, `<html><head><title>Nothing here</title></head></html>`, nil)

	_, _, err := resolve(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for a page with no feed, got nil")
	}
	if !strings.Contains(err.Error(), "html") {
		t.Errorf("error = %q, want it to say what it got instead", err)
	}
}

func TestResolveWhenTheAdvertisedFeedIsBroken(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" href="/feed.xml">
	</head></html>`
	srv := site(t, page, map[string]string{"/feed.xml": "<html><body>nope</body></html>"})

	_, _, err := resolve(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	for _, want := range []string{srv.URL, "/feed.xml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
}

func TestResolvePropagatesTransportErrors(t *testing.T) {
	_, _, err := resolve(context.Background(), "http://127.0.0.1:1/nothing")
	if err == nil {
		t.Fatal("expected an error for an unreachable host, got nil")
	}
}

func TestResolveStoresWhereAPermanentRedirectLands(t *testing.T) {
	feed := site(t, "", map[string]string{"/feed.xml": testFeedXML})
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, feed.URL+"/feed.xml", http.StatusMovedPermanently)
	}))
	t.Cleanup(old.Close)

	_, url, err := resolve(context.Background(), old.URL)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := feed.URL + "/feed.xml"; url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
}

func TestResolveKeepsTheGivenURLOnATemporaryRedirect(t *testing.T) {
	feed := site(t, "", map[string]string{"/feed.xml": testFeedXML})
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, feed.URL+"/feed.xml", http.StatusFound)
	}))
	t.Cleanup(old.Close)

	_, url, err := resolve(context.Background(), old.URL)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if url != old.URL {
		t.Errorf("url = %q, want the address that was given: %q", url, old.URL)
	}
}
