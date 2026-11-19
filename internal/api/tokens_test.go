package api_test

import (
	"net/http"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
)

func me(t *testing.T, url, tok string) int {
	t.Helper()
	res, _ := send(t, "GET", url+"/v1/me", tok, nil, nil)
	return res.StatusCode
}

func daysLeft(t *testing.T, db *testdb.DB, tok string) float64 {
	t.Helper()
	var days float64
	db.QueryRow(t, &days, "SELECT extract(epoch FROM expires_at - now()) / 86400 FROM api_tokens WHERE token_hash = $1", auth.HashToken(tok))
	return days
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	tok := token(t, db, srv, "alice")

	if got := me(t, srv.URL, tok); got != http.StatusOK {
		t.Fatalf("fresh token: /v1/me = %d, want 200", got)
	}
	db.Exec(t, "UPDATE api_tokens SET expires_at = now() - interval '1 minute' WHERE token_hash = $1", auth.HashToken(tok))
	if got := me(t, srv.URL, tok); got != http.StatusUnauthorized {
		t.Errorf("expired token: /v1/me = %d, want 401", got)
	}
}

func TestANewTokenLastsNinetyDays(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	tok := token(t, db, srv, "alice")

	if d := daysLeft(t, db, tok); d < 89.9 || d > 90.1 {
		t.Errorf("a new token expires in %.2f days, want 90", d)
	}
}

func TestUsingATokenPushesItsExpiryBackAtMostOnceADay(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	tok := token(t, db, srv, "alice")
	hash := auth.HashToken(tok)

	db.Exec(t, "UPDATE api_tokens SET expires_at = now() + interval '1 day', last_used_at = NULL WHERE token_hash = $1", hash)
	me(t, srv.URL, tok)
	if d := daysLeft(t, db, tok); d < 89.9 {
		t.Fatalf("after its first use the token expires in %.2f days, want 90", d)
	}

	db.Exec(t, "UPDATE api_tokens SET expires_at = now() + interval '1 day', last_used_at = now() - interval '1 hour' WHERE token_hash = $1", hash)
	me(t, srv.URL, tok)
	if d := daysLeft(t, db, tok); d > 1.1 {
		t.Errorf("a token used an hour ago was written again: expires in %.2f days, want 1", d)
	}

	db.Exec(t, "UPDATE api_tokens SET last_used_at = now() - interval '25 hours' WHERE token_hash = $1", hash)
	me(t, srv.URL, tok)
	if d := daysLeft(t, db, tok); d < 89.9 {
		t.Errorf("a token last used yesterday was not extended: expires in %.2f days, want 90", d)
	}
}

func TestSigningOutEverywhereRevokesOnlyThatUsersTokens(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	laptop := token(t, db, srv, "alice")
	res, err := remote.Login(t.Context(), srv.URL, "alice", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	phone := res.Token
	bob := token(t, db, srv, "bob")

	n, err := remote.New(srv.URL, laptop).LogoutAll(t.Context())
	if err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if n != 2 {
		t.Errorf("revoked %d tokens, want alice's 2", n)
	}
	for name, tok := range map[string]string{"alice's laptop": laptop, "alice's phone": phone} {
		if got := me(t, srv.URL, tok); got != http.StatusUnauthorized {
			t.Errorf("%s after signing out everywhere: /v1/me = %d, want 401", name, got)
		}
	}
	if got := me(t, srv.URL, bob); got != http.StatusOK {
		t.Errorf("bob after alice signed out everywhere: /v1/me = %d, want 200", got)
	}
}

func TestExpiredTokensAreDeletedAndLiveOnesKept(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	old := token(t, db, srv, "alice")
	live := token(t, db, srv, "bob")
	db.Exec(t, "UPDATE api_tokens SET expires_at = now() - interval '1 second' WHERE token_hash = $1", auth.HashToken(old))

	n, err := db.DeleteExpiredAPITokens(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("deleted %d tokens, want 1", n)
	}
	if got := me(t, srv.URL, live); got != http.StatusOK {
		t.Errorf("the live token after cleanup: /v1/me = %d, want 200", got)
	}
}
