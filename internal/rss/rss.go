package rss

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/text/encoding/htmlindex"
)

type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`

	Content string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate string `xml:"pubDate"`
}

type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle"`
	Link     []atomLink  `xml:"link"`
	Entry    []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Link      []atomLink `xml:"link"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
}

func alternate(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "" || l.Rel == "alternate" {
			return l.Href
		}
	}
	return ""
}

func (a atomFeed) toRSS() *RSSFeed {
	r := &RSSFeed{}
	r.Channel.Title = a.Title
	r.Channel.Link = alternate(a.Link)
	r.Channel.Description = a.Subtitle

	r.Channel.Item = make([]RSSItem, len(a.Entry))
	for i, e := range a.Entry {
		r.Channel.Item[i] = RSSItem{
			Title:       e.Title,
			Link:        alternate(e.Link),
			Description: e.Summary,
			Content:     e.Content,
			PubDate:     firstNonEmpty(e.Published, e.Updated),
		}
	}
	return r
}

type rdfFeed struct {
	XMLName xml.Name `xml:"RDF"`
	Channel struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
	} `xml:"channel"`
	Item []struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
		Date        string `xml:"date"`
	} `xml:"item"`
}

func (f rdfFeed) toRSS() *RSSFeed {
	r := &RSSFeed{}
	r.Channel.Title = f.Channel.Title
	r.Channel.Link = f.Channel.Link
	r.Channel.Description = f.Channel.Description

	r.Channel.Item = make([]RSSItem, len(f.Item))
	for i, it := range f.Item {
		r.Channel.Item[i] = RSSItem{
			Title:       it.Title,
			Link:        it.Link,
			Description: it.Description,
			Content:     it.Content,
			PubDate:     it.Date,
		}
	}
	return r
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

var ErrNotModified = errors.New("feed not modified")

type Validators struct {
	ETag         string
	LastModified string
}

func FetchFeed(ctx context.Context, feedURL string, prev Validators) (*RSSFeed, Validators, error) {
	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, prev, err
	}
	req.Header.Set("User-Agent", "gator")
	if prev.ETag != "" {
		req.Header.Set("If-None-Match", prev.ETag)
	}
	if prev.LastModified != "" {
		req.Header.Set("If-Modified-Since", prev.LastModified)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, prev, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		return nil, prev, ErrNotModified
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, prev, fmt.Errorf("fetching %s: unexpected status %s", feedURL, res.Status)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, prev, fmt.Errorf("reading %s: %w", feedURL, err)
	}

	feed, err := parseFeed(data, finalURL(res, feedURL))
	if err != nil {
		return nil, prev, err
	}

	next := Validators{
		ETag:         res.Header.Get("ETag"),
		LastModified: res.Header.Get("Last-Modified"),
	}
	return feed, next, nil
}

func parseFeed(data []byte, feedURL string) (*RSSFeed, error) {
	root, err := rootElement(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", feedURL, err)
	}

	var feed *RSSFeed
	switch root {
	case "rss":
		var r RSSFeed
		if err := newDecoder(data).Decode(&r); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", feedURL, err)
		}
		feed = &r

	case "feed":
		var a atomFeed
		if err := newDecoder(data).Decode(&a); err != nil {
			return nil, fmt.Errorf("parsing %s as Atom: %w", feedURL, err)
		}
		feed = a.toRSS()

	case "RDF":
		var r rdfFeed
		if err := newDecoder(data).Decode(&r); err != nil {
			return nil, fmt.Errorf("parsing %s as RSS 1.0: %w", feedURL, err)
		}
		feed = r.toRSS()

	default:
		return nil, notAFeed(data, feedURL, root)
	}

	resolveBody(feed)
	unescapeString(feed)
	return feed, nil
}

func resolveBody(f *RSSFeed) {
	for i := range f.Channel.Item {
		it := &f.Channel.Item[i]
		it.Description = firstNonEmpty(it.Content, it.Description)
	}
}

func rootElement(data []byte) (string, error) {
	d := newDecoder(data)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return "", errors.New("no XML element found")
		}
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func newDecoder(data []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = charsetReader
	return d
}

func charsetReader(label string, in io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(label)
	if err != nil {
		return nil, fmt.Errorf("unsupported charset %q: %w", label, err)
	}
	return enc.NewDecoder().Reader(in), nil
}

func unescapeString(rss *RSSFeed) *RSSFeed {
	rss.Channel.Title = html.UnescapeString(rss.Channel.Title)
	rss.Channel.Description = html.UnescapeString(rss.Channel.Description)
	for i := range rss.Channel.Item {
		rss.Channel.Item[i].Title = html.UnescapeString(rss.Channel.Item[i].Title)
		rss.Channel.Item[i].Description = html.UnescapeString(rss.Channel.Item[i].Description)
	}
	return rss
}

func finalURL(res *http.Response, requested string) string {
	if res.Request != nil && res.Request.URL != nil {
		return res.Request.URL.String()
	}
	return requested
}

func notAFeed(data []byte, pageURL, root string) error {
	e := &NotAFeedError{URL: pageURL, Root: root}
	if !strings.EqualFold(root, "html") {
		return e
	}
	if base, err := url.Parse(pageURL); err == nil {
		e.Links = discoverLinks(data, base)
	}
	return e
}
