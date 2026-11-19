package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/api"
	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/testdb"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

var loopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}

func serveWith(t *testing.T, db *testdb.DB, opts api.Options) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(api.NewForTest(db.Queries, log.New(io.Discard, "", 0), opts))
	t.Cleanup(srv.Close)
	return srv
}

func send(t *testing.T, method, url, token string, body any, header http.Header) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		r = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res, data
}

func from(addr string) http.Header {
	return http.Header{"X-Forwarded-For": {addr}}
}

func login(t *testing.T, srv *httptest.Server, name, password string, header http.Header) *http.Response {
	t.Helper()
	res, _ := send(t, "POST", srv.URL+"/v1/tokens", "", wire.TokenRequest{Login: name, Password: password}, header)
	return res
}

func wantTooMany(t *testing.T, res *http.Response, what string) {
	t.Helper()
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("%s = %d, want 429", what, res.StatusCode)
	}
	if secs, err := strconv.Atoi(res.Header.Get("Retry-After")); err != nil || secs < 1 {
		t.Errorf("%s: Retry-After = %q, want a whole number of seconds", what, res.Header.Get("Retry-After"))
	}
}

func token(t *testing.T, db *testdb.DB, srv *httptest.Server, name string) string {
	t.Helper()
	res, err := remote.Register(t.Context(), srv.URL, wire.RegisterRequest{
		Name: name, Email: name + "@example.com", Password: "correct horse battery",
		Invite: db.Invite(t, time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("registering %s: %v", name, err)
	}
	return res.Token
}

func TestTheEleventhLoginFromOneAddressInAMinuteIsRefused(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	for i := range 10 {
		if res := login(t, srv, "nobody"+strconv.Itoa(i), "wrong password", nil); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401", i+1, res.StatusCode)
		}
	}
	wantTooMany(t, login(t, srv, "nobody10", "wrong password", nil), "11th login")
}

func TestAForgedForwardedForBuysNoMoreAttempts(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	for i := range 10 {
		login(t, srv, "nobody"+strconv.Itoa(i), "wrong password", from("198.51.100."+strconv.Itoa(i)))
	}
	wantTooMany(t, login(t, srv, "nobody10", "wrong password", from("198.51.100.200")), "11th login with a fresh X-Forwarded-For")
}

func TestBehindATrustedProxyEachClientHasItsOwnAllowance(t *testing.T) {
	db := testdb.Open(t)
	srv := serveWith(t, db, api.Options{TrustedProxies: loopback})

	for i := range 10 {
		login(t, srv, "nobody"+strconv.Itoa(i), "wrong password", from("198.51.100.1"))
	}
	wantTooMany(t, login(t, srv, "nobody10", "wrong password", from("198.51.100.1")), "11th from the same client")
	if res := login(t, srv, "nobody11", "wrong password", from("198.51.100.2")); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("another client behind the proxy = %d, want 401: it shares no bucket with the first", res.StatusCode)
	}
}

func TestGuessingOneAccountsPasswordIsSlowedWhateverTheAddress(t *testing.T) {
	db := testdb.Open(t)
	srv := serveWith(t, db, api.Options{TrustedProxies: loopback})
	token(t, db, srv, "victim")

	for i := range 10 {
		if res := login(t, srv, "victim", "guess"+strconv.Itoa(i), from("198.51.100."+strconv.Itoa(i))); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("guess %d = %d, want 401", i+1, res.StatusCode)
		}
	}
	wantTooMany(t, login(t, srv, "VICTIM", "guess10", from("198.51.100.99")), "11th guess from a new address")
	if res := login(t, srv, "someone-else", "guess", from("198.51.100.98")); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("guessing another name = %d, want 401: the allowance is per name", res.StatusCode)
	}
}

func TestSuccessfulLoginsSpendNothing(t *testing.T) {
	db := testdb.Open(t)
	srv := serveWith(t, db, api.Options{TrustedProxies: loopback})
	token(t, db, srv, "marko")

	for i := range 15 {
		if res := login(t, srv, "marko", "correct horse battery", from("198.51.100."+strconv.Itoa(i))); res.StatusCode != http.StatusOK {
			t.Fatalf("login %d = %d, want 200: signing in on many devices is not guessing", i+1, res.StatusCode)
		}
	}
}

func TestTwoUsersHaveSeparateAllowancesForTheCostlyRoutes(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	alice, bob := token(t, db, srv, "alice"), token(t, db, srv, "bob")
	fulltext := srv.URL + "/v1/posts/" + uuid.NewString() + "/fulltext"

	for i := range 60 {
		if res, body := send(t, "GET", fulltext, alice, nil, nil); res.StatusCode != http.StatusNotFound {
			t.Fatalf("alice's request %d = %d, want 404: %s", i+1, res.StatusCode, body)
		}
	}
	res, _ := send(t, "GET", fulltext, alice, nil, nil)
	wantTooMany(t, res, "alice's 61st full-text request")

	if res, _ := send(t, "GET", fulltext, bob, nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("bob = %d, want 404: alice's requests must not spend his allowance", res.StatusCode)
	}
	if res, _ := send(t, "GET", srv.URL+"/v1/me", alice, nil, nil); res.StatusCode != http.StatusOK {
		t.Errorf("alice's cheap request = %d, want 200: the costly allowance is separate", res.StatusCode)
	}
}

