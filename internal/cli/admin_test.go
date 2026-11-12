package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/store/local"
	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func accountWithPassword(t *testing.T, db *testdb.DB, name, password string) database.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	u, err := db.CreateUserWithCredentials(t.Context(), database.CreateUserWithCredentialsParams{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Name:         name,
		PasswordHash: hash,
	})
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	return u
}

func makeAdmin(t *testing.T, db *testdb.DB, name string) {
	t.Helper()
	db.Exec(t, "UPDATE users SET is_admin = true WHERE name = $1", name)
}

func typedPassword(t *testing.T, password string) *[]string {
	t.Helper()
	asked := &[]string{}
	saved := readAdminPassword
	readAdminPassword = func(name string) (string, error) {
		*asked = append(*asked, name)
		return password, nil
	}
	t.Cleanup(func() { readAdminPassword = saved })
	return asked
}

func signedInAs(db *testdb.DB, name string) *state {
	return &state{
		Db:    db.Queries,
		Store: local.New(db.Queries, name),
		Cfg:   &config.Config{CurrentUserName: name},
	}
}

func guarded(t *testing.T, name string) handlerFunc {
	t.Helper()
	e, ok := lookup(name)
	if !ok {
		t.Fatalf("%s is not a command", name)
	}
	return guardMode(e)
}

func TestEveryAdminCommandWorksOnTheDatabaseItself(t *testing.T) {
	want := map[string]bool{"prune": true, "refetch": true, "reset": true, "feeds": true, "invite": true, "admin": true}
	for _, e := range allCommands() {
		if e.admin != want[e.name] {
			t.Errorf("%s: admin = %v, want %v", e.name, e.admin, want[e.name])
		}
		if e.admin && !e.localOnly {
			t.Errorf("%s is an admin command but not localOnly; the admin role must never be reachable through a server", e.name)
		}
		if e.bootstraps && e.name != "admin" {
			t.Errorf("%s may run without any admin; only gator admin should", e.name)
		}
	}
}

func TestOnlyAnAdminIsOfferedTheAdminCommands(t *testing.T) {
	plain := namesOf(offered(true, true, false))
	admin := namesOf(offered(true, true, true))

	for _, e := range allCommands() {
		if !e.admin || e.hidden {
			continue
		}
		if plain[e.name] {
			t.Errorf("a user who is not an admin is offered %s", e.name)
		}
		if !admin[e.name] {
			t.Errorf("an admin is not offered %s", e.name)
		}
	}
}

func TestAnAdminCommandWithoutAnyAdminSaysHowToGetOne(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	asked := typedPassword(t, "correct horse")

	err := guarded(t, "feeds")(t.Context(), signedInAs(db, "alice"), command{Name: "feeds"})
	if !errors.Is(err, errNoAdmin) {
		t.Fatalf("err = %v, want %v", err, errNoAdmin)
	}
	if len(*asked) != 0 {
		t.Error("a password was asked for although nobody can be an admin yet")
	}
}

func TestAUserWhoIsNotAnAdminIsRefused(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	accountWithPassword(t, db, "bob", "battery staple")
	makeAdmin(t, db, "alice")
	typedPassword(t, "battery staple")

	err := guarded(t, "feeds")(t.Context(), signedInAs(db, "bob"), command{Name: "feeds"})
	if err == nil || !strings.Contains(err.Error(), "is for admins") {
		t.Fatalf("err = %v, want the refusal", err)
	}
	if n := db.Count(t, "SELECT count(*) FROM admin_log"); n != 0 {
		t.Errorf("a refused command left %d admin_log rows", n)
	}
}

func TestLoggingInAsTheAdminIsNotEnoughWithoutTheirPassword(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	makeAdmin(t, db, "alice")
	asked := typedPassword(t, "guess")

	_, err := capture(t, func() error {
		return guarded(t, "feeds")(t.Context(), signedInAs(db, "alice"), command{Name: "feeds"})
	})
	if err == nil || !strings.Contains(err.Error(), "wrong password") {
		t.Fatalf("err = %v, want wrong password", err)
	}
	if len(*asked) != 1 || (*asked)[0] != "alice" {
		t.Errorf("asked for %v, want alice's password once", *asked)
	}
	if n := db.Count(t, "SELECT count(*) FROM admin_log"); n != 0 {
		t.Errorf("a refused command left %d admin_log rows", n)
	}
}

func TestAnAdminWithTheRightPasswordRunsTheCommandAndLeavesATrace(t *testing.T) {
	db := testdb.Open(t)
	alice := accountWithPassword(t, db, "alice", "correct horse")
	makeAdmin(t, db, "alice")
	typedPassword(t, "correct horse")

	_, err := capture(t, func() error {
		return guarded(t, "refetch")(t.Context(), signedInAs(db, "alice"), command{Name: "refetch", Args: []string{"--all"}})
	})
	if err != nil {
		t.Fatalf("refetch as admin: %v", err)
	}

	rows, err := db.ListAdminLog(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("admin_log has %d rows, want 1", len(rows))
	}
	got := rows[0]
	if got.UserID.UUID != alice.ID || got.UserName != "alice" || got.Command != "refetch" || got.Args != "--all" {
		t.Errorf("logged %+v, want alice running refetch --all", got)
	}
}

