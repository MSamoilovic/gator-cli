package opml

import (
	"bytes"
	"strings"
	"testing"
)

const feedlyLike = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
  <head><title>marko subscriptions</title></head>
  <body>
    <outline text="Tech" title="Tech">
      <outline type="rss" text="Ars Technica" title="Ars Technica"
               xmlUrl="https://arstechnica.com/feed/" htmlUrl="https://arstechnica.com"/>
      <outline type="rss" text="Lobsters" xmlUrl="https://lobste.rs/rss"/>
    </outline>
    <outline text="Sport" title="Sport">
      <outline type="rss" text="BBC Sport" xmlUrl="https://feeds.bbci.co.uk/sport/rss.xml"/>
    </outline>
    <outline type="rss" text="Bez foldera" xmlUrl="https://example.com/root.xml"/>
  </body>
</opml>`

func TestParseNestedOutlines(t *testing.T) {
	got, err := Parse(strings.NewReader(feedlyLike))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d feeds, want 4: %+v", len(got), got)
	}

	first := got[0]
	if first.Title != "Ars Technica" || first.XMLURL != "https://arstechnica.com/feed/" {
		t.Errorf("first feed = %+v", first)
	}
	if first.Category != "Tech" {
		t.Errorf("category = %q, want %q", first.Category, "Tech")
	}
	if first.HTMLURL != "https://arstechnica.com" {
		t.Errorf("htmlUrl = %q", first.HTMLURL)
	}

	if got[1].Title != "Lobsters" {
		t.Errorf("feed without title attr = %q, want it to fall back to text", got[1].Title)
	}
	if got[3].Category != "" {
		t.Errorf("root feed category = %q, want empty", got[3].Category)
	}
}

func TestParseSkipsFolders(t *testing.T) {
	got, err := Parse(strings.NewReader(feedlyLike))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.XMLURL == "" {
			t.Errorf("folder leaked in as a feed: %+v", f)
		}
	}
}

func TestParseDeduplicatesByURL(t *testing.T) {
	doc := `<opml version="2.0"><body>
	  <outline text="A"><outline type="rss" text="Isti" xmlUrl="https://x/feed"/></outline>
	  <outline text="B"><outline type="rss" text="Isti opet" xmlUrl="https://x/feed"/></outline>
	</body></opml>`

	got, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d feeds, want 1 — the same xmlUrl in two folders is one feed", len(got))
	}
	if got[0].Category != "A" {
		t.Errorf("category = %q, want the first occurrence to win", got[0].Category)
	}
}

func TestParseDeeplyNested(t *testing.T) {
	doc := `<opml version="2.0"><body>
	  <outline text="Spolja">
	    <outline text="Unutra">
	      <outline type="rss" text="Duboko" xmlUrl="https://x/deep"/>
	    </outline>
	  </outline>
	</body></opml>`

	got, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d feeds, want 1", len(got))
	}
	if got[0].Category != "Unutra" {
		t.Errorf("category = %q, want the nearest named folder", got[0].Category)
	}
}

func TestParseEmptyBodyIsNotAnError(t *testing.T) {
	got, err := Parse(strings.NewReader(`<opml version="2.0"><head/><body/></opml>`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d feeds from an empty body", len(got))
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse(strings.NewReader("ovo nije XML")); err == nil {
		t.Fatal("expected an error for a non-XML document")
	}
}

func TestWriteEscapesAndRoundTrips(t *testing.T) {
	in := []Feed{
		{Title: `Ars & "Friends" <3`, XMLURL: "https://arstechnica.com/feed/", HTMLURL: "https://arstechnica.com"},
		{Title: "Lobsters", XMLURL: "https://lobste.rs/rss"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, "gator subscriptions", in); err != nil {
		t.Fatalf("Write: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "<?xml") {
		t.Error("output has no XML declaration")
	}
	if strings.Contains(out, `"Friends"`) || strings.Contains(out, "Ars & ") {
		t.Errorf("title was not escaped:\n%s", out)
	}

	back, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("re-parsing our own output: %v", err)
	}
	if len(back) != len(in) {
		t.Fatalf("round trip returned %d feeds, want %d", len(back), len(in))
	}
	for i := range in {
		if back[i].Title != in[i].Title || back[i].XMLURL != in[i].XMLURL {
			t.Errorf("round trip %d: got %+v, want %+v", i, back[i], in[i])
		}
	}
}

func TestWriteEmptyList(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "prazno", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Parse(&buf)
	if err != nil {
		t.Fatalf("empty export does not parse back: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d feeds from an empty export", len(got))
	}
}

func TestWriteGroupsByCategoryAndRoundTrips(t *testing.T) {
	in := []Feed{
		{Title: "Lobsters", XMLURL: "https://lobste.rs/rss", Category: "Tech"},
		{Title: "Bez foldera", XMLURL: "https://example.com/a.xml"},
		{Title: "Ars Technica", XMLURL: "https://arstechnica.com/feed/", Category: "Tech"},
		{Title: "BBC Sport", XMLURL: "https://feeds.bbci.co.uk/sport/rss.xml", Category: "Sport"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, "test", in); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("round trip returned %d feeds, want %d", len(got), len(in))
	}

	byURL := make(map[string]Feed, len(got))
	for _, f := range got {
		byURL[f.XMLURL] = f
	}
	for _, want := range in {
		f, ok := byURL[want.XMLURL]
		if !ok {
			t.Errorf("%s did not survive the round trip", want.XMLURL)
			continue
		}
		if f.Category != want.Category {
			t.Errorf("%s category = %q, want %q", want.Title, f.Category, want.Category)
		}
		if f.Title != want.Title {
			t.Errorf("title = %q, want %q", f.Title, want.Title)
		}
	}
}

func TestWriteSortsFoldersAndPutsRootLast(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "test", []Feed{
		{Title: "R", XMLURL: "https://r", Category: "Sport"},
		{Title: "K", XMLURL: "https://k"},
		{Title: "A", XMLURL: "https://a", Category: "Comics"},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	out := buf.String()
	comics, sport, root := strings.Index(out, `"Comics"`), strings.Index(out, `"Sport"`), strings.Index(out, "https://k")
	if comics < 0 || sport < 0 || root < 0 {
		t.Fatalf("missing entries in output:\n%s", out)
	}
	if !(comics < sport) {
		t.Error("folders are not in alphabetical order")
	}
	if !(sport < root) {
		t.Error("uncategorized feed is not last")
	}
}
