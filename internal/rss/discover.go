package rss

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type Link struct {
	URL   string
	Title string
	Type  string
}

type NotAFeedError struct {
	URL   string
	Root  string
	Links []Link
}

func (e *NotAFeedError) Error() string {
	if len(e.Links) > 0 {
		return fmt.Sprintf("%s is a web page, not a feed; it advertises %s", e.URL, e.Links[0].URL)
	}
	return fmt.Sprintf("parsing %s: unsupported feed format, root element is <%s>", e.URL, e.Root)
}

var feedTypes = map[string]bool{
	"application/rss+xml":  true,
	"application/atom+xml": true,
	"application/rdf+xml":  true,
	"application/xml":      true,
	"text/xml":             true,
}

func discoverLinks(data []byte, base *url.URL) []Link {
	var (
		links []Link
		seen  = make(map[string]bool)
	)

	z := html.NewTokenizer(bytes.NewReader(data))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return links

		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "link":
				if !hasAttr {
					continue
				}
				if l, ok := feedLink(z, base); ok && !seen[l.URL] {
					seen[l.URL] = true
					links = append(links, l)
				}
			case "body":
				return links
			}
		}
	}
}

func feedLink(z *html.Tokenizer, base *url.URL) (Link, bool) {
	var rel, typ, href, title string
	for {
		key, val, more := z.TagAttr()
		switch string(key) {
		case "rel":
			rel = string(val)
		case "type":
			typ = string(val)
		case "href":
			href = string(val)
		case "title":
			title = string(val)
		}
		if !more {
			break
		}
	}

	if !strings.EqualFold(rel, "alternate") || href == "" {
		return Link{}, false
	}
	typ = strings.ToLower(strings.TrimSpace(strings.Split(typ, ";")[0]))
	if !feedTypes[typ] {
		return Link{}, false
	}

	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return Link{}, false
	}
	return Link{
		URL:   base.ResolveReference(ref).String(),
		Title: strings.TrimSpace(title),
		Type:  typ,
	}, true
}
