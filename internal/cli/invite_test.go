package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/invite"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

func TestTheLinkTakesItsAddressFromTheFlagTheEnvironmentOrTheBuild(t *testing.T) {
	const built = "https://built-in.example.test"

	cases := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{"the flag", "https://flag.example.test", "", "https://flag.example.test"},
		{"the environment", "", "https://env.example.test", "https://env.example.test"},
		{"the flag wins", "https://flag.example.test", "https://env.example.test", "https://flag.example.test"},
		{"the build", "", "", built},
		{"a trailing slash is dropped", "https://flag.example.test/", "", "https://flag.example.test"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(serverURLEnv, c.env)
			withDefaultServer(t, built)

			got, err := inviteServer(c.flag)
			if err != nil {
				t.Fatalf("inviteServer(%q) with %s=%q: %v", c.flag, serverURLEnv, c.env, err)
			}
			if got != c.want {
				t.Errorf("inviteServer(%q) = %q, want %q", c.flag, got, c.want)
			}
		})
	}

	t.Run("nothing at all", func(t *testing.T) {
		t.Setenv(serverURLEnv, "")
		withDefaultServer(t, "")

		if _, err := inviteServer(""); err == nil {
			t.Fatal("an invite was built with no address; the link would point nowhere")
		} else if !strings.Contains(err.Error(), serverURLEnv) {
			t.Errorf("error = %q, want it to name %s", err, serverURLEnv)
		}
	})

	t.Run("plain http to a public host", func(t *testing.T) {
		t.Setenv(serverURLEnv, "")
		withDefaultServer(t, "")

		if _, err := inviteServer("http://gator.example.test"); err == nil {
			t.Error("a link that sends the password over plain http was accepted")
		}
	})
}

func TestInvitePrintsALinkTheClientCanRead(t *testing.T) {
	db := testdb.Open(t)
	t.Setenv(serverURLEnv, "https://gator.example.test")

	out, err := capture(t, func() error {
		return handlerInvite(t.Context(), &state{Db: db.Queries}, command{
			Name: "invite", Args: []string{"--ttl=1h", "--note=za peru"},
		})
	})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}

	server, token, err := invite.Parse(strings.SplitN(out, "\n", 2)[0])
	if err != nil {
		t.Fatalf("the printed link does not parse: %v\n%s", err, out)
	}
	if server != "https://gator.example.test" {
		t.Errorf("the link points at %q", server)
	}
	if !strings.HasPrefix(token, auth.InvitePrefix) {
		t.Errorf("code %q lacks the %q prefix", token, auth.InvitePrefix)
	}

	row, err := db.GetInvite(t.Context(), database.GetInviteParams{
		Now:       time.Now(),
		TokenHash: auth.HashToken(token),
	})
	if err != nil {
		t.Fatalf("the printed code is not the one in the database: %v", err)
	}
	if row.Note != "za peru" {
		t.Errorf("note = %q", row.Note)
	}
	if row.Expired || row.ClaimedAt.Valid {
		t.Errorf("a fresh invite reads as expired=%v claimed=%v", row.Expired, row.ClaimedAt.Valid)
	}
	if n := db.Count(t, "select count(*) from invites where token_hash = $1", token); n != 0 {
		t.Error("the code itself was stored; only its hash may be")
	}
}

