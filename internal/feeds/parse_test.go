package feeds

import (
	"testing"
	"time"
)

func TestParsePubDate(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
		want  time.Time
	}{
		{
			name:  "RFC1123Z",
			input: "Mon, 02 Jan 2006 15:04:05 -0700",
			valid: true,
			want:  time.Date(2006, time.January, 2, 15, 4, 5, 0, time.FixedZone("", -7*3600)),
		},
		{
			name:  "RFC3339",
			input: "2006-01-02T15:04:05Z",
			valid: true,
			want:  time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC),
		},
		{
			name:  "no weekday with offset",
			input: "02 Jan 2006 15:04:05 -0700",
			valid: true,
			want:  time.Date(2006, time.January, 2, 15, 4, 5, 0, time.FixedZone("", -7*3600)),
		},
		{
			name:  "date only",
			input: "2026-08-13",
			valid: true,
			want:  time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "empty string",
			input: "",
			valid: false,
		},
		{
			name:  "garbage",
			input: "not a date",
			valid: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParsePubDate(tc.input)
			if got.Valid != tc.valid {
				t.Fatalf("Valid = %v, want %v", got.Valid, tc.valid)
			}
			if tc.valid && !got.Time.Equal(tc.want) {
				t.Errorf("Time = %v, want %v", got.Time, tc.want)
			}
		})
	}
}

func TestParsePubDateIgnoresSurroundingWhitespace(t *testing.T) {
	const want = "2026-10-14T06:52:45Z"
	for _, raw := range []string{
		"\n\t\t\t2026-10-14T06:52:45+00:00\t\t",
		"  Mon, 02 Jan 2006 15:04:05 -0700  ",
		"\r\n2026-10-14\r\n",
	} {
		got := ParsePubDate(raw)
		if !got.Valid {
			t.Errorf("ParsePubDate(%q) is not valid — pretty-printed XML indents the value", raw)
		}
	}

	got := ParsePubDate("\n\t\t\t2026-10-14T06:52:45+00:00\t\t")
	if !got.Valid || got.Time.UTC().Format(time.RFC3339) != want {
		t.Errorf("ParsePubDate = %v, want %s", got, want)
	}
}

func TestParsePubDateAcceptsFormatsRealFeedsSend(t *testing.T) {
	cases := map[string]string{
		"MDN Blog, single-digit day":     "Wed, 8 Apr 2026 00:00:00 +0000",
		"MDN Blog, two-digit day":        "Mon, 15 Jun 2026 00:00:00 +0000",
		"BBC Good Food, GMT":             "Tue, 13 Oct 2026 21:14:00 GMT",
		"ESPN, US zone abbreviation":     "Wed, 14 Oct 2026 01:56:38 EST",
		"A List Apart, indented RFC3339": "\n\t\t\t2026-10-14T06:52:45+00:00\t\t",
		"Golang Weekly, channel level":   "Fri, 9 Oct 2026 00:00:00 +0000",
	}
	for name, raw := range cases {
		if got := ParsePubDate(raw); !got.Valid {
			t.Errorf("%s: ParsePubDate(%q) did not parse", name, raw)
		}
	}
}
