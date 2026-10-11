package catalog

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/rss"
)

type fetched struct {
	category string
	name     string
	url      string
	items    int
	links    map[string]bool
	err      error
}

func fetchWholeCatalog(t *testing.T) []fetched {
	t.Helper()

	cats, err := Categories()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var (
		mu  sync.Mutex
		out []fetched
		wg  sync.WaitGroup
		sem = make(chan struct{}, 12)
	)

	for _, c := range cats {
		for _, f := range c.Feeds {
			wg.Add(1)
			go func(category string, f Feed) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				feed, _, err := rss.FetchFeed(ctx, rss.Source{URL: f.URL})
				got := fetched{category: category, name: f.Name, url: f.URL, err: err}
				if err == nil {
					got.items = len(feed.Channel.Item)
					got.links = make(map[string]bool, got.items)
					for _, it := range feed.Channel.Item {
						got.links[it.Link] = true
					}
				}

				mu.Lock()
				out = append(out, got)
				mu.Unlock()
			}(c.ID, f)
		}
	}
	wg.Wait()

	return out
}

func TestEveryCatalogFeedStillResolves(t *testing.T) {
	if os.Getenv("GATOR_CATALOG_CHECK") == "" {
		t.Skip("set GATOR_CATALOG_CHECK=1 to fetch every catalog feed over the network")
	}

	results := fetchWholeCatalog(t)

	broken, empty := 0, 0
	for _, r := range results {
		switch {
		case r.err != nil:
			broken++
			t.Errorf("%s / %s (%s): %v", r.category, r.name, r.url, r.err)
		case r.items == 0:
			empty++
			t.Errorf("%s / %s (%s): fetched but served no items", r.category, r.name, r.url)
		}
	}
	t.Logf("%d feeds checked, %d unreachable, %d empty", len(results), broken, empty)
}

func TestNoCategoryServesTheSameItemsTwice(t *testing.T) {
	if os.Getenv("GATOR_CATALOG_CHECK") == "" {
		t.Skip("set GATOR_CATALOG_CHECK=1 to fetch every catalog feed over the network")
	}

	byCategory := make(map[string][]fetched)
	for _, r := range fetchWholeCatalog(t) {
		if r.err == nil && r.items > 0 {
			byCategory[r.category] = append(byCategory[r.category], r)
		}
	}

	const tooMuch = 0.6
	for category, feeds := range byCategory {
		for i := range feeds {
			for j := i + 1; j < len(feeds); j++ {
				if share := overlap(feeds[i].links, feeds[j].links); share > tooMuch {
					t.Errorf("%s: %q and %q share %.0f%% of their items — one of them should go",
						category, feeds[i].name, feeds[j].name, share*100)
				}
			}
		}
	}
}

func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(b) < len(a) {
		a, b = b, a
	}

	shared := 0
	for link := range a {
		if b[link] {
			shared++
		}
	}
	return float64(shared) / float64(len(a))
}
