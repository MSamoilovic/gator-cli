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
	Title string `xml:"title"`
	Link  string `xml:"link"`
	// Description je posle parsiranja uvek najduzi tekst koji feed nudi:
	// Content ako postoji, inace <description>. Vidi resolveBody.
	Description string `xml:"description"`
	// Content je <content:encoded>, gde vecina feedova salje ceo clanak dok u
	// <description> stoji samo izvod. Namespace se navodi jer je <encoded>
	// suvise obicno ime da bi se hvatalo po lokalnom delu.
	Content string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate string `xml:"pubDate"`
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
			Title:       e.Title,
			Link:        alternate(e.Link),
			Description: e.Summary,
			Content:     e.Content,
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
		Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
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

// ErrNotModified znaci da je server odgovorio 304: feed se nije promenio od
// proslog preuzimanja. Nije greska u radu nego ocekivan ishod, po ugledu na
// io.EOF — pozivalac ga hvata sa errors.Is.
var ErrNotModified = errors.New("feed not modified")

// Validators su otisci verzije koje server salje uz feed. Cuvaju se i vracaju
// doslovno: ETag je neprozirna niska, a Last-Modified bi se preformatiranjem
// razisao sa onim sto server ocekuje.
type Validators struct {
	ETag         string
	LastModified string
}

// FetchFeed preuzme i isparsira feed. Ako prev nije prazan, zahtev nosi uslovna
// zaglavlja i server sme da odgovori 304 — tada se vraca ErrNotModified, feed je
// nil, a prev se vraca nepromenjen da pozivalac ne bi obrisao ono sto ima.
//
// Trecina feedova ovo ne podrzava, a neki (Ars Technica) salju ETag pa ga
// ignorisu, zato je 304 precica a ne pretpostavka: pun put mora da radi uvek.
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

	// Parsira se u odnosu na adresu na kojoj smo zavrsili, ne na onu koja je
	// zatrazena: ako je bilo preusmerenja, relativne veze u stranici vaze
	// prema krajnjoj.
	feed, err := parseFeed(data, finalURL(res, feedURL))
	if err != nil {
		return nil, prev, err
	}

	// Novi otisci vaze samo uz telo koje je upravo isparsirano; ako ih server ne
	// salje, ostaje prazno i sledeci zahtev ide bezuslovno.
	next := Validators{
		ETag:         res.Header.Get("ETag"),
		LastModified: res.Header.Get("Last-Modified"),
	}
	return feed, next, nil
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
		return nil, notAFeed(data, feedURL, root)
	}

	resolveBody(feed)
	unescapeString(feed)
	return feed, nil
}

// resolveBody bira najduzi tekst koji feed nudi za svaku stavku. Vecina
// feedova u <description> salje samo izvod, a ceo clanak u <content:encoded>
// (Atom: <content>), pa detalj panel bez ovoga prikazuje dva pasusa umesto
// teksta. Pravilo stoji ovde, a ne u feeds.Scrape, da bi sva tri formata
// prolazila kroz isto.
func resolveBody(f *RSSFeed) {
	for i := range f.Channel.Item {
		it := &f.Channel.Item[i]
		it.Description = firstNonEmpty(it.Content, it.Description)
	}
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

// finalURL je adresa na kojoj je zahtev zavrsio, posle svih preusmerenja.
func finalURL(res *http.Response, requested string) string {
	if res.Request != nil && res.Request.URL != nil {
		return res.Request.URL.String()
	}
	return requested
}

// notAFeed pravi gresku za odgovor koji nije feed. Ako je HTML, usput
// pokupi feedove koje stranica oglasava — telo je vec tu, pa je to besplatno.
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