func TestRegistrationRefusesNamesThatPassForAnEmailOrCarryAnEscape(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	register := func(name, password string) (*http.Response, []byte) {
		return send(t, "POST", srv.URL+"/v1/users", "", wire.RegisterRequest{
			Name: name, Email: uuid.NewString() + "@example.com", Password: password,
			Invite: db.Invite(t, time.Now().Add(time.Hour)),
		}, nil)
	}

	for _, name := range []string{"a@b", "evil\x1b[31m", "two words", "slash/name"} {
		if res, body := register(name, "correct horse battery"); res.StatusCode != http.StatusBadRequest {
			t.Errorf("name %q = %d, want 400: %s", name, res.StatusCode, body)
		}
	}
	if res, body := register("ok_name-1.x", "correct horse battery"); res.StatusCode != http.StatusCreated {
		t.Errorf("name ok_name-1.x = %d, want 201: %s", res.StatusCode, body)
	}

	res, body := register("longpass", strings.Repeat("a", 73))
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "password must be at most 72 bytes") {
		t.Errorf("73-byte password = %d %s, want 400 with a fixed sentence", res.StatusCode, body)
	}
	if strings.Contains(string(body), "bcrypt") || strings.Contains(string(body), "hashing") {
		t.Errorf("the reply leaks the library's own error: %s", body)
	}
}

func TestOversizedInputIsABadRequest(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	tok := token(t, db, srv, "marko")

	ids := make([]uuid.UUID, wire.MaxReadBatch+1)
	for i := range ids {
		ids[i] = uuid.New()
	}
	if res, body := send(t, "POST", srv.URL+"/v1/posts/read", tok, wire.MarkReadRequest{PostIDs: ids}, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("%d post_ids = %d, want 400: %s", len(ids), res.StatusCode, body)
	}
	if res, _ := send(t, "POST", srv.URL+"/v1/posts/read", tok, wire.MarkReadRequest{PostIDs: ids[:wire.MaxReadBatch]}, nil); res.StatusCode != http.StatusNoContent {
		t.Errorf("exactly %d post_ids = %d, want 204", wire.MaxReadBatch, res.StatusCode)
	}

	for _, query := range []string{"q=" + strings.Repeat("a", 201), "offset=100001"} {
		if res, body := send(t, "GET", srv.URL+"/v1/posts?"+query, tok, nil, nil); res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400: %s", query[:8], res.StatusCode, body)
		}
	}
}

func TestALimitOfZeroMeansThePageSizeNotNothing(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	tok := token(t, db, srv, "marko")

	user, err := db.GetUser(t.Context(), "marko")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.Feed(t, user.ID, "Example", "https://example.com/feed.xml")
	db.Follow(t, user.ID, feed.ID)
	for i := range 3 {
		db.Post(t, feed.ID, "post "+strconv.Itoa(i), "https://example.com/"+strconv.Itoa(i), time.Now())
	}

	res, body := send(t, "GET", srv.URL+"/v1/posts?limit=0", tok, nil, nil)
	var out wire.Posts
	if err := json.Unmarshal(body, &out); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("limit=0 = %d %s", res.StatusCode, body)
	}
	if len(out.Posts) != 3 {
		t.Errorf("limit=0 returned %d posts, want all 3: LIMIT 0 is never what a client means", len(out.Posts))
	}
}

func TestResponsesCarryTheirSafetyHeaders(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	tok := token(t, db, srv, "marko")

	res, _ := send(t, "GET", srv.URL+"/v1/healthz", "", nil, nil)
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("healthz X-Content-Type-Options = %q, want nosniff", got)
	}
	res, _ = send(t, "GET", srv.URL+"/v1/me", tok, nil, nil)
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("an authed reply's Cache-Control = %q, want no-store", got)
	}
	res, _ = send(t, "GET", srv.URL+"/join/"+db.Invite(t, time.Now().Add(time.Hour)), "", nil, nil)
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("join page Referrer-Policy = %q, want no-referrer: the token is in the path", got)
	}
}

func TestTheJoinPageNamesThePublicAddressNotWhatTheRequestClaims(t *testing.T) {
	db := testdb.Open(t)
	srv := serveWith(t, db, api.Options{PublicURL: "https://gator.example.com/"})
	tok := db.Invite(t, time.Now().Add(time.Hour))

	res, body := send(t, "GET", srv.URL+"/join/"+tok, "", nil, http.Header{"X-Forwarded-Proto": {"http"}})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	if want := "--link=https://gator.example.com/join/" + tok; !strings.Contains(string(body), want) {
		t.Errorf("page does not carry %q:\n%s", want, body)
	}
}
