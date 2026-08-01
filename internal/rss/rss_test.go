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

func TestFetchFeedRejectsAtom(t *testing.T) {
	atom := `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <entry><title>Entry One</title><link href="https://example.com/1"/></entry>
</feed>`
	srv := serve(t, http.StatusOK, atom)

	if _, err := FetchFeed(t.Context(), srv.URL); err == nil {
		t.Fatal("expected error for Atom feed, got nil")
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