func TestInviteListTellsActiveFromExpiredAndSpent(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "pera")
	s := &state{Db: db.Queries}
	inAZoneAheadOfUTC(t)

	cases := []struct {
		note    string
		expires time.Duration
		claimed bool
		want    string
	}{
		{"za peru", time.Hour, false, "active"},
		{"stari", -time.Hour, false, "expired"},
		{"potrosen", time.Hour, true, "used"},
	}

	for _, c := range cases {
		row, err := db.CreateInvite(t.Context(), database.CreateInviteParams{
			ID:        uuid.New(),
			TokenHash: c.note,
			Note:      c.note,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(c.expires),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !c.claimed {
			continue
		}
		if _, err := db.ClaimInvite(t.Context(), database.ClaimInviteParams{
			Now: time.Now(), TokenHash: c.note,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.SetInviteUser(t.Context(), database.SetInviteUserParams{
			ID: row.ID, UserID: uuid.NullUUID{UUID: user.ID, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
	}

	out, err := capture(t, func() error {
		return handlerInvite(t.Context(), s, command{Name: "invite", Args: []string{"--list"}})
	})
	if err != nil {
		t.Fatalf("invite --list: %v", err)
	}

	for _, c := range cases {
		line := lineWith(out, c.note)
		switch {
		case line == "":
			t.Errorf("the listing never mentions %q:\n%s", c.note, out)
		case !strings.HasPrefix(line, c.want):
			t.Errorf("%q reads as %q, want %q — the status column:\n%s", c.note, line, c.want, out)
		}
	}
	if line := lineWith(out, "potrosen"); !strings.Contains(line, "pera") {
		t.Errorf("a spent invite does not say who used it: %q", line)
	}
}

func lineWith(out, want string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	return ""
}

func inAZoneAheadOfUTC(t *testing.T) {
	t.Helper()
	was := time.Local
	time.Local = time.FixedZone("TEST", 2*60*60)
	t.Cleanup(func() { time.Local = was })
}

func TestRevokeDeletesAnUnusedLinkAndKeepsASpentOne(t *testing.T) {
	db := testdb.Open(t)
	s := &state{Db: db.Queries}
	t.Setenv(serverURLEnv, "https://gator.example.test")

	out, err := capture(t, func() error {
		return handlerInvite(t.Context(), s, command{Name: "invite", Args: []string{}})
	})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	_, token, err := invite.Parse(strings.SplitN(out, "\n", 2)[0])
	if err != nil {
		t.Fatal(err)
	}

	row, err := db.GetInvite(t.Context(), database.GetInviteParams{Now: time.Now(), TokenHash: auth.HashToken(token)})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := capture(t, func() error {
		return handlerInvite(t.Context(), s, command{Name: "invite", Args: []string{"--revoke=" + row.ID.String()}})
	}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if n := db.Count(t, "select count(*) from invites"); n != 0 {
		t.Errorf("%d invites left after revoking the only one", n)
	}

	if _, err := capture(t, func() error {
		return handlerInvite(t.Context(), s, command{Name: "invite", Args: []string{"--revoke=" + uuid.New().String()}})
	}); err == nil {
		t.Error("revoking an id that is not there reported success")
	}

	spent, err := db.CreateInvite(t.Context(), database.CreateInviteParams{
		ID:        uuid.New(),
		TokenHash: "already-spent",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ClaimInvite(t.Context(), database.ClaimInviteParams{Now: time.Now(), TokenHash: "already-spent"}); err != nil {
		t.Fatal(err)
	}

	if _, err := capture(t, func() error {
		return handlerInvite(t.Context(), s, command{Name: "invite", Args: []string{"--revoke=" + spent.ID.String()}})
	}); err == nil {
		t.Error("a spent invite was revoked; the record of who was let in must survive")
	}
	if n := db.Count(t, "select count(*) from invites where id = $1", spent.ID); n != 1 {
		t.Error("revoking deleted the spent invite")
	}
}

func TestRegisterTakesTheCodeAndTheServerFromTheLink(t *testing.T) {
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
	withDefaultServer(t, "http://127.0.0.1:1")

	if _, err := capture(t, func() error {
		return handlerRegister(t.Context(), &state{}, command{Name: "register", Args: []string{
			"--email=pera@example.com", "--password=pera-pass-91",
			"--link=" + srv.URL + invite.Path + "gatinv_from-the-link", "pera",
		}})
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if got.Invite != "gatinv_from-the-link" {
		t.Errorf("the server was sent invite %q, want the code out of the link", got.Invite)
	}
	if got.Name != "pera" || got.Email != "pera@example.com" {
		t.Errorf("request = %+v", got)
	}
}

func TestALinkThatFightsTheServerFlagIsRefused(t *testing.T) {
	if _, _, err := registrationTarget("https://other.example.test", "https://gator.example.test/join/gatinv_x"); err == nil {
		t.Fatal("a link and a --server pointing at different hosts were both accepted")
	}
	server, token, err := registrationTarget("https://gator.example.test/", "https://gator.example.test/join/gatinv_x")
	if err != nil {
		t.Fatalf("the same host written two ways was refused: %v", err)
	}
	if server != "https://gator.example.test" || token != "gatinv_x" {
		t.Errorf("registrationTarget = %q, %q", server, token)
	}
}
