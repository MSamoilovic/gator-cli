package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/storetest"
)

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

func TestSignInErrorTellsYouWhatToRun(t *testing.T) {
	const server = "https://gator.example.test"

	tests := []struct {
		name   string
		server string
		err    error
		want   string
		plain  bool
	}{
		{
			name:   "istekao token imenuje komandu",
			server: server,
			err:    fmt.Errorf("fetching me: %w", store.ErrUnauthorized),
			want:   "gator login --server " + server,
		},
		{
			name:   "nedostupan server nudi izlaz",
			server: server,
			err:    fmt.Errorf("reaching %s: %w: %w", server, store.ErrUnreachable, errors.New("dial tcp: refused")),
			want:   "gator logout",
		},
		{
			name:   "lokalni rezim ostaje na staroj poruci",
			server: "",
			err:    errors.New("no user named marko"),
			want:   "user not logged in",
			plain:  true,
		},
		{
			name:   "nepoznata greska protiv servera ostaje citljiva",
			server: server,
			err:    errors.New("decoding response: unexpected EOF"),
			want:   "unexpected EOF",
			plain:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &state{Cfg: &config.Config{ServerURL: tt.server}}

			got := signInError(s, tt.err).Error()
			if !strings.Contains(got, tt.want) {
				t.Errorf("signInError() = %q, want it to contain %q", got, tt.want)
			}
			if !tt.plain && strings.Contains(got, "dial tcp") {
				t.Errorf("signInError() leaked the transport error: %q", got)
			}
		})
	}
}

func TestSignInErrorSurvivesAMissingConfig(t *testing.T) {
	if got := signInError(&state{}, store.ErrUnauthorized).Error(); !strings.Contains(got, "not logged in") {
		t.Errorf("signInError() with no config = %q", got)
	}
}

func TestMiddlewareUsesTheSignInMessageNotARawWrap(t *testing.T) {
	const server = "https://gator.example.test"

	f := storetest.New("marko")
	f.Err = fmt.Errorf("fetching me: %w", store.ErrUnauthorized)
	s := &state{Store: f, Cfg: &config.Config{CurrentUserName: "marko", ServerURL: server}}

	ran := false
	h := middlewareLoggedIn(func(context.Context, *state, command, store.User) error {
		ran = true
		return nil
	})

	err := h(context.Background(), s, command{Name: "tui"})
	if err == nil {
		t.Fatal("the middleware let an unauthorized call through")
	}
	if ran {
		t.Error("the handler ran despite a failed sign-in")
	}
	if got := err.Error(); !strings.Contains(got, "gator login --server "+server) {
		t.Errorf("middleware error = %q, want the sign-in instruction", got)
	}
}
