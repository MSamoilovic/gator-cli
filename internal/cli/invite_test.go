package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

func TestTheInviteCodeComesFromTheFlagOrTheEnvironment(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{"neither closes registration", "", "", ""},
		{"the flag", "from-flag", "", "from-flag"},
		{"the environment", "", "from-env", "from-env"},
		{"the flag wins", "from-flag", "from-env", "from-flag"},
		{"both are trimmed", "  padded  ", "", "padded"},
		{"an environment code is trimmed too", "", "\tpadded\n", "padded"},
		{"whitespace is not a code", "   ", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(inviteEnv, c.env)
			if got := inviteCode(c.flag); got != c.want {
				t.Errorf("inviteCode(%q) with %s=%q = %q, want %q", c.flag, inviteEnv, c.env, got, c.want)
			}
		})
	}
}

func TestRegisterSendsTheInviteCodeToTheServer(t *testing.T) {
	var got wire.RegisterRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(wire.TokenResponse{
			Token: "gat_fake",
			User:  wire.User{ID: uuid.New(), Name: "pera"},
		})
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	withDefaultServer(t, srv.URL)

	if _, err := capture(t, func() error {
		return handlerRegister(t.Context(), &state{}, command{Name: "register", Args: []string{
			"--email=pera@example.com", "--password=pera-pass-91", "--invite=let-me-in", "pera",
		}})
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if got.Invite != "let-me-in" {
		t.Errorf("the server was sent invite %q, want it carried from --invite", got.Invite)
	}
	if got.Name != "pera" || got.Email != "pera@example.com" {
		t.Errorf("request = %+v", got)
	}
}
