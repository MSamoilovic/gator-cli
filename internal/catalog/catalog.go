// Package catalog nosi kuriranu listu feedova po kategorijama, ugradjenu u
// binary. Zahvaljujuci njoj novi korisnik ne mora unapred da zna nijedan RSS
// URL da bi poceo da cita.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed catalog.json
var raw []byte

type Feed struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Feeds []Feed `json:"feeds"`
}

var Categories = sync.OnceValues(func() ([]Category, error) {
	var doc struct {
		Categories []Category `json:"categories"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing embedded catalog: %w", err)
	}
	return doc.Categories, nil
})

func Find(id string) (Category, error) {
	cats, err := Categories()
	if err != nil {
		return Category{}, err
	}

	known := make([]string, 0, len(cats))
	for _, c := range cats {
		if c.ID == id {
			return c, nil
		}
		known = append(known, c.ID)
	}
	return Category{}, fmt.Errorf("unknown category %q (have: %s)", id, strings.Join(known, ", "))
}

func Resolve(ids []string) ([]Feed, error) {
	var (
		out  []Feed
		seen = make(map[string]bool)
	)
	for _, id := range ids {
		c, err := Find(id)
		if err != nil {
			return nil, err
		}
		for _, f := range c.Feeds {
			if seen[f.URL] {
				continue
			}
			seen[f.URL] = true
			out = append(out, f)
		}
	}
	return out, nil
}
