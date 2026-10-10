package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/api"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/local"
	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func serve(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(api.New(db.Queries, log.New(io.Discard, "", 0)))
	t.Cleanup(srv.Close)
	return srv
}

func register(t *testing.T, srv *httptest.Server, name string) *remote.Store {
	t.Helper()
	res, err := remote.Register(t.Context(), srv.URL, name, name+"@example.com", "correct horse battery")
	if err != nil {
		t.Fatalf("registering %s: %v", name, err)
	}
	return remote.New(srv.URL, res.Token)
}

func raw(t *testing.T, method, url, token string, body any) (*http.Response, []byte) {
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

func asJSON(t *testing.T, v any) string {
	t.Helper()
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

func sorted(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return out
}

func TestEveryRouteButTheOpenOnesRefusesAMissingOrUnknownToken(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	routes := []struct{ method, path string }{
		{"GET", "/v1/me"},
		{"GET", "/v1/posts"},
		{"GET", "/v1/posts/bookmarked"},
		{"GET", "/v1/posts/read"},
		{"POST", "/v1/posts/read"},
		{"GET", "/v1/subscriptions"},
		{"POST", "/v1/subscriptions"},
		{"GET", "/v1/subscriptions/stats"},
		{"GET", "/v1/unread"},
		{"DELETE", "/v1/tokens/current"},
		{"POST", "/v1/posts/" + uuid.NewString() + "/bookmark"},
	}
	for _, rt := range routes {
		for _, token := range []string{"", "gat_nonsense"} {
			res, _ := raw(t, rt.method, srv.URL+rt.path, token, nil)
			if res.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s %s with token %q = %d, want 401", rt.method, rt.path, token, res.StatusCode)
			}
		}
	}

	if res, _ := raw(t, "GET", srv.URL+"/v1/healthz", "", nil); res.StatusCode != http.StatusOK {
		t.Errorf("healthz = %d, want 200", res.StatusCode)
	}
}

func TestRegisterThenLoginReachesTheSameUserAndNeverLeaksTheHash(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	reg, err := remote.Register(t.Context(), srv.URL, "alice", "Alice@Example.com", "correct horse battery")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	for _, login := range []string{"alice", "alice@example.com"} {
		res, err := remote.Login(t.Context(), srv.URL, login, "correct horse battery")
		if err != nil {
			t.Fatalf("login as %q: %v", login, err)
		}
		if res.User.ID != reg.User.ID {
			t.Errorf("login as %q reached user %s, want %s", login, res.User.ID, reg.User.ID)
		}
		if res.Token == reg.Token {
			t.Errorf("login as %q reused the registration token", login)
		}
	}

	res, body := raw(t, "GET", srv.URL+"/v1/me", reg.Token, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("me = %d", res.StatusCode)
	}
	if strings.Contains(strings.ToLower(string(body)), "password") || strings.Contains(string(body), "$2a$") {
		t.Errorf("/v1/me exposes the credential: %s", body)
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	register(t, srv, "alice")
	db.User(t, "localonly")

	cases := map[string]struct{ login, password string }{
		"wrong password":       {"alice", "not the password"},
		"unknown user":         {"nobody", "correct horse battery"},
		"unknown email":        {"nobody@example.com", "correct horse battery"},
		"local-only account":   {"localonly", ""},
		"local-only, any text": {"localonly", "anything"},
	}
	var bodies []string
	for name, c := range cases {
		res, body := raw(t, "POST", srv.URL+"/v1/tokens", "", map[string]string{"login": c.login, "password": c.password})
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, res.StatusCode)
		}
		bodies = append(bodies, string(body))
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Errorf("login failures differ: %q vs %q", bodies[0], b)
		}
	}
}

func TestRegisterRejectsWeakAndDuplicateAccounts(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	register(t, srv, "alice")

	cases := map[string]struct {
		name, email, password string
		want                  int
	}{
		"short password":   {"bob", "bob@example.com", "short", http.StatusBadRequest},
		"no email":         {"bob", "", "correct horse battery", http.StatusBadRequest},
		"bad email":        {"bob", "not-an-email", "correct horse battery", http.StatusBadRequest},
		"no name":          {"", "bob@example.com", "correct horse battery", http.StatusBadRequest},
		"long name":        {strings.Repeat("x", 21), "bob@example.com", "correct horse battery", http.StatusBadRequest},
		"taken name":       {"alice", "other@example.com", "correct horse battery", http.StatusConflict},
		"taken email":      {"bob", "ALICE@example.com", "correct horse battery", http.StatusConflict},
		"over 72 bytes":    {"bob", "bob@example.com", strings.Repeat("p", 80), http.StatusBadRequest},
		"valid is created": {"bob", "bob@example.com", "correct horse battery", http.StatusCreated},
	}
	for name, c := range cases {
		res, body := raw(t, "POST", srv.URL+"/v1/users", "", map[string]string{"name": c.name, "email": c.email, "password": c.password})
		if res.StatusCode != c.want {
			t.Errorf("%s: status %d, want %d (%s)", name, res.StatusCode, c.want, body)
		}
	}
}

