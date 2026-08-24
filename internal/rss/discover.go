package rss

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Link je feed koji stranica sama oglasava preko <link rel="alternate">.
type Link struct {
	URL   string
	Title string
	Type  string
}

// NotAFeedError kaze da odgovor nije feed. Kad je u pitanju HTML stranica,
// nosi i feedove koje ona oglasava — nalaze se u telu koje je vec povuceno, pa
// pozivalac ne mora da je povlaci po drugi put.
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

// feedTypes su MIME tipovi pod kojima se feed oglasava. application/xml i
// text/xml su tu jer ih stariji sajtovi koriste umesto preciznijih.
var feedTypes = map[string]bool{
	"application/rss+xml":  true,
	"application/atom+xml": true,
	"application/rdf+xml":  true,
	"application/xml":      true,
	"text/xml":             true,
}

// discoverLinks vadi feedove koje HTML stranica oglasava, redom kojim ih je
// navela. Redosled se cuva jer je znacajan: sajtovi glavni feed stavljaju
// prvi, a sporedne (komentari, kategorije) iza njega.
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
				// <link> pripada zaglavlju; dalje se samo troši vreme.
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
	// Tip je jedini pouzdan znak da je rel="alternate" feed a ne prevod
	// stranice ili verzija za stampu.
	typ = strings.ToLower(strings.TrimSpace(strings.Split(typ, ";")[0]))
	if !feedTypes[typ] {
		return Link{}, false
	}

	// Relativne adrese se razresavaju u odnosu na stranicu, i to onu na kojoj
	// smo zavrsili posle preusmerenja.
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