func TestTheFirstAdminMustProveTheyOwnTheAccount(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "marko", "correct horse")
	s := &state{Db: db.Queries, Store: local.New(db.Queries, ""), Cfg: &config.Config{}}

	typedPassword(t, "guess")
	_, err := capture(t, func() error {
		return guarded(t, "admin")(t.Context(), s, command{Name: "admin", Args: []string{"marko"}})
	})
	if err == nil || !strings.Contains(err.Error(), "wrong password") {
		t.Fatalf("err = %v, want wrong password", err)
	}
	if n := db.Count(t, "SELECT count(*) FROM users WHERE is_admin"); n != 0 {
		t.Fatalf("%d admins after a wrong password", n)
	}

	typedPassword(t, "correct horse")
	out, err := capture(t, func() error {
		return guarded(t, "admin")(t.Context(), s, command{Name: "admin", Args: []string{"marko"}})
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if !strings.Contains(out, "marko is now an admin") {
		t.Errorf("output = %q", out)
	}
	if n := db.Count(t, "SELECT count(*) FROM admin_log WHERE user_name = 'marko' AND command = 'admin'"); n != 1 {
		t.Errorf("the bootstrap left %d admin_log rows, want 1", n)
	}
}

func TestAnAccountWithoutAPasswordCannotBecomeAnAdmin(t *testing.T) {
	db := testdb.Open(t)
	db.User(t, "legacy")
	typedPassword(t, "")
	s := &state{Db: db.Queries, Store: local.New(db.Queries, ""), Cfg: &config.Config{}}

	err := guarded(t, "admin")(t.Context(), s, command{Name: "admin", Args: []string{"legacy"}})
	if err == nil || !strings.Contains(err.Error(), "no password") {
		t.Fatalf("err = %v, want the no-password refusal", err)
	}

	user, err := db.GetUser(t.Context(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	n, err := db.GrantAdmin(t.Context(), database.GrantAdminParams{UpdatedAt: time.Now(), ID: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("GrantAdmin promoted an account without a password; the SQL guard is gone")
	}
}

func TestOnceAnAdminExistsOnlyAnAdminCanMakeAnother(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	accountWithPassword(t, db, "bob", "battery staple")
	makeAdmin(t, db, "alice")
	typedPassword(t, "battery staple")

	err := guarded(t, "admin")(t.Context(), signedInAs(db, "bob"), command{Name: "admin", Args: []string{"bob"}})
	if err == nil || !strings.Contains(err.Error(), "is for admins") {
		t.Fatalf("err = %v, want the refusal", err)
	}

	typedPassword(t, "correct horse")
	_, err = capture(t, func() error {
		return guarded(t, "admin")(t.Context(), signedInAs(db, "alice"), command{Name: "admin", Args: []string{"bob"}})
	})
	if err != nil {
		t.Fatalf("alice granting bob: %v", err)
	}
	if n := db.Count(t, "SELECT count(*) FROM users WHERE is_admin"); n != 2 {
		t.Errorf("%d admins, want 2", n)
	}
}

func TestBootstrappingOnlyWorksWhileThereIsNoAdmin(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	bob := accountWithPassword(t, db, "bob", "battery staple")
	makeAdmin(t, db, "alice")

	n, err := db.GrantAdmin(t.Context(), database.GrantAdminParams{UpdatedAt: time.Now(), ID: bob.ID, OnlyIfNone: true})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("init --user would have made a second admin over an existing one")
	}
}

func TestTheLastAdminCannotBeRevoked(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	accountWithPassword(t, db, "bob", "battery staple")
	makeAdmin(t, db, "alice")
	typedPassword(t, "correct horse")
	run := guarded(t, "admin")
	s := signedInAs(db, "alice")

	err := run(t.Context(), s, command{Name: "admin", Args: []string{"--revoke", "alice"}})
	if err == nil || !strings.Contains(err.Error(), "last admin") {
		t.Fatalf("err = %v, want the last-admin refusal", err)
	}

	makeAdmin(t, db, "bob")
	_, err = capture(t, func() error {
		return run(t.Context(), s, command{Name: "admin", Args: []string{"--revoke", "alice"}})
	})
	if err != nil {
		t.Fatalf("revoking alice while bob is an admin: %v", err)
	}
	if n := db.Count(t, "SELECT count(*) FROM users WHERE is_admin AND name = 'alice'"); n != 0 {
		t.Error("alice is still an admin")
	}
}

func TestResetCountsInvitesAndTheAdminLog(t *testing.T) {
	if total(database.CountAllRowsRow{Invites: 1}) == 0 {
		t.Error("a database holding only an invite reads as empty, so reset would leave it")
	}
	if total(database.CountAllRowsRow{AdminLog: 1}) == 0 {
		t.Error("a database holding only admin_log rows reads as empty")
	}
}
