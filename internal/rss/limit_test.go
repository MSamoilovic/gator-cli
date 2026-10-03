package rss

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

func TestReadBodyAcceptsExactlyTheLimit(t *testing.T) {
	data, err := readBody(strings.NewReader(strings.Repeat("a", maxBody)))
	if err != nil {
		t.Fatalf("a body of exactly maxBody was rejected: %v", err)
	}
	if len(data) != maxBody {
		t.Errorf("read %d bytes, want %d", len(data), maxBody)
	}
}

func TestReadBodyRefusesOneByteOver(t *testing.T) {
	_, err := readBody(strings.NewReader(strings.Repeat("a", maxBody+1)))
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("readBody = %v, want ErrBodyTooLarge", err)
	}
}

func TestReadBodyNeverTruncatesSilently(t *testing.T) {
	data, err := readBody(strings.NewReader(strings.Repeat("a", maxBody*2)))
	if err == nil {
		t.Fatalf("an oversized body was accepted, truncated to %d bytes", len(data))
	}
	if data != nil {
		t.Errorf("readBody returned %d bytes alongside an error", len(data))
	}
}

func TestFetchFeedRefusesAnOversizedFeed(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		io.Copy(w, io.LimitReader(endlessZeros{}, maxBody*2))
	})

	_, err := fetch(t, srv.URL)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("FetchFeed = %v, want ErrBodyTooLarge", err)
	}
	if !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error %q does not name the feed", err)
	}
}

func TestFetchFeedStopsReadingAnEndlessBody(t *testing.T) {
	var served atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		buf := make([]byte, 32<<10)
		for {
			n, err := w.Write(buf)
			served.Add(int64(n))
			if err != nil {
				return
			}
			if served.Load() > maxBody*8 {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	if _, err := fetch(t, srv.URL); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("FetchFeed = %v, want ErrBodyTooLarge", err)
	}

	if got := served.Load(); got > maxBody*2 {
		t.Errorf("server wrote %d bytes for a %d byte limit — the read is not bounded", got, maxBody)
	}
}

func TestFetchFeedStillAcceptsALargeButLegitimateFeed(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Big</title>`)
	for i := 0; sb.Len() < 2<<20; i++ {
		fmt.Fprintf(&sb, `<item><title>Post %d</title><link>https://e.example/%d</link></item>`, i, i)
	}
	sb.WriteString(`</channel></rss>`)

	srv := serve(t, http.StatusOK, sb.String())

	feed, err := fetch(t, srv.URL)
	if err != nil {
		t.Fatalf("a %d byte feed was rejected: %v", sb.Len(), err)
	}
	if len(feed.Channel.Item) == 0 {
		t.Error("no items parsed from the large feed")
	}
}

type endlessZeros struct{}

func (endlessZeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '0'
	}
	return len(p), nil
}
