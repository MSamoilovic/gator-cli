package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Boot.dev &amp; Friends</title>
    <link>https://blog.boot.dev</link>
    <description>Backend &amp; more</description>
    <item>
      <title>Learn Go &amp; Chill</title>
      <link>https://blog.boot.dev/go</link>
      <description>Goroutines &lt;3</description>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>Second Post</title>
      <link>https://blog.boot.dev/2</link>
    </item>
  </channel>
</rss>`

func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "gator" {
			t.Errorf("User-Agent = %q, want %q", got, "gator")
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchFeed(t *testing.T) {
	srv := serve(t, http.StatusOK, sampleFeed)

	feed, err := FetchFeed(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	if got, want := feed.Channel.Title, "Boot.dev & Friends"; got != want {
		t.Errorf("channel title = %q, want %q", got, want)
	}
	if got, want := feed.Channel.Description, "Backend & more"; got != want {
		t.Errorf("channel description = %q, want %q", got, want)
	}
	if got, want := len(feed.Channel.Item), 2; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
	}

	first := feed.Channel.Item[0]
	if got, want := first.Title, "Learn Go & Chill"; got != want {
		t.Errorf("item title = %q, want %q", got, want)
	}
	if got, want := first.Description, "Goroutines <3"; got != want {
		t.Errorf("item description = %q, want %q", got, want)
	}
	if got, want := first.Link, "https://blog.boot.dev/go"; got != want {
		t.Errorf("item link = %q, want %q", got, want)
	}
	if got, want := first.PubDate, "Mon, 02 Jan 2006 15:04:05 -0700"; got != want {
		t.Errorf("item pubDate = %q, want %q", got, want)
	}
}

func TestFetchFeedMissingFieldsAreEmpty(t *testing.T) {
	srv := serve(t, http.StatusOK, sampleFeed)

	feed, err := FetchFeed(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	second := feed.Channel.Item[1]
	if second.PubDate != "" || second.Description != "" {
		t.Errorf("missing fields should stay empty, got pubDate=%q description=%q",
			second.PubDate, second.Description)
	}
}

func TestFetchFeedMalformedXML(t *testing.T) {
	srv := serve(t, http.StatusOK, "<rss><channel><item></channel></rss>")

	if _, err := FetchFeed(t.Context(), srv.URL); err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
}

// Stariji feedovi jos deklarisu ISO-8859-1 ili windows-1252. Bajtovi ispod su
// namerno latin-1, ne UTF-8 — bez CharsetReader-a ovo ne prodje kroz parser.
func TestFetchFeedDecodesNonUTF8(t *testing.T) {
	cases := []struct{ name, declared string }{
		{"latin-1", "ISO-8859-1"},
		{"windows-1252", "windows-1252"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `<?xml version="1.0" encoding="` + tc.declared + `"?>
<rss version="2.0"><channel>
  <title>Caf` + "\xe9" + ` Wars</title>
  <item><title>Se` + "\xf1" + `or</title></item>
</channel></rss>`
			srv := serve(t, http.StatusOK, body)

			feed, err := FetchFeed(t.Context(), srv.URL)
			if err != nil {
				t.Fatalf("FetchFeed: %v", err)
			}
			if got, want := feed.Channel.Title, "Café Wars"; got != want {
				t.Errorf("channel title = %q, want %q", got, want)
			}
			if got, want := feed.Channel.Item[0].Title, "Señor"; got != want {
				t.Errorf("item title = %q, want %q", got, want)
			}
		})
	}
}

func TestFetchFeedUnknownCharsetIsAnError(t *testing.T) {
	body := `<?xml version="1.0" encoding="nepostojeci-charset"?><rss><channel></channel></rss>`
	srv := serve(t, http.StatusOK, body)

	_, err := FetchFeed(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for an unknown charset, got nil")
	}
	if !strings.Contains(err.Error(), "nepostojeci-charset") {
		t.Errorf("error = %q, want it to name the charset", err)
	}
}

const sampleAtom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>The Go Blog</title>
  <subtitle>Go &amp; friends</subtitle>
  <link rel="self" href="https://go.dev/blog/feed.atom"/>
  <link href="https://go.dev/blog"/>
  <entry>
    <title>Range Over Func</title>
    <link rel="edit" href="https://go.dev/api/1"/>
    <link rel="alternate" href="https://go.dev/blog/range-func"/>
    <summary>Iterators &lt;3</summary>
    <published>2026-07-27T18:55:00Z</published>
    <updated>2026-08-01T10:00:00Z</updated>
  </entry>
  <entry>
    <title>Only Content</title>
    <link href="https://go.dev/blog/two"/>
    <content>Full text here</content>
    <updated>2026-08-02T10:00:00Z</updated>
  </entry>
</feed>`

