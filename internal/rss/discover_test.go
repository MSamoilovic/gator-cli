package rss

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func links(t *testing.T, page, base string) []Link {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parsing base %q: %v", base, err)
	}
	return discoverLinks([]byte(page), u)
}

func urls(links []Link) []string {
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.URL
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDiscoverFindsAdvertisedFeed(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/atom+xml" title="Julia Evans" href="/atom.xml">
	</head><body>...</body></html>`

	got := links(t, page, "https://jvns.ca")
	if len(got) != 1 {
		t.Fatalf("found %v, want one feed", urls(got))
	}
	if got[0].URL != "https://jvns.ca/atom.xml" {
		t.Errorf("URL = %q, want the relative href resolved against the page", got[0].URL)
	}
	if got[0].Title != "Julia Evans" {
		t.Errorf("Title = %q", got[0].Title)
	}
}

func TestDiscoverKeepsTheOrderThePageGave(t *testing.T) {
	// Redosled je znacajan: pozivalac uzima prvi, a sajtovi glavni feed
	// navode ispred komentara.
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" title="Feed" href="/feed/">
		<link rel="alternate" type="application/rss+xml" title="Comments Feed" href="/comments/feed/">
	</head></html>`

	got := urls(links(t, page, "https://krebsonsecurity.com"))
	want := []string{"https://krebsonsecurity.com/feed/", "https://krebsonsecurity.com/comments/feed/"}
	if !equalStrings(got, want) {
		t.Errorf("feeds = %v, want %v", got, want)
	}
}

func TestDiscoverHandlesATagSplitAcrossLines(t *testing.T) {
	// Lobste.rs pise bas ovako. Linijska pretraga bi ga promasila.
	page := "<html><head>\n<link rel=\"alternate\" type=\"application/rss+xml\"\n      title=\"RSS 2.0\" href=\"https://lobste.rs/rss\">\n</head></html>"

	got := links(t, page, "https://lobste.rs")
	if len(got) != 1 || got[0].URL != "https://lobste.rs/rss" {
		t.Errorf("feeds = %v, want the one split across lines", urls(got))
	}
}

func TestDiscoverResolvesEveryKindOfHref(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" href="/root.xml">
		<link rel="alternate" type="application/rss+xml" href="relative.xml">
		<link rel="alternate" type="application/rss+xml" href="https://other.test/absolute.xml">
		<link rel="alternate" type="application/rss+xml" href="//scheme.test/relative.xml">
	</head></html>`

	got := urls(links(t, page, "https://example.test/blog/index.html"))
	want := []string{
		"https://example.test/root.xml",
		"https://example.test/blog/relative.xml",
		"https://other.test/absolute.xml",
		"https://scheme.test/relative.xml",
	}
	if !equalStrings(got, want) {
		t.Errorf("feeds = %v, want %v", got, want)
	}
}

func TestDiscoverIgnoresLinksThatAreNotFeeds(t *testing.T) {
	page := `<html><head>
		<link rel="stylesheet" type="text/css" href="/style.css">
		<link rel="icon" href="/favicon.ico">
		<link rel="canonical" href="https://example.test/">
		<link rel="alternate" hreflang="de" href="/de/">
		<link rel="alternate" type="text/html" href="/print">
		<link rel="alternate" type="application/rss+xml" href="/feed.xml">
	</head></html>`

	got := urls(links(t, page, "https://example.test"))
	if want := []string{"https://example.test/feed.xml"}; !equalStrings(got, want) {
		t.Errorf("feeds = %v, want %v", got, want)
	}
}

func TestDiscoverAcceptsEveryFeedMIMEType(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" href="/1">
		<link rel="alternate" type="application/atom+xml" href="/2">
		<link rel="alternate" type="application/rdf+xml" href="/3">
		<link rel="alternate" type="application/xml" href="/4">
		<link rel="alternate" type="text/xml" href="/5">
	</head></html>`

	if got := links(t, page, "https://example.test"); len(got) != 5 {
		t.Errorf("found %d feeds, want 5: %v", len(got), urls(got))
	}
}

func TestDiscoverIsCaseAndParameterTolerant(t *testing.T) {
	page := `<html><HEAD>
		<LINK REL="Alternate" TYPE="Application/RSS+XML; charset=utf-8" HREF="/feed.xml">
	</HEAD></html>`

	got := links(t, page, "https://example.test")
	if len(got) != 1 || got[0].URL != "https://example.test/feed.xml" {
		t.Errorf("feeds = %v, want the uppercase tag to be found", urls(got))
	}
}

func TestDiscoverDropsRepeatedURLs(t *testing.T) {
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" href="/feed.xml">
		<link rel="alternate" type="application/rss+xml" href="/feed.xml">
	</head></html>`

	if got := links(t, page, "https://example.test"); len(got) != 1 {
		t.Errorf("feeds = %v, want the duplicate dropped", urls(got))
	}
}

func TestDiscoverStopsAtTheBody(t *testing.T) {
	// <link> pripada zaglavlju; ono u telu je tudji sadrzaj, ne ponuda sajta.
	page := `<html><head>
		<link rel="alternate" type="application/rss+xml" href="/real.xml">
	</head><body>
		<link rel="alternate" type="application/rss+xml" href="/injected.xml">
	</body></html>`

	got := urls(links(t, page, "https://example.test"))
	if want := []string{"https://example.test/real.xml"}; !equalStrings(got, want) {
		t.Errorf("feeds = %v, want %v", got, want)
	}
}

func TestDiscoverOnAPageWithNoFeeds(t *testing.T) {
	if got := links(t, `<html><head><title>Nothing</title></head></html>`, "https://example.test"); len(got) != 0 {
		t.Errorf("feeds = %v, want none", urls(got))
	}
}

func TestNotAFeedErrorNamesTheAdvertisedFeed(t *testing.T) {
	page := `<html><head><link rel="alternate" type="application/rss+xml" href="/feed.xml"></head></html>`
	srv := serve(t, http.StatusOK, page)

	_, err := fetch(t, srv.URL)
	if err == nil {
		t.Fatal("expected an error for a web page, got nil")
	}

	var notFeed *NotAFeedError
	if !errors.As(err, &notFeed) {
		t.Fatalf("error %[1]T (%[1]v) does not unwrap to *NotAFeedError", err)
	}
	if len(notFeed.Links) != 1 {
		t.Fatalf("error carries %v, want the one advertised feed", urls(notFeed.Links))
	}
	if !strings.Contains(err.Error(), "/feed.xml") {
		t.Errorf("error = %q, want it to name the feed it found", err)
	}
}

func TestNotAFeedErrorWithoutLinksStillNamesTheRoot(t *testing.T) {
	srv := serve(t, http.StatusOK, `<html><body>not a feed</body></html>`)

	_, err := fetch(t, srv.URL)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var notFeed *NotAFeedError
	if !errors.As(err, &notFeed) {
		t.Fatalf("error %[1]T does not unwrap to *NotAFeedError", err)
	}
	if len(notFeed.Links) != 0 {
		t.Errorf("links = %v, want none", urls(notFeed.Links))
	}
	if !strings.Contains(err.Error(), "html") {
		t.Errorf("error = %q, want it to name the root element", err)
	}
}

func TestNotAFeedErrorForNonHTMLRoot(t *testing.T) {
	srv := serve(t, http.StatusOK, `<opml version="2.0"><body/></opml>`)

	_, err := fetch(t, srv.URL)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "opml") {
		t.Errorf("error = %q, want it to name the root element", err)
	}
}

func TestDiscoveryResolvesAgainstTheURLAfterRedirects(t *testing.T) {
	// Relativna adresa u stranici vazi prema mestu na kom smo zavrsili, ne
	// prema onome sto je zatrazeno.
	final := serve(t, http.StatusOK,
		`<html><head><link rel="alternate" type="application/rss+xml" href="feed.xml"></head></html>`)

	redirector := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/blog/", http.StatusMovedPermanently)
	})

	_, err := fetch(t, redirector.URL)
	var notFeed *NotAFeedError
	if !errors.As(err, &notFeed) {
		t.Fatalf("error %[1]T does not unwrap to *NotAFeedError", err)
	}
	if len(notFeed.Links) != 1 {
		t.Fatalf("links = %v, want one", urls(notFeed.Links))
	}
	if want := final.URL + "/blog/feed.xml"; notFeed.Links[0].URL != want {
		t.Errorf("feed = %q, want %q", notFeed.Links[0].URL, want)
	}
}
