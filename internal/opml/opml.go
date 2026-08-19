// Package opml cita i pise OPML — format kojim citaci razmenjuju liste
// pretplata. Nosi samo feedove, ne i procitano, bookmark-e ni postove.
package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Feed je jedan unos iz OPML-a. Category je ime foldera u kome je stajao;
// prazno znaci da je bio u korenu.
type Feed struct {
	Title    string
	XMLURL   string
	HTMLURL  string
	Category string
}

// outline je i folder i feed — razlikuju se po tome da li imaju xmlUrl.
// Citaci ih ugnjezduju proizvoljno duboko, pa outline sadrzi sam sebe.
type outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr,omitempty"`
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Children []outline `xml:"outline,omitempty"`
}

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    struct {
		Title       string `xml:"title,omitempty"`
		DateCreated string `xml:"dateCreated,omitempty"`
	} `xml:"head"`
	Body struct {
		Outlines []outline `xml:"outline"`
	} `xml:"body"`
}

// Parse izvuce sve feedove iz OPML-a, ma koliko duboko bili ugnjezdeni.
// Isti xmlUrl u dva foldera se vraca jednom — prvi nalaz pobedjuje.
func Parse(r io.Reader) ([]Feed, error) {
	var doc opmlDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsing OPML: %w", err)
	}

	var (
		out  []Feed
		seen = make(map[string]bool)
	)
	var walk func(items []outline, category string)
	walk = func(items []outline, category string) {
		for _, o := range items {
			label := firstNonEmpty(o.Title, o.Text)

			// Outline bez xmlUrl je folder; njegovo ime postaje kategorija za
			// sve ispod, ali samo na prvom nivou — dublja gnezda zadrzavaju
			// najblizeg imenovanog pretka.
			if url := strings.TrimSpace(o.XMLURL); url != "" {
				if !seen[url] {
					seen[url] = true
					out = append(out, Feed{
						Title:    label,
						XMLURL:   url,
						HTMLURL:  o.HTMLURL,
						Category: category,
					})
				}
			}

			if len(o.Children) > 0 {
				next := category
				if o.XMLURL == "" && label != "" {
					next = label
				}
				walk(o.Children, next)
			}
		}
	}
	walk(doc.Body.Outlines, "")

	return out, nil
}

// Write ispise feedove kao OPML 2.0. Lista je ravna: gator ne pamti kategorije
// feedova, a izmisljanje foldera iz kataloga bi lagalo o rucno dodatim.
func Write(w io.Writer, title string, feeds []Feed) error {
	var doc opmlDoc
	doc.Version = "2.0"
	doc.Head.Title = title
	doc.Head.DateCreated = time.Now().UTC().Format(time.RFC1123Z)

	doc.Body.Outlines = make([]outline, len(feeds))
	for i, f := range feeds {
		doc.Body.Outlines[i] = outline{
			Text:    f.Title,
			Title:   f.Title,
			Type:    "rss",
			XMLURL:  f.XMLURL,
			HTMLURL: f.HTMLURL,
		}
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("writing OPML: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
