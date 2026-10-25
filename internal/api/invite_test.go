package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

func TestRegistrationNeedsALinkThatIsStillGood(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	used := db.Invite(t, time.Now().Add(time.Hour))
	register(t, db, srv, "alice")
	if _, err := remote.Register(t.Context(), srv.URL, wire.RegisterRequest{
		Name: "taken", Email: "taken@example.com", Password: "correct horse battery", Invite: used,
	}); err != nil {
		t.Fatalf("spending an invite: %v", err)
	}

	cases := []struct {
		name  string
		user  string
		token string
		want  int
	}{
		{"no link at all", "bob", "", http.StatusForbidden},
		{"whitespace is no link", "gina", " \t ", http.StatusForbidden},
		{"a code nobody handed out", "carol", "gatinv_" + strings.Repeat("x", 43), http.StatusForbidden},
		{"a code that expired", "dave", db.Invite(t, time.Now().Add(-time.Minute)), http.StatusForbidden},
		{"a code already spent", "erin", used, http.StatusForbidden},
		{"a code with spaces around it", "frank", "  " + db.Invite(t, time.Now().Add(time.Hour)) + "\n", http.StatusCreated},
		{"a code straight off the link", "grace", db.Invite(t, time.Now().Add(time.Hour)), http.StatusCreated},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, body := raw(t, "POST", srv.URL+"/v1/users", "", map[string]string{
				"name": c.user, "email": c.user + "@example.com",
				"password": "correct horse battery", "invite": c.token,
			})
			if res.StatusCode != c.want {
				t.Fatalf("status %d, want %d: %s", res.StatusCode, c.want, body)
			}

			created := db.Count(t, "select count(*) from users where name = $1", c.user)
			if want := map[bool]int{true: 1, false: 0}[c.want == http.StatusCreated]; created != want {
				t.Errorf("%d rows for %s, want %d", created, c.user, want)
			}
			if c.want != http.StatusForbidden {
				return
			}
			if token := strings.TrimSpace(c.token); token != "" && strings.Contains(string(body), token) {
				t.Errorf("the refusal quotes the code back: %s", body)
			}
			if strings.TrimSpace(c.token) == "" && !strings.Contains(string(body), "--link") {
				t.Errorf("sending no link at all answers %s, want it to name the flag — otherwise the "+
					"one thing the user can act on is missing", body)
			}
		})
	}
}

func TestEachLinkMakesOneAccountAndRemembersWho(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	token := db.Invite(t, time.Now().Add(time.Hour))

	first, err := remote.Register(t.Context(), srv.URL, wire.RegisterRequest{
		Name: "alice", Email: "alice@example.com", Password: "correct horse battery", Invite: token,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var claimed uuid.NullUUID
	db.QueryRow(t, &claimed, "select user_id from invites where claimed_at is not null")
	if !claimed.Valid || claimed.UUID != first.User.ID {
		t.Errorf("invites.user_id = %v, want %s — an invite nobody can trace back is not an invite",
			claimed, first.User.ID)
	}

	res, body := raw(t, "POST", srv.URL+"/v1/users", "", map[string]string{
		"name": "bob", "email": "bob@example.com", "password": "correct horse battery", "invite": token,
	})
	if res.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "already been used") {
		t.Errorf("reusing a spent link = %d (%s), want 403 saying it is used", res.StatusCode, body)
	}
	if n := db.Count(t, "select count(*) from users where name = 'bob'"); n != 0 {
		t.Errorf("%d rows for bob", n)
	}
}

func TestARefusedRegistrationLeavesTheLinkUsable(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	db.User(t, "alice")
	token := db.Invite(t, time.Now().Add(time.Hour))

	for _, c := range []struct {
		name           string
		user, password string
		want           int
	}{
		{"a password the server refuses", "bob", "short", http.StatusBadRequest},
		{"a name somebody already has", "alice", "correct horse battery", http.StatusConflict},
	} {
		res, body := raw(t, "POST", srv.URL+"/v1/users", "", map[string]string{
			"name": c.user, "email": c.user + "2@example.com", "password": c.password, "invite": token,
		})
		if res.StatusCode != c.want {
			t.Fatalf("%s: status %d, want %d: %s", c.name, res.StatusCode, c.want, body)
		}
		if n := db.Count(t, "select count(*) from invites where claimed_at is not null"); n != 0 {
			t.Fatalf("%s: the link was spent on a registration that failed — the invitee would have to "+
				"ask for another one", c.name)
		}
	}

	if _, err := remote.Register(t.Context(), srv.URL, wire.RegisterRequest{
		Name: "bob", Email: "bob@example.com", Password: "correct horse battery", Invite: token,
	}); err != nil {
		t.Errorf("registering after two refusals: %v", err)
	}
}

func TestAServerWithNoInvitesTakesNoNewAccountsAtAll(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	reg, err := remote.Register(t.Context(), srv.URL, wire.RegisterRequest{
		Name: "alice", Email: "alice@example.com", Password: "correct horse battery",
		Invite: db.Invite(t, time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(t, "delete from invites")

	res, body := raw(t, "POST", srv.URL+"/v1/users", "", map[string]string{
		"name": "bob", "email": "bob@example.com", "password": "correct horse battery",
		"invite": "gatinv_" + strings.Repeat("y", 43),
	})
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("registering with no invites on the server = %d, want 403 — a server nobody has been "+
			"invited to must not fall back to open registration: %s", res.StatusCode, body)
	}
	if n := db.Count(t, "select count(*) from users where name = 'bob'"); n != 0 {
		t.Errorf("%d rows for bob", n)
	}

	if _, err := remote.Login(t.Context(), srv.URL, "alice", "correct horse battery"); err != nil {
		t.Errorf("logging in with no invites left: %v — handing out no invites must not lock out the "+
			"accounts that already exist", err)
	}
	if _, err := remote.New(srv.URL, reg.Token).Me(t.Context()); err != nil {
		t.Errorf("an existing token with no invites left: %v", err)
	}
}

func TestTheJoinPageSaysWhatToRunAndSpendsNothing(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	token := db.Invite(t, time.Now().Add(time.Hour))

	res, body := raw(t, "GET", srv.URL+"/join/"+token, "", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", res.StatusCode, body)
	}
	if want := "gator register --link=" + srv.URL + "/join/" + token; !strings.Contains(string(body), want) {
		t.Errorf("the page does not carry %q:\n%s", want, body)
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain — the link lands in a browser", got)
	}

	if n := db.Count(t, "select count(*) from invites where claimed_at is not null"); n != 0 {
		t.Fatalf("opening the page spent the invite — a mail client that prefetches links would " +
			"break every invitation")
	}
	if _, err := remote.Register(t.Context(), srv.URL, wire.RegisterRequest{
		Name: "alice", Email: "alice@example.com", Password: "correct horse battery", Invite: token,
	}); err != nil {
		t.Fatalf("registering after opening the page: %v", err)
	}

	cases := map[string]struct {
		token string
		want  int
	}{
		"a spent link":          {token, http.StatusGone},
		"an expired one":        {db.Invite(t, time.Now().Add(-time.Minute)), http.StatusGone},
		"one nobody handed out": {"gatinv_" + strings.Repeat("z", 43), http.StatusNotFound},
	}
	for name, c := range cases {
		res, body := raw(t, "GET", srv.URL+"/join/"+c.token, "", nil)
		if res.StatusCode != c.want {
			t.Errorf("%s: status %d, want %d: %s", name, res.StatusCode, c.want, body)
		}
		if strings.Contains(string(body), "gator register") {
			t.Errorf("%s: the page still tells the reader to register: %s", name, body)
		}
	}
}
