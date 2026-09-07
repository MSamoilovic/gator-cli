package article

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const page = `<!doctype html>
<html><head><title>Ime stranice</title></head>
<body>
  <nav><a href="/a">Home</a><a href="/b">Sport</a><a href="/c">News</a></nav>
  <article>
    <h1>Forests withered during an ancient warming episode</h1>
    <p>A long time ago the planet warmed sharply, and the forests of the tropics
       thinned out in a way that researchers are only now able to reconstruct in
       any detail from the fossil record they left behind.</p>
    <p>The team behind the work compared pollen counts across two dozen sites and
       found the same signal in every one of them, which is the sort of agreement
       that rarely happens by accident in this kind of study.</p>
  </article>
  <footer>Copyright, all rights reserved, subscribe to our newsletter today</footer>
</body></html>`

func TestExtractPullsTheArticleOut(t *testing.T) {
	got, err := Extract(strings.NewReader(page), "https://example.test/story")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	for _, want := range []string{"planet warmed sharply", "pollen counts"} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("text is missing %q:\n%s", want, got.Text)
		}
	}
	for _, unwanted := range []string{"subscribe to our newsletter", "Sport"} {
		if strings.Contains(got.Text, unwanted) {
			t.Errorf("text still carries the page furniture %q:\n%s", unwanted, got.Text)
		}
	}
	if got.Title == "" {
		t.Error("no title")
	}
}

func TestExtractSeparatesParagraphs(t *testing.T) {
	got, err := Extract(strings.NewReader(page), "https://example.test/story")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(got.Text, "\n\n") {
		t.Errorf("paragraphs run together:\n%s", got.Text)
	}
	if strings.Contains(got.Text, "  ") {
		t.Errorf("text keeps the source indentation:\n%s", got.Text)
	}
}

func TestPageWithoutAnArticleIsNotAnImprovement(t *testing.T) {
	// Readability nad stranicom bez clanka vrati tekst navigacije, sto je
	// kratko; zato pozivalac poredi sa onim sto je feed vec dao.
	nav := `<html><body><nav>Home Sport News</nav></body></html>`
	stub := "A perfectly serviceable one-sentence summary that the feed already gave us."

	got, err := Extract(strings.NewReader(nav), "https://example.test/")
	if err == nil && Improves(got.Text, stub) {
		t.Errorf("navigation text %q was accepted over the feed summary", got.Text)
	}
}

func TestImproves(t *testing.T) {
	tests := []struct {
		got, had string
		want     bool
	}{
		{"duzi tekst clanka", "kratko", true},
		{"kratko", "duzi tekst clanka", false},
		{"isto", "isto", false},
		{"nesto", "", true},
		{"", "", false},
		{"   ", "x", false},
	}

	for _, tt := range tests {
		if got := Improves(tt.got, tt.had); got != tt.want {
			t.Errorf("Improves(%q, %q) = %v, want %v", tt.got, tt.had, got, tt.want)
		}
	}
}

func TestExtractRejectsABadURL(t *testing.T) {
	if _, err := Extract(strings.NewReader(page), "://nonsense"); err == nil {
		t.Error("expected an error for an unparseable URL")
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
		w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)

	got, err := Fetch(context.Background(), srv.URL+"/story")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(got.Text, "pollen counts") {
		t.Errorf("text = %q", got.Text)
	}
}

func TestFetchReportsHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, err := Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected an error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to name the status", err)
	}
}

func TestFetchHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Fetch(ctx, "https://example.test/"); err == nil {
		t.Error("expected an error for a cancelled context")
	}
}

func TestNormalise(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"jedan", "jedan"},
		{"  jedan   dva  ", "jedan dva"},
		{"a\n\n\n\nb", "a\n\nb"},
		{"a\n   \nb", "a\n\nb"},
		{"red\n  uvucen  red", "red\n\nuvucen red"},
	}

	for _, tt := range tests {
		if got := normalise(tt.in); got != tt.want {
			t.Errorf("normalise(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
