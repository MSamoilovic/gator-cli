package text

import "testing"

func TestStripHTML(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "paragraphs become blank lines",
			input: `<p>Article URL: <a href="http://x">http://x</a></p><p>Points: 42</p>`,
			want:  "Article URL: http://x\n\nPoints: 42",
		},
		{
			name:  "entities are decoded",
			input: "Go &amp; Rust &lt;3 &quot;fast&quot;",
			want:  `Go & Rust <3 "fast"`,
		},
		{
			name:  "br becomes newline",
			input: "prvi<br/>drugi",
			want:  "prvi\ndrugi",
		},
		{
			name:  "attributes on block tags are handled",
			input: `<div class="a b"><p style="x">tekst</p></div>`,
			want:  "tekst",
		},
		{
			name:  "inline tags are dropped without spacing damage",
			input: "<b>bold</b><i>italic</i>",
			want:  "bolditalic",
		},
		{
			name:  "plain text passes through",
			input: "nema tagova",
			want:  "nema tagova",
		},
		{
			name:  "only markup collapses to empty",
			input: "<p></p>",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripHTML(tc.input); got != tc.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{
			name:  "shorter than the limit is untouched",
			input: "kratak tekst",
			max:   400,
			want:  "kratak tekst",
		},
		{
			name:  "exactly the limit is untouched",
			input: "12345",
			max:   5,
			want:  "12345",
		},
		{
			name:  "cut falls back to a word boundary",
			input: "jedan dva tri cetiri pet",
			max:   12,
			want:  "jedan dva…",
		},
		{
			name:  "a single word longer than the limit is cut hard",
			input: "aaaaaaaaaaaaaaaaaaaa",
			max:   5,
			want:  "aaaaa…",
		},
		{
			name:  "multibyte text is not broken",
			input: "Ćirilica је ту",
			max:   9,
			want:  "Ćirilica…",
		},
		{
			name:  "zero limit gives nothing",
			input: "nesto",
			max:   0,
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Truncate(tc.input, tc.max); got != tc.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
		})
	}
}

func TestOneLineCollapsesWhitespace(t *testing.T) {
	cases := map[string]string{
		"\n   Learn Data Structures\n   Visually  \n": "Learn Data Structures Visually",
		"  already clean  ":                           "already clean",
		"":                                            "",
		"\t\n ":                                       "",
	}
	for in, want := range cases {
		if got := OneLine(in); got != want {
			t.Errorf("OneLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintableDropsWhatATerminalWouldObey(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"escape sequence", "red \x1b[31malert\x1b[0m", "red [31malert[0m"},
		{"eight-bit CSI", "a\u009b2Jb", "a2Jb"},
		{"OSC 52 clipboard write", "x\x1b]52;c;cm0gLXJmIH4=\x07y", "x]52;c;cm0gLXJmIH4=y"},
		{"bell", "ding\x07", "ding"},
		{"delete", "a\x7fb", "ab"},
		{"carriage return", "fake\rreal", "fakereal"},
		{"bidi override", "evil‮gnp.exe", "evilgnp.exe"},
		{"bidi isolate", "a⁦b⁩c", "abc"},
		{"newline and tab stay", "line one\n\tline two", "line one\n\tline two"},
		{"ordinary unicode stays", "Ćevapi — ★ ● 日本", "Ćevapi — ★ ● 日本"},
		{"invalid UTF-8 cannot pass as C1", "a\x9bb", "a�b"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Printable(c.input); got != c.want {
				t.Errorf("Printable(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestTheHelpersNeverReturnAnEscape(t *testing.T) {
	evil := "a&#27;]52;c;eA==&#7;b\x1b[2J\u009b"
	for name, got := range map[string]string{
		"StripHTML": StripHTML("<p>" + evil + "</p>"),
		"OneLine":   OneLine(evil),
		"Truncate":  Truncate(evil, 400),
	} {
		for _, r := range got {
			if hidden(r) {
				t.Errorf("%s(%q) = %q, still carries %U", name, evil, got, r)
			}
		}
	}
}
