package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

func withDefaultServer(t *testing.T, url string) {
	t.Helper()
	was := defaultServer
	defaultServer = url
	t.Cleanup(func() { defaultServer = was })
}

func TestTheBuiltInServerIsUsedOnlyWhenNothingIsConfigured(t *testing.T) {
	const built = "https://reader.example.test"

	cases := []struct {
		name   string
		config string
		flag   string
		want   string
	}{
		{"no config at all", "", "", built},
		{"config configures nothing", `{"db_url":"","current_user_name":""}`, "", built},
		{"a local database wins over the built-in server", `{"db_url":"postgres://localhost/gator"}`, "", ""},
		{"a configured server wins over the built-in one", `{"db_url":"","server_url":"https://other.example.test"}`, "", "https://other.example.test"},
		{"the flag wins over everything", `{"db_url":"postgres://localhost/gator"}`, "https://flag.example.test", "https://flag.example.test"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			withDefaultServer(t, built)

			if c.config != "" {
				path := filepath.Join(home, ".gatorconfig.json")
				if err := os.WriteFile(path, []byte(c.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if got := serverTarget(c.flag); got != c.want {
				t.Errorf("serverTarget(%q) = %q, want %q", c.flag, got, c.want)
			}
		})
	}
}

func TestWithoutABuiltInServerNothingChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	withDefaultServer(t, "")

	if got := serverTarget(""); got != "" {
		t.Errorf("serverTarget() = %q on a source build, want the local path", got)
	}
}

func TestVersionNamesTheBuiltInServer(t *testing.T) {
	withDefaultServer(t, "https://reader.example.test")

	out, err := capture(t, func() error {
		return handlerVersion(t.Context(), &state{}, command{Name: "version"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "reader.example.test") {
		t.Errorf("version = %q, want it to name the server the build talks to", out)
	}
	if !strings.Contains(out, "gator init") {
		t.Errorf("version = %q, want it to name the way back to a local database", out)
	}

	withDefaultServer(t, "")
	bare, err := capture(t, func() error {
		return handlerVersion(t.Context(), &state{}, command{Name: "version"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bare, "server") {
		t.Errorf("version = %q on a source build, want no server line", bare)
	}
}

func TestRegisterAndLoginReachTheBuiltInServerWithoutAFlag(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(wire.TokenResponse{
			Token: "gat_fake",
			User:  wire.User{ID: uuid.New(), Name: "pera"},
		})
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	withDefaultServer(t, srv.URL)

	out, err := capture(t, func() error {
		return handlerRegister(t.Context(), &state{},
			command{Name: "register", Args: []string{"--email=pera@example.com", "--password=pera-pass-91", "pera"}})
	})
	if err != nil {
		t.Fatalf("register against the built-in server: %v (output %q)", err, out)
	}
	if want := "POST /v1/users"; len(paths) == 0 || paths[0] != want {
		t.Fatalf("server saw %v, want %q — register ignored the built-in server", paths, want)
	}
	if !strings.Contains(out, srv.URL) {
		t.Errorf("register printed %q, want it to name the server it signed you in to", out)
	}

	cfg, err := config.Read()
	if err != nil {
		t.Fatalf("reading the config register wrote: %v", err)
	}
	if cfg.ServerURL != srv.URL {
		t.Errorf("config server_url = %q, want %q", cfg.ServerURL, srv.URL)
	}
	if cfg.DBURL != "" {
		t.Errorf("config db_url = %q, want a client that needs no database", cfg.DBURL)
	}

	if _, err := capture(t, func() error {
		return handlerLogin(t.Context(), &state{},
			command{Name: "login", Args: []string{"--password=pera-pass-91", "pera"}})
	}); err != nil {
		t.Fatalf("login with the server already in the config: %v", err)
	}
	if want := "POST /v1/tokens"; len(paths) < 2 || paths[1] != want {
		t.Errorf("server saw %v, want %q second", paths, want)
	}
}
