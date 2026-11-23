package rss

import (
	"net/http"
	"strings"
	"testing"
)

func TestHTMLEntitiesInAFeedAreReadNotFatal(t *testing.T) {
	cases := map[string]string{
		"rss": `<?xml version="1.0"?><rss version="2.0"><channel><title>Caf&eacute;&nbsp;Notes</title><link>https://cafe.test</link>
<item><title>One&nbsp;&mdash; two&hellip;</title><link>https://cafe.test/1</link><description>a&nbsp;b</description></item>
</channel></rss>`,
		"atom": `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Caf&eacute;&nbsp;Notes</title>
<entry><title>One&nbsp;&mdash; two&hellip;</title><link rel="alternate" href="https://cafe.test/1"/><content>a&nbsp;b</content></entry>
</feed>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			feed, err := get(t, serve(t, http.StatusOK, body).URL)
			if err != nil {
				t.Fatalf("a feed with &nbsp; failed: %v", err)
			}
			if got, want := feed.Channel.Title, "Café Notes"; got != want {
				t.Errorf("channel title = %q, want %q", got, want)
			}
			if got, want := feed.Channel.Item[0].Title, "One — two…"; got != want {
				t.Errorf("item title = %q, want %q", got, want)
			}
			if got, want := feed.Channel.Item[0].Description, "a b"; got != want {
				t.Errorf("description = %q, want %q", got, want)
			}
		})
	}
}

func TestAnEntityNobodyDefinesIsStillAnError(t *testing.T) {
	body := `<?xml version="1.0"?><rss version="2.0"><channel><title>x &madeup; y</title></channel></rss>`
	if _, err := get(t, serve(t, http.StatusOK, body).URL); err == nil {
		t.Fatal("a feed with an undefined entity parsed; the decoder should still be strict")
	}
}

func TestItemLinksLoseTheWhitespaceAroundThem(t *testing.T) {
	body := "<?xml version=\"1.0\"?><rss version=\"2.0\"><channel><title>CBS</title><link>https://cbs.test</link>" +
		"<item><title>a</title><link>\n                    https://cbs.test/a\n\t</link></item></channel></rss>"
	feed, err := get(t, serve(t, http.StatusOK, body).URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := feed.Channel.Item[0].Link; got != "https://cbs.test/a" {
		t.Errorf("link = %q, want it without the surrounding whitespace", got)
	}
}

func TestRelativeItemLinksResolveAgainstTheSite(t *testing.T) {
	body := `<?xml version="1.0"?><rss version="2.0"><channel><title>Blog</title><link>https://site.test/blog/</link>
<item><title>root</title><link>/2026/post</link></item>
<item><title>sibling</title><link>post-2</link></item>
<item><title>absolute</title><link>https://elsewhere.test/x</link></item>
<item><title>none</title></item>
</channel></rss>`
	feed, err := get(t, serve(t, http.StatusOK, body).URL)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://site.test/2026/post", "https://site.test/blog/post-2", "https://elsewhere.test/x", ""}
	for i, w := range want {
		if got := feed.Channel.Item[i].Link; got != w {
			t.Errorf("item %d link = %q, want %q", i, got, w)
		}
	}
}

func TestRelativeLinksFallBackToTheFeedsOwnAddress(t *testing.T) {
	cases := map[string]string{
		"rss without a site link": `<?xml version="1.0"?><rss version="2.0"><channel><title>x</title><item><title>a</title><link>/a</link></item></channel></rss>`,
		"atom":                    `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>x</title><entry><title>a</title><link href="/a"/></entry></feed>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv := serve(t, http.StatusOK, body)
			feed, err := get(t, srv.URL+"/feeds/main.xml")
			if err != nil {
				t.Fatal(err)
			}
			if got, want := feed.Channel.Item[0].Link, srv.URL+"/a"; got != want {
				t.Errorf("link = %q, want %q", got, want)
			}
			if strings.HasPrefix(feed.Channel.Link, "/") {
				t.Errorf("channel link %q is still relative", feed.Channel.Link)
			}
		})
	}
}
