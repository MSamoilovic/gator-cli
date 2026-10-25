package invite

import "testing"

func TestLinkAndParseAreEachOthersInverse(t *testing.T) {
	cases := []struct {
		server string
		token  string
	}{
		{"https://gator.example.test", "gatinv_abc"},
		{"https://gator.example.test/", "gatinv_abc"},
		{"http://127.0.0.1:8099", "gatinv_abc"},
	}

	for _, c := range cases {
		link := Link(c.server, c.token)
		server, token, err := Parse(link)
		if err != nil {
			t.Fatalf("Parse(%q): %v", link, err)
		}
		if token != c.token {
			t.Errorf("Parse(%q) token = %q, want %q", link, token, c.token)
		}
		if want := Link(server, token); want != link {
			t.Errorf("Link(Parse(%q)) = %q", link, want)
		}
	}
}

func TestParseRefusesWhatIsNotALink(t *testing.T) {
	cases := map[string]string{
		"empty":                   "",
		"a bare code":             "gatinv_abc",
		"no code":                 "https://gator.example.test/join/",
		"another path":            "https://gator.example.test/joinx/gatinv_abc",
		"the code at the root":    "https://gator.example.test/gatinv_abc",
		"no host":                 "https:///join/gatinv_abc",
		"no scheme":               "gator.example.test/join/gatinv_abc",
		"a scheme we never serve": "ftp://gator.example.test/join/gatinv_abc",
		"a deeper path":           "https://gator.example.test/join/gatinv_abc/more",
	}

	for name, raw := range cases {
		if server, token, err := Parse(raw); err == nil {
			t.Errorf("%s: Parse(%q) = %q, %q, want an error", name, raw, server, token)
		}
	}
}

func TestParseTrimsWhatAChatClientAdds(t *testing.T) {
	server, token, err := Parse("  https://gator.example.test/join/gatinv_abc\n")
	if err != nil {
		t.Fatalf("a pasted link was refused: %v", err)
	}
	if server != "https://gator.example.test" || token != "gatinv_abc" {
		t.Errorf("Parse = %q, %q", server, token)
	}
}
