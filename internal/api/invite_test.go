package api_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/api"
	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
	"github.com/MSamoilovic/gator-cli/internal/wire"
)

func TestRegistrationNeedsTheInviteCode(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	cases := []struct {
		name   string
		user   string
		invite string
		want   int
	}{
		{"no code at all", "bob", "", http.StatusForbidden},
		{"whitespace is no code", "gina", " \t ", http.StatusForbidden},
		{"the wrong code", "carol", "open-sesame", http.StatusForbidden},
		{"a prefix of the code", "dave", invite[:4], http.StatusForbidden},
		{"the code with spaces around it", "erin", "  " + invite + "\n", http.StatusCreated},
		{"the code", "frank", invite, http.StatusCreated},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, body := raw(t, "POST", srv.URL+"/v1/users", "", map[string]string{
				"name": c.user, "email": c.user + "@example.com",
				"password": "correct horse battery", "invite": c.invite,
			})
			if res.StatusCode != c.want {
				t.Fatalf("status %d, want %d: %s", res.StatusCode, c.want, body)
			}

			created := db.Count(t, "select count(*) from users where name = $1", c.user)
			if want := map[bool]int{true: 1, false: 0}[c.want == http.StatusCreated]; created != want {
				t.Errorf("%d rows for %s, want %d", created, c.user, want)
			}
			if c.want == http.StatusForbidden && strings.Contains(string(body), invite) {
				t.Errorf("the refusal quotes the code back: %s", body)
			}
			if strings.TrimSpace(c.invite) == "" && c.want == http.StatusForbidden &&
				!strings.Contains(string(body), "--invite") {
				t.Errorf("sending no code at all answers %s, want it to name the flag — otherwise "+
					"the one thing the user can act on is missing", body)
			}
		})
	}
}

func TestAServerWithNoInviteCodeTakesNoNewAccountsAtAll(t *testing.T) {
	db := testdb.Open(t)
	reg, err := remote.Register(t.Context(), serve(t, db).URL, wire.RegisterRequest{
		Name: "alice", Email: "alice@example.com", Password: "correct horse battery", Invite: invite,
	})
	if err != nil {
		t.Fatal(err)
	}

	closed := httptest.NewServer(api.New(db.Queries, log.New(io.Discard, "", 0), api.Config{}))
	defer closed.Close()

	for _, given := range []string{"", invite} {
		res, body := raw(t, "POST", closed.URL+"/v1/users", "", map[string]string{
			"name": "bob", "email": "bob@example.com", "password": "correct horse battery", "invite": given,
		})
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("registering with invite %q = %d, want 403 — a server started without a code must "+
				"not fall back to open registration: %s", given, res.StatusCode, body)
		}
	}
	if n := db.Count(t, "select count(*) from users where name = 'bob'"); n != 0 {
		t.Errorf("%d rows for bob", n)
	}

	if _, err := remote.Login(t.Context(), closed.URL, "alice", "correct horse battery"); err != nil {
		t.Errorf("logging in on a closed server: %v — closing registration must not lock out the "+
			"accounts that already exist", err)
	}
	if _, err := remote.New(closed.URL, reg.Token).Me(t.Context()); err != nil {
		t.Errorf("an existing token on a closed server: %v", err)
	}
}
