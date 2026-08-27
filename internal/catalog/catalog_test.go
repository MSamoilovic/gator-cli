package catalog

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func TestCatalogIsWellFormed(t *testing.T) {
	cats, err := Categories()
	if err != nil {
		t.Fatalf("Categories() = %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("catalog is empty")
	}

	seenID := make(map[string]bool)
	seenURL := make(map[string]string)

	for _, c := range cats {
		if !kebab.MatchString(c.ID) {
			t.Errorf("category id %q is not kebab-case", c.ID)
		}
		if seenID[c.ID] {
			t.Errorf("duplicate category id %q", c.ID)
		}
		seenID[c.ID] = true

		if strings.TrimSpace(c.Label) == "" {
			t.Errorf("category %q has no label", c.ID)
		}
		if len(c.Feeds) == 0 {
			t.Errorf("category %q has no feeds", c.ID)
		}

		for _, f := range c.Feeds {
			if strings.TrimSpace(f.Name) == "" {
				t.Errorf("category %q has a feed without a name", c.ID)
			}

			u, err := url.Parse(f.URL)
			if err != nil {
				t.Errorf("%s: %q does not parse: %v", f.Name, f.URL, err)
				continue
			}
			if u.Scheme != "https" || u.Host == "" {
				t.Errorf("%s: %q is not an absolute https URL", f.Name, f.URL)
			}

			if prev, dup := seenURL[f.URL]; dup {
				t.Errorf("%q appears in both %q and %q", f.URL, prev, c.ID)
			}
			seenURL[f.URL] = c.ID
		}
	}
}

func TestFindUnknownCategoryListsKnownOnes(t *testing.T) {
	_, err := Find("nepostojeca")
	if err == nil {
		t.Fatal("Find on an unknown id returned no error")
	}
	if !strings.Contains(err.Error(), "tech") {
		t.Errorf("error does not list known categories: %v", err)
	}
}

func TestResolveCollectsFeedsInOrder(t *testing.T) {
	tech, err := Find("tech")
	if err != nil {
		t.Fatal(err)
	}
	sport, err := Find("sport")
	if err != nil {
		t.Fatal(err)
	}

	got, err := Resolve([]string{"tech", "sport"})
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}
	if want := len(tech.Feeds) + len(sport.Feeds); len(got) != want {
		t.Fatalf("Resolve returned %d feeds, want %d", len(got), want)
	}
	if got[0].URL != tech.Feeds[0].URL {
		t.Errorf("first feed = %q, want %q", got[0].URL, tech.Feeds[0].URL)
	}
}

func TestResolveDeduplicatesRepeatedCategories(t *testing.T) {
	once, err := Resolve([]string{"tech"})
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Resolve([]string{"tech", "tech"})
	if err != nil {
		t.Fatal(err)
	}

	if len(once) != len(twice) {
		t.Errorf("Resolve(tech,tech) = %d feeds, want %d", len(twice), len(once))
	}
}

func TestResolveFailsOnUnknownCategory(t *testing.T) {
	if _, err := Resolve([]string{"tech", "nepostojeca"}); err == nil {
		t.Error("Resolve accepted an unknown category")
	}
}