func TestLogoutRevokesOnlyThatToken(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	first, err := remote.Register(t.Context(), srv.URL, "alice", "alice@example.com", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	second, err := remote.Login(t.Context(), srv.URL, "alice", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}

	if err := remote.New(srv.URL, first.Token).Logout(t.Context()); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if _, err := remote.New(srv.URL, first.Token).Me(t.Context()); err == nil {
		t.Error("the revoked token still works")
	}
	if _, err := remote.New(srv.URL, second.Token).Me(t.Context()); err != nil {
		t.Errorf("the other token stopped working: %v", err)
	}
}

func TestRemoteAndLocalStoresAgree(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	rs := register(t, srv, "alice")
	me, err := rs.Me(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ls := local.New(db.Queries, "alice")

	other := db.User(t, "bob")
	golang := db.Feed(t, me.ID, "Go Blog", "https://go.dev/blog/feed.atom")
	rust := db.Feed(t, other.ID, "Rust Blog", "https://blog.rust-lang.org/feed.xml")
	unfollowed := db.Feed(t, other.ID, "Elsewhere", "https://example.com/feed")
	db.Follow(t, me.ID, golang.ID)
	db.Follow(t, me.ID, rust.ID)

	now := time.Now().Add(-time.Hour)
	var all []uuid.UUID
	for i, f := range []struct {
		feed  uuid.UUID
		title string
	}{{golang.ID, "Generics"}, {golang.ID, "Iterators"}, {rust.ID, "Editions"}, {rust.ID, "Async"}, {unfollowed.ID, "Invisible"}} {
		p := db.Post(t, f.feed, f.title, "https://example.com/post/"+f.title, now.Add(time.Duration(i)*time.Minute))
		all = append(all, p.ID)
	}
	db.Bookmark(t, me.ID, all[0])
	db.Bookmark(t, other.ID, all[2])
	db.MarkRead(t, me.ID, all[1])
	db.MarkRead(t, me.ID, all[2])

	queries := map[string]store.PostQuery{
		"default":        {Limit: 50},
		"ascending":      {Limit: 50, SortDir: "asc"},
		"one feed by id": {Limit: 50, FeedID: golang.ID},
		"feed by name":   {Limit: 50, FeedName: "rust"},
		"unread only":    {Limit: 50, UnreadOnly: true},
		"since":          {Limit: 50, Since: now.Add(150 * time.Second)},
		"paged":          {Limit: 1, Offset: 1},
		"search":         {Limit: 50, Query: "async"},
		"search nothing": {Limit: 50, Query: "zzzz-no-match"},
	}
	for name, q := range queries {
		want, err := ls.Posts(t.Context(), q)
		if err != nil {
			t.Fatalf("%s local: %v", name, err)
		}
		got, err := rs.Posts(t.Context(), q)
		if err != nil {
			t.Fatalf("%s remote: %v", name, err)
		}
		if asJSON(t, got) != asJSON(t, want) {
			t.Errorf("%s: remote and local disagree\nlocal : %s\nremote: %s", name, asJSON(t, want), asJSON(t, got))
		}
	}

	posts, _ := ls.Posts(t.Context(), store.PostQuery{Limit: 50})
	for _, p := range posts {
		if p.Title == "Invisible" {
			t.Fatal("the fixture is wrong: an unfollowed feed's post is visible locally")
		}
	}

	type pair struct {
		name  string
		local func() (any, error)
		rem   func() (any, error)
	}
	ctx := t.Context()
	since := now.Add(-24 * time.Hour)
	pairs := []pair{
		{"bookmarked", func() (any, error) { return ls.BookmarkedPosts(ctx) }, func() (any, error) { return rs.BookmarkedPosts(ctx) }},
		{"read posts", func() (any, error) { return ls.ReadPosts(ctx) }, func() (any, error) { return rs.ReadPosts(ctx) }},
		{"bookmarked ids", func() (any, error) { ids, err := ls.BookmarkedIDs(ctx); return sorted(ids), err }, func() (any, error) { ids, err := rs.BookmarkedIDs(ctx); return sorted(ids), err }},
		{"read ids", func() (any, error) { ids, err := ls.ReadIDs(ctx); return sorted(ids), err }, func() (any, error) { ids, err := rs.ReadIDs(ctx); return sorted(ids), err }},
		{"subscriptions", func() (any, error) { return ls.Subscriptions(ctx) }, func() (any, error) { return rs.Subscriptions(ctx) }},
		{"stats", func() (any, error) { return ls.Stats(ctx, since) }, func() (any, error) { return rs.Stats(ctx, since) }},
		{"unread counts", func() (any, error) { return ls.UnreadCounts(ctx) }, func() (any, error) { return rs.UnreadCounts(ctx) }},
		{"post by url", func() (any, error) { return ls.PostByURL(ctx, "https://example.com/post/Async") }, func() (any, error) { return rs.PostByURL(ctx, "https://example.com/post/Async") }},
	}
	for _, p := range pairs {
		want, err := p.local()
		if err != nil {
			t.Fatalf("%s local: %v", p.name, err)
		}
		got, err := p.rem()
		if err != nil {
			t.Fatalf("%s remote: %v", p.name, err)
		}
		if asJSON(t, got) != asJSON(t, want) {
			t.Errorf("%s: remote and local disagree\nlocal : %s\nremote: %s", p.name, asJSON(t, want), asJSON(t, got))
		}
	}
}

func TestOneUserNeverSeesAnothersState(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)

	alice := register(t, srv, "alice")
	bob := register(t, srv, "bob")
	aliceUser, _ := alice.Me(t.Context())

	feed := db.Feed(t, aliceUser.ID, "Shared", "https://example.com/shared")
	db.Follow(t, aliceUser.ID, feed.ID)
	post := db.Post(t, feed.ID, "Hello", "https://example.com/hello", time.Now())

	if _, err := alice.Bookmark(t.Context(), post.ID); err != nil {
		t.Fatal(err)
	}
	if err := alice.SetRead(t.Context(), post.ID, true); err != nil {
		t.Fatal(err)
	}

	for name, fn := range map[string]func() (int, error){
		"posts": func() (int, error) {
			p, err := bob.Posts(t.Context(), store.PostQuery{Limit: 50})
			return len(p), err
		},
		"bookmarks": func() (int, error) {
			p, err := bob.BookmarkedPosts(t.Context())
			return len(p), err
		},
		"bookmarked ids": func() (int, error) {
			p, err := bob.BookmarkedIDs(t.Context())
			return len(p), err
		},
		"read ids": func() (int, error) {
			p, err := bob.ReadIDs(t.Context())
			return len(p), err
		},
		"subscriptions": func() (int, error) {
			p, err := bob.Subscriptions(t.Context())
			return len(p), err
		},
	} {
		n, err := fn()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n != 0 {
			t.Errorf("bob sees %d of alice's %s", n, name)
		}
	}
}

func TestWritesRoundTripAndAreIdempotent(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	rs := register(t, srv, "alice")
	me, _ := rs.Me(t.Context())

	feed := db.Feed(t, me.ID, "Blog", "https://example.com/feed")
	db.Follow(t, me.ID, feed.ID)
	p1 := db.Post(t, feed.ID, "One", "https://example.com/1", time.Now())
	p2 := db.Post(t, feed.ID, "Two", "https://example.com/2", time.Now())
	ctx := t.Context()

	created, err := rs.Bookmark(ctx, p1.ID)
	if err != nil || !created {
		t.Fatalf("first bookmark = %v, %v; want created", created, err)
	}
	created, err = rs.Bookmark(ctx, p1.ID)
	if err != nil || created {
		t.Fatalf("second bookmark = %v, %v; want a quiet no-op", created, err)
	}
	if err := rs.Unbookmark(ctx, p1.ID); err != nil {
		t.Fatal(err)
	}
	if err := rs.Unbookmark(ctx, p1.ID); err != nil {
		t.Fatalf("unbookmarking twice should not fail: %v", err)
	}

	for range 2 {
		if err := rs.SetRead(ctx, p1.ID, true); err != nil {
			t.Fatalf("marking read twice should not fail: %v", err)
		}
	}
	if err := rs.SetAllRead(ctx, []uuid.UUID{p1.ID, p2.ID}); err != nil {
		t.Fatal(err)
	}
	if ids, _ := rs.ReadIDs(ctx); len(ids) != 2 {
		t.Errorf("read ids = %d, want 2", len(ids))
	}
	if err := rs.SetRead(ctx, p1.ID, false); err != nil {
		t.Fatal(err)
	}
	if ids, _ := rs.ReadIDs(ctx); len(ids) != 1 {
		t.Errorf("after unread, read ids = %d, want 1", len(ids))
	}
	if err := rs.SetAllRead(ctx, nil); err != nil {
		t.Errorf("an empty batch should be a no-op: %v", err)
	}
}

func TestSubscriptionsFollowCategorizeAndUnfollow(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	rs := register(t, srv, "alice")
	ctx := t.Context()

	bob := db.User(t, "bob")
	feed := db.Feed(t, bob.ID, "Go Blog", "https://go.dev/blog/feed.atom")

	follow, created, err := rs.FollowURL(ctx, feed.Url)
	if err != nil || !created || follow.FeedName != "Go Blog" {
		t.Fatalf("follow = %+v, %v, %v", follow, created, err)
	}
	follow, created, err = rs.FollowURL(ctx, feed.Url)
	if err != nil || created || follow.FeedName != "Go Blog" {
		t.Fatalf("second follow = %+v, %v, %v; want it to still name the feed", follow, created, err)
	}

	if _, err := rs.Categorize(ctx, feed.Url, "dev"); err != nil {
		t.Fatalf("categorize: %v", err)
	}
	subs, _ := rs.Subscriptions(ctx)
	if len(subs) != 1 || subs[0].Category != "dev" {
		t.Errorf("subscriptions = %+v, want one in folder dev", subs)
	}

	if _, _, err := rs.FollowURL(ctx, "https://nowhere.example/feed"); !errors.Is(err, store.ErrFeedNotFound) {
		t.Errorf("following an unknown feed = %v, want ErrFeedNotFound", err)
	}
	if _, err := rs.UnfollowURL(ctx, "https://nowhere.example/feed"); !errors.Is(err, store.ErrFeedNotFound) {
		t.Errorf("unfollowing an unknown feed = %v, want ErrFeedNotFound", err)
	}

	other := db.Feed(t, bob.ID, "Other", "https://example.com/other")
	if _, err := rs.Categorize(ctx, other.Url, "x"); !errors.Is(err, store.ErrNotFollowed) {
		t.Errorf("categorizing an unfollowed feed = %v, want ErrNotFollowed", err)
	}

	if got, err := rs.UnfollowURL(ctx, feed.Url); err != nil || got.Name != "Go Blog" {
		t.Fatalf("unfollow = %+v, %v", got, err)
	}
	if subs, _ := rs.Subscriptions(ctx); len(subs) != 0 {
		t.Errorf("still following %d feeds", len(subs))
	}
}

func TestAddFeedFetchesOnTheServerAndNamesItFromTheTitle(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	rs := register(t, srv, "alice")

	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Served Title</title><link>http://x</link><description>d</description><item><title>Hi</title><link>http://x/1</link><description>b</description></item></channel></rss>`)
	}))
	defer feedSrv.Close()

	feed, created, err := rs.AddFeed(t.Context(), "", feedSrv.URL)
	if err != nil || !created || feed.Name != "Served Title" {
		t.Fatalf("add = %+v, %v, %v", feed, created, err)
	}
	if _, created, err = rs.AddFeed(t.Context(), "", feedSrv.URL); err != nil || created {
		t.Errorf("adding the same feed again = %v, %v; want a follow, not an error", created, err)
	}

	results := rs.AddFeeds(t.Context(), nil, nil)
	if len(results) != 0 {
		t.Errorf("no entries gave %d results", len(results))
	}
	if _, _, err := rs.AddFeed(t.Context(), "", "http://127.0.0.1:1/nothing"); err == nil {
		t.Error("adding an unreachable feed succeeded")
	}
}

func TestMalformedInputIsABadRequestNotAnInternalError(t *testing.T) {
	db := testdb.Open(t)
	srv := serve(t, db)
	rs := register(t, srv, "alice")
	_ = rs
	res, _ := raw(t, "POST", srv.URL+"/v1/tokens", "", nil)
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("empty body = %d, want 400", res.StatusCode)
	}
}
