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
