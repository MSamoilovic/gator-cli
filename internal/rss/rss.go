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
	PubDate     string `xml:"pubDate"`
}

// Atom (RFC 4287). Imena elemenata se poklapaju po lokalnom delu, pa namespace
// prefiks ne treba navoditi.
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

// alternate vrati vezu koja vodi na sam tekst. Atom dozvoljava vise <link>
// elemenata, pa bi uzimanje prvog cesto dalo rel="self" — vezu na feed umesto
// na clanak.
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
			Title: e.Title,
			Link:  alternate(e.Link),
			// Summary pre Content-a, da Atom feedovi daju isto sto i RSS
			// description. Pun tekst je zasebna tema (ideas.md, tacka 1).
			Description: firstNonEmpty(e.Summary, e.Content),
			PubDate:     firstNonEmpty(e.Published, e.Updated),
		}
	}
	return r
}

// RSS 1.0 / RDF. Stavke su braca elementa <channel>, ne njegova deca, pa se ne
// mogu opisati istom strukturom kao RSS 2.0.
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
		Date        string `xml:"date"` // dc:date
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

func FetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gator")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %s: unexpected status %s", feedURL, res.Status)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", feedURL, err)
	}

	return parseFeed(data, feedURL)
}

// parseFeed prepozna format po korenskom elementu i sve svede na RSS 2.0
// oblik, pa pozivaoci (feeds.Add, feeds.Scrape) i dalje rade samo sa
// Channel/Item i ne znaju kojim je formatom feed napisan.
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
		return nil, fmt.Errorf("parsing %s: unsupported feed format, root element is <%s>", feedURL, root)
	}

	unescapeString(feed)
	return feed, nil
}

// rootElement vrati ime prvog elementa, ne citajuci ostatak dokumenta.
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

// charsetReader prima feedove koji nisu UTF-8. Bez njega encoding/xml po
// specifikaciji odbija svaki dokument koji deklarise drugo kodiranje, pa su
// stariji feedovi (Slashdot i dosta evropskih sajtova) padali na
// „encoding ISO-8859-1 declared but Decoder.CharsetReader is nil".
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
