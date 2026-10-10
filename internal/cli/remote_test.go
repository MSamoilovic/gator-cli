package cli

import "testing"

func TestNormalizeServerURL(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"https://gator.example.com", "https://gator.example.com", false},
		{"https://gator.example.com/", "https://gator.example.com", false},
		{"  https://gator.example.com  ", "https://gator.example.com", false},
		{"http://localhost:8080", "http://localhost:8080", false},
		{"http://127.0.0.1:8080/", "http://127.0.0.1:8080", false},
		{"http://[::1]:8080", "http://[::1]:8080", false},
		{"http://gator.example.com", "", true},
		{"http://192.168.1.10:8080", "", true},
		{"gator.example.com", "", true},
		{"ftp://gator.example.com", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := normalizeServerURL(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("normalizeServerURL(%q) error = %v, want error: %v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeServerURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
