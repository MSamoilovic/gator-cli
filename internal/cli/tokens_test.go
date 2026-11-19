package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/credentials"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func signedInOn(t *testing.T, db *testdb.DB, user database.User, devices int) {
	t.Helper()
	for range devices {
		if _, err := db.CreateAPIToken(t.Context(), database.CreateAPITokenParams{
			ID:           uuid.New(),
			UserID:       user.ID,
			TokenHash:    uuid.NewString(),
			CreatedAt:    time.Now(),
			LifetimeDays: 90,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func tokensOf(t *testing.T, db *testdb.DB, user database.User) int {
	t.Helper()
	return db.Count(t, "SELECT count(*) FROM api_tokens WHERE user_id = $1", user.ID)
}

func revokeTokens(t *testing.T, db *testdb.DB, caller, target string) (string, error) {
	t.Helper()
	cmd := command{Name: "admin", Args: []string{"--revoke-tokens", target}}
	return capture(t, func() error { return guarded(t, "admin")(t.Context(), signedInAs(db, caller), cmd) })
}

func TestRevokingTokensNeedsAnAdmin(t *testing.T) {
	db := testdb.Open(t)
	alice := accountWithPassword(t, db, "alice", "correct horse")
	bob := accountWithPassword(t, db, "bob", "battery staple")
	signedInOn(t, db, alice, 1)
	signedInOn(t, db, bob, 1)

	typedPassword(t, "battery staple")
	if _, err := revokeTokens(t, db, "bob", "alice"); !errors.Is(err, errNoAdmin) {
		t.Errorf("with no admin at all: err = %v, want %v", err, errNoAdmin)
	}

	makeAdmin(t, db, "alice")
	if _, err := revokeTokens(t, db, "bob", "alice"); err == nil || !strings.Contains(err.Error(), "is for admins") {
		t.Errorf("as bob, who is not an admin: err = %v, want the refusal", err)
	}

	typedPassword(t, "guess")
	if _, err := revokeTokens(t, db, "alice", "bob"); err == nil || !strings.Contains(err.Error(), "wrong password") {
		t.Errorf("as alice with the wrong password: err = %v, want wrong password", err)
	}

	if tokensOf(t, db, alice) != 1 || tokensOf(t, db, bob) != 1 {
		t.Error("a refused --revoke-tokens deleted a token")
	}
	if n := db.Count(t, "SELECT count(*) FROM admin_log"); n != 0 {
		t.Errorf("refused attempts left %d admin_log rows", n)
	}
}

func TestAnAdminSignsSomeoneOutOfEveryDeviceAndLeavesATrace(t *testing.T) {
	db := testdb.Open(t)
	alice := accountWithPassword(t, db, "alice", "correct horse")
	bob := accountWithPassword(t, db, "bob", "battery staple")
	makeAdmin(t, db, "alice")
	signedInOn(t, db, alice, 1)
	signedInOn(t, db, bob, 3)
	typedPassword(t, "correct horse")

	out, err := revokeTokens(t, db, "alice", "bob")
	if err != nil {
		t.Fatalf("--revoke-tokens bob: %v", err)
	}
	if !strings.Contains(out, "Signed bob out of 3 sessions") {
		t.Errorf("printed %q, want how many sessions went", out)
	}
	if n := tokensOf(t, db, bob); n != 0 {
		t.Errorf("bob still has %d tokens", n)
	}
	if n := tokensOf(t, db, alice); n != 1 {
		t.Errorf("alice has %d tokens, want her own 1 untouched", n)
	}

	rows, err := db.ListAdminLog(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Command != "admin" || rows[0].Args != "--revoke-tokens bob" {
		t.Errorf("admin_log = %+v, want one admin --revoke-tokens bob", rows)
	}
}

func TestRevokingTokensOfNobodySaysSo(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	makeAdmin(t, db, "alice")
	typedPassword(t, "correct horse")

	if _, err := revokeTokens(t, db, "alice", "carol"); err == nil || !strings.Contains(err.Error(), "no user named carol") {
		t.Errorf("err = %v, want no user named carol", err)
	}
}

type fakeTokenServer struct {
	status int
	calls  []string
}

func (f *fakeTokenServer) start(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		if f.status != http.StatusOK {
			w.WriteHeader(f.status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"revoked":3}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func connectedTo(t *testing.T, server string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	leaveOfflineData(t, home, "alice")
	cfg := config.Config{ServerURL: server, CurrentUserName: "alice"}
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}
	if err := credentials.Save(credentials.Credentials{ServerURL: server, Token: "gat_alice"}); err != nil {
		t.Fatal(err)
	}
	return home
}

func logoutAll(t *testing.T) (string, error) {
	t.Helper()
	return capture(t, func() error {
		return handlerLogout(t.Context(), nil, command{Name: "logout", Args: []string{"--all"}})
	})
}

func TestLoggingOutEverywhereWhenTheServerRefusesChangesNothingHere(t *testing.T) {
	fake := &fakeTokenServer{status: http.StatusServiceUnavailable}
	server := fake.start(t)
	home := connectedTo(t, server)

	if _, err := logoutAll(t); err == nil {
		t.Fatal("logout --all succeeded although the server refused")
	}

	cfg, err := config.Read()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != server || cfg.CurrentUserName != "alice" {
		t.Errorf("config after a refused logout --all = %+v, want still connected", cfg)
	}
	if _, err := credentials.Load(server); err != nil {
		t.Errorf("the token is gone after a refused logout --all: %v", err)
	}
	if left := offlineFilesLeft(t, home); len(left) != 2 {
		t.Errorf("a refused logout --all left only %v; the queue would be lost", left)
	}
}

func TestLoggingOutEverywhereRevokesOnTheServerThenDisconnects(t *testing.T) {
	fake := &fakeTokenServer{status: http.StatusOK}
	server := fake.start(t)
	home := connectedTo(t, server)

	out, err := logoutAll(t)
	if err != nil {
		t.Fatalf("logout --all: %v", err)
	}
	if !strings.Contains(out, "Signed out 3 sessions on "+server) {
		t.Errorf("printed %q, want how many sessions went", out)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "DELETE /v1/tokens" {
		t.Errorf("server saw %v, want only DELETE /v1/tokens", fake.calls)
	}

	cfg, err := config.Read()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "" || cfg.CurrentUserName != "" {
		t.Errorf("config after logout --all = %+v, want disconnected", cfg)
	}
	if _, err := os.Stat(filepath.Join(home, ".gatorcredentials.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("credentials file after logout --all: %v, want it gone", err)
	}
	if left := offlineFilesLeft(t, home); len(left) != 0 {
		t.Errorf("logout --all left %v behind", left)
	}
}

func TestLoggingOutEverywhereIsRefusedOnALocalInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.Config{DBURL: "postgres://localhost/gator", CurrentUserName: "alice"}
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}

	if _, err := logoutAll(t); err == nil || !strings.Contains(err.Error(), "this install is local") {
		t.Fatalf("err = %v, want the local refusal", err)
	}
	if got, _ := config.Read(); got.CurrentUserName != "alice" {
		t.Error("a refused logout --all logged alice out")
	}
}

func TestEveryAggTickDeletesTheExpiredTokens(t *testing.T) {
	db := testdb.Open(t)
	alice := db.User(t, "alice")
	bob := db.User(t, "bob")
	signedInOn(t, db, alice, 2)
	signedInOn(t, db, bob, 1)
	db.Exec(t, "UPDATE api_tokens SET expires_at = now() - interval '1 second' WHERE user_id = $1", alice.ID)

	out, err := capture(t, func() error {
		scrapeFeeds(t.Context(), &state{Db: db.Queries})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := tokensOf(t, db, alice); n != 0 {
		t.Errorf("alice still has %d expired tokens after an agg tick", n)
	}
	if n := tokensOf(t, db, bob); n != 1 {
		t.Errorf("bob has %d tokens after an agg tick, want his live 1", n)
	}
	if !strings.Contains(out, "Deleted 2 sign-in tokens that had expired") {
		t.Errorf("printed %q, want the count", out)
	}
}

func TestTheCountOfRevokedSessionsLeavesOutTheExpiredOnes(t *testing.T) {
	db := testdb.Open(t)
	accountWithPassword(t, db, "alice", "correct horse")
	bob := accountWithPassword(t, db, "bob", "battery staple")
	makeAdmin(t, db, "alice")
	signedInOn(t, db, bob, 1)
	db.Exec(t, "UPDATE api_tokens SET expires_at = now() - interval '1 day'")
	signedInOn(t, db, bob, 1)
	typedPassword(t, "correct horse")

	out, err := revokeTokens(t, db, "alice", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Signed bob out of 1 session\n") {
		t.Errorf("printed %q, want 1 session: the expired token was not one", out)
	}
	if n := tokensOf(t, db, bob); n != 0 {
		t.Errorf("bob still has %d tokens; the expired one should go too", n)
	}
}
