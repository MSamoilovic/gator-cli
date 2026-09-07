package article

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gator-cli/internal/text"

	readability "github.com/go-shiori/go-readability"
)

const (
	fetchTimeout = 20 * time.Second
	maxBody      = 8 << 20
	userAgent    = "gator"
)

type Article struct {
	Title  string
	Text   string
	HTML   string
	Byline string
	Site   string
}

func Fetch(ctx context.Context, pageURL string) (Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return Article{}, err
	}
	req.Header.Set("User-Agent", userAgent)

	client := http.Client{Timeout: fetchTimeout}
	res, err := client.Do(req)
	if err != nil {
		return Article{}, fmt.Errorf("fetching %s: %w", pageURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Article{}, fmt.Errorf("fetching %s: unexpected status %s", pageURL, res.Status)
	}

	return Extract(io.LimitReader(res.Body, maxBody), pageURL)
}

func Extract(r io.Reader, pageURL string) (Article, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return Article{}, fmt.Errorf("parsing %s: %w", pageURL, err)
	}

	parsed, err := readability.FromReader(r, base)
	if err != nil {
		return Article{}, fmt.Errorf("reading %s: %w", pageURL, err)
	}

	body := normalise(text.StripHTML(parsed.Content))
	if body == "" {
		body = normalise(parsed.TextContent)
	}
	if body == "" {
		return Article{}, fmt.Errorf("no article text found at %s", pageURL)
	}

	return Article{
		Title:  strings.TrimSpace(parsed.Title),
		Text:   body,
		HTML:   parsed.Content,
		Byline: strings.TrimSpace(parsed.Byline),
		Site:   strings.TrimSpace(parsed.SiteName),
	}, nil
}

func Improves(got, had string) bool {
	return len([]rune(strings.TrimSpace(got))) > len([]rune(strings.TrimSpace(had)))
}

func normalise(s string) string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if p := strings.Join(strings.Fields(para), " "); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}
