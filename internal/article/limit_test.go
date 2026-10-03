package article

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBoundedReaderAllowsExactlyTheLimit(t *testing.T) {
	b := &boundedReader{r: strings.NewReader(strings.Repeat("a", 100)), limit: 100}

	n, err := io.Copy(io.Discard, b)
	if err != nil {
		t.Fatalf("a body of exactly the limit was rejected: %v", err)
	}
	if n != 100 {
		t.Errorf("copied %d bytes, want 100", n)
	}
	if b.exceeded() {
		t.Error("exceeded() is true for a body of exactly the limit")
	}
}

func TestBoundedReaderFlagsOneByteOver(t *testing.T) {
	b := &boundedReader{r: strings.NewReader(strings.Repeat("a", 101)), limit: 100}

	_, err := io.Copy(io.Discard, b)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("Copy = %v, want ErrBodyTooLarge", err)
	}
	if !b.exceeded() {
		t.Error("exceeded() is false after the limit was passed")
	}
}

func TestFetchRefusesAnOversizedPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><body><article><p>`)
		io.Copy(w, io.LimitReader(endlessText{}, maxBody*2))
		fmt.Fprint(w, `</p></article></body></html>`)
	}))
	t.Cleanup(srv.Close)

	_, err := Fetch(t.Context(), srv.URL)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("Fetch = %v, want ErrBodyTooLarge", err)
	}
	if !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error %q does not name the page", err)
	}
}

func TestFetchNeverReturnsTruncatedTextForCaching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><head><title>Huge</title></head><body><article>`)
		for i := 0; i < 400; i++ {
			fmt.Fprintf(w, `<p>Paragraph %d with enough words in it to look like real prose to the extractor.</p>`, i)
		}
		io.Copy(w, io.LimitReader(endlessText{}, maxBody*2))
		fmt.Fprint(w, `</article></body></html>`)
	}))
	t.Cleanup(srv.Close)

	got, err := Fetch(t.Context(), srv.URL)
	if err == nil {
		t.Fatalf("an oversized page was accepted, returning %d runes that would be cached", len([]rune(got.Text)))
	}
	if got.Text != "" {
		t.Errorf("Fetch returned %d runes of text alongside the error — it could still be cached", len([]rune(got.Text)))
	}
}

func TestFetchStopsReadingAnEndlessPage(t *testing.T) {
	var served atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		buf := make([]byte, 32<<10)
		for i := range buf {
			buf[i] = 'x'
		}
		for {
			n, err := w.Write(buf)
			served.Add(int64(n))
			if err != nil || served.Load() > maxBody*8 {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	if _, err := Fetch(t.Context(), srv.URL); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("Fetch = %v, want ErrBodyTooLarge", err)
	}
	if got := served.Load(); got > maxBody*2 {
		t.Errorf("server wrote %d bytes for a %d byte limit — the read is not bounded", got, maxBody)
	}
}

func TestFetchStillReadsALargeButRealisticPage(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<!doctype html><html><head><title>Long read</title></head><body><article>`)
	for i := 0; sb.Len() < 2<<20; i++ {
		fmt.Fprintf(&sb, `<p>Paragraph %d of a genuinely long article, with ordinary sentences that the extractor should keep in full.</p>`, i)
	}
	sb.WriteString(`</article></body></html>`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sb.String())
	}))
	t.Cleanup(srv.Close)

	got, err := Fetch(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("a %d byte page was rejected: %v", sb.Len(), err)
	}
	if !strings.Contains(got.Text, "Paragraph 0 of a genuinely long article") {
		t.Error("the start of the article is missing")
	}
	if !strings.Contains(got.Text, "extractor should keep in full") {
		t.Error("the article text was not extracted")
	}
}

type endlessText struct{}

func (endlessText) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'y'
	}
	return len(p), nil
}
