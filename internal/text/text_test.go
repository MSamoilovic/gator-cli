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