func TestFetchFeedParsesAtom(t *testing.T) {
	srv := serve(t, http.StatusOK, sampleAtom)

	feed, err := FetchFeed(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	if got, want := feed.Channel.Title, "The Go Blog"; got != want {
		t.Errorf("channel title = %q, want %q", got, want)
	}
	if got, want := feed.Channel.Description, "Go & friends"; got != want {
		t.Errorf("channel description = %q, want %q", got, want)
	}
	// rel="self" vodi na feed, ne na sajt — mora biti preskocen.
	if got, want := feed.Channel.Link, "https://go.dev/blog"; got != want {
		t.Errorf("channel link = %q, want %q", got, want)
	}
	if got, want := len(feed.Channel.Item), 2; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
	}

	first := feed.Channel.Item[0]
	if got, want := first.Title, "Range Over Func"; got != want {
		t.Errorf("item title = %q, want %q", got, want)
	}
	if got, want := first.Link, "https://go.dev/blog/range-func"; got != want {
		t.Errorf("item link = %q, want %q — rel=edit must not win", got, want)
	}
	if got, want := first.Description, "Iterators <3"; got != want {
		t.Errorf("item description = %q, want %q", got, want)
	}
	if got, want := first.PubDate, "2026-07-27T18:55:00Z"; got != want {
		t.Errorf("item pubDate = %q, want %q — published, not updated", got, want)
	}
}

func TestFetchFeedAtomFallsBackToContentAndUpdated(t *testing.T) {
	srv := serve(t, http.StatusOK, sampleAtom)

	feed, err := FetchFeed(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	second := feed.Channel.Item[1]
	if got, want := second.Description, "Full text here"; got != want {
		t.Errorf("description = %q, want %q — content when summary is missing", got, want)
	}
	if got, want := second.PubDate, "2026-08-02T10:00:00Z"; got != want {
		t.Errorf("pubDate = %q, want %q — updated when published is missing", got, want)
	}
}

func TestFetchFeedParsesRSS1(t *testing.T) {
	rdf := `<?xml version="1.0" encoding="ISO-8859-1"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns="http://purl.org/rss/1.0/"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel rdf:about="https://slashdot.org/">
    <title>Slashdot</title>
    <link>https://slashdot.org/</link>
    <description>News for nerds</description>
  </channel>
  <item rdf:about="https://slashdot.org/story/1">
    <title>Caf` + "\xe9" + ` Story</title>
    <link>https://slashdot.org/story/1</link>
    <description>Something happened</description>
    <dc:date>2026-08-13T09:00:00+00:00</dc:date>
  </item>
</rdf:RDF>`
	srv := serve(t, http.StatusOK, rdf)

	feed, err := FetchFeed(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	if got, want := feed.Channel.Title, "Slashdot"; got != want {
		t.Errorf("channel title = %q, want %q", got, want)
	}
	if got, want := len(feed.Channel.Item), 1; got != want {
		t.Fatalf("item count = %d, want %d — items are siblings of <channel>", got, want)
	}

	item := feed.Channel.Item[0]
	if got, want := item.Title, "Café Story"; got != want {
		t.Errorf("item title = %q, want %q", got, want)
	}
	if got, want := item.PubDate, "2026-08-13T09:00:00+00:00"; got != want {
		t.Errorf("item pubDate = %q, want %q — the dc:date value", got, want)
	}
}

func TestFetchFeedUnsupportedRootIsNamed(t *testing.T) {
	srv := serve(t, http.StatusOK, `<html><body>not a feed</body></html>`)

	_, err := FetchFeed(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for a non-feed document, got nil")
	}
	if !strings.Contains(err.Error(), "html") {
		t.Errorf("error = %q, want it to name the root element", err)
	}
}

func TestFetchFeedHTTPError(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"not found", http.StatusNotFound},
		{"forbidden", http.StatusForbidden},
		{"server error", http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := serve(t, tc.status, `<rss version="2.0"><channel></channel></rss>`)

			_, err := FetchFeed(t.Context(), srv.URL)
			if err == nil {
				t.Fatal("expected error for non-2xx status, got nil")
			}
			if !strings.Contains(err.Error(), strconv.Itoa(tc.status)) {
				t.Errorf("error = %q, want it to mention status %d", err, tc.status)
			}
		})
	}
}

func TestFetchFeedEmptyChannelIsNotAnError(t *testing.T) {
	srv := serve(t, http.StatusOK, `<rss version="2.0"><channel><title>Prazan</title></channel></rss>`)

	feed, err := FetchFeed(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
	if len(feed.Channel.Item) != 0 {
		t.Fatalf("item count = %d, want 0", len(feed.Channel.Item))
	}
}

func TestFetchFeedContextCanceled(t *testing.T) {
	srv := serve(t, http.StatusOK, sampleFeed)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := FetchFeed(ctx, srv.URL); err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

func TestFetchFeedBadURL(t *testing.T) {
	if _, err := FetchFeed(t.Context(), "://not-a-url"); err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}
