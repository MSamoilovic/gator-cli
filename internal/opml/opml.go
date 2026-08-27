package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type Feed struct {
	Title    string
	XMLURL   string
	HTMLURL  string
	Category string
}

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

func Write(w io.Writer, title string, feeds []Feed) error {
	var doc opmlDoc
	doc.Version = "2.0"
	doc.Head.Title = title
	doc.Head.DateCreated = time.Now().UTC().Format(time.RFC1123Z)

	grouped := make(map[string][]outline)
	var rooted []outline

	for _, f := range feeds {
		o := outline{
			Text:    f.Title,
			Title:   f.Title,
			Type:    "rss",
			XMLURL:  f.XMLURL,
			HTMLURL: f.HTMLURL,
		}
		if c := strings.TrimSpace(f.Category); c != "" {
			grouped[c] = append(grouped[c], o)
			continue
		}
		rooted = append(rooted, o)
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		doc.Body.Outlines = append(doc.Body.Outlines, outline{
			Text:     name,
			Title:    name,
			Children: grouped[name],
		})
	}
	doc.Body.Outlines = append(doc.Body.Outlines, rooted...)

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
