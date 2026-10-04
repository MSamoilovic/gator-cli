package testdb

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gator-cli/internal/database"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const EnvURL = "GATOR_TEST_DB_URL"

var (
	once   sync.Once
	shared *sql.DB
	setErr error
)

type DB struct {
	*database.Queries
	SQL *sql.DB
}

func Open(t *testing.T) *DB {
	t.Helper()

	dbURL := os.Getenv(EnvURL)
	if dbURL == "" {
		t.Skipf("%s is not set", EnvURL)
	}

	once.Do(func() { shared, setErr = prepare(dbURL) })
	if setErr != nil {
		t.Fatalf("preparing the test database: %v", setErr)
	}

	db := &DB{Queries: database.New(shared), SQL: shared}
	if err := db.TruncateAll(t.Context()); err != nil {
		t.Fatalf("clearing tables before the test: %v", err)
	}
	t.Cleanup(func() { shared.Exec("TRUNCATE users, feeds, posts, feed_follows, bookmarks, post_reads") })

	return db
}

func prepare(dbURL string) (*sql.DB, error) {
	if err := checkSafe(dbURL); err != nil {
		return nil, err
	}

	admin, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}
	defer admin.Close()

	if err := admin.Ping(); err != nil {
		return nil, fmt.Errorf("connecting: %w", err)
	}

	schema := schemaName()

	if _, err := admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE"); err != nil {
		return nil, fmt.Errorf("dropping schema %s: %w", schema, err)
	}
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		return nil, fmt.Errorf("creating schema %s: %w", schema, err)
	}

	scoped, err := sql.Open("postgres", withSearchPath(dbURL, schema))
	if err != nil {
		return nil, err
	}

	migrations, err := upSections()
	if err != nil {
		return nil, err
	}
	for _, m := range migrations {
		if _, err := scoped.Exec(m.sql); err != nil {
			return nil, fmt.Errorf("applying %s: %w", m.name, err)
		}
	}
	return scoped, nil
}

func schemaName() string {
	base := strings.TrimSuffix(filepath.Base(os.Args[0]), ".test")

	var sb strings.Builder
	sb.WriteString("gator_test_")
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r + ('a' - 'A'))
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

func withSearchPath(dbURL, schema string) string {
	if strings.Contains(dbURL, "?") {
		return dbURL + "&search_path=" + schema
	}
	return dbURL + "?search_path=" + schema
}

func checkSafe(dbURL string) error {
	u, err := url.Parse(dbURL)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", EnvURL, err)
	}

	host := u.Hostname()
	if host != "localhost" && host != "" {
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("%s points at %s; refusing to use a non-local database", EnvURL, host)
		}
	}

	name := strings.TrimPrefix(u.Path, "/")
	if !strings.Contains(name, "test") {
		return fmt.Errorf("%s names database %q; refusing to use a database whose name does not contain \"test\"", EnvURL, name)
	}
	return nil
}

type migration struct {
	name string
	sql  string
}

func upSections() ([]migration, error) {
	dir, err := schemaDir()
	if err != nil {
		return nil, err
	}

	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no migrations found in %s", dir)
	}
	sort.Strings(paths)

	out := make([]migration, 0, len(paths))
	for _, p := range paths {
		body, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{name: filepath.Base(p), sql: upSection(string(body))})
	}
	return out, nil
}

func upSection(body string) string {
	_, after, found := strings.Cut(body, "-- +goose Up")
	if !found {
		return body
	}
	up, _, _ := strings.Cut(after, "-- +goose Down")
	return up
}

func schemaDir() (string, error) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate the testdb source directory")
	}
	return filepath.Join(filepath.Dir(self), "..", "..", "sql", "schema"), nil
}

func (db *DB) User(t *testing.T, name string) database.User {
	t.Helper()
	u, err := db.CreateUser(t.Context(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
	})
	if err != nil {
		t.Fatalf("creating user %q: %v", name, err)
	}
	return u
}

func (db *DB) Feed(t *testing.T, owner uuid.UUID, name, feedURL string) database.Feed {
	t.Helper()
	f, err := db.CreateFeed(t.Context(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
		Url:       feedURL,
		UserID:    uuid.NullUUID{UUID: owner, Valid: true},
	})
	if err != nil {
		t.Fatalf("creating feed %q: %v", name, err)
	}
	return f
}

func (db *DB) Follow(t *testing.T, userID, feedID uuid.UUID) {
	t.Helper()
	if _, err := db.CreateFeedFollow(t.Context(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    userID,
		FeedID:    feedID,
	}); err != nil {
		t.Fatalf("following feed: %v", err)
	}
}

func (db *DB) Post(t *testing.T, feedID uuid.UUID, title, postURL string, published time.Time) database.Post {
	t.Helper()
	p, err := db.CreatePost(t.Context(), database.CreatePostParams{
		ID:          uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       title,
		Url:         postURL,
		Description: sql.NullString{String: title + " body", Valid: true},
		PublishedAt: sql.NullTime{Time: published, Valid: true},
		FeedID:      feedID,
	})
	if err != nil {
		t.Fatalf("creating post %q: %v", title, err)
	}
	return p
}

func (db *DB) Bookmark(t *testing.T, userID, postID uuid.UUID) {
	t.Helper()
	if _, err := db.CreateBookmark(t.Context(), database.CreateBookmarkParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UserID:    userID,
		PostID:    postID,
	}); err != nil {
		t.Fatalf("bookmarking: %v", err)
	}
}

func (db *DB) MarkRead(t *testing.T, userID, postID uuid.UUID) {
	t.Helper()
	if err := db.MarkPostRead(t.Context(), database.MarkPostReadParams{
		UserID: userID,
		PostID: postID,
		ReadAt: time.Now(),
	}); err != nil {
		t.Fatalf("marking read: %v", err)
	}
}

func (db *DB) Count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.SQL.QueryRowContext(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

func (db *DB) Exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := db.SQL.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("executing %q: %v", query, err)
	}
}

func (db *DB) QueryRow(t *testing.T, dest any, query string, args ...any) {
	t.Helper()
	if err := db.SQL.QueryRowContext(t.Context(), query, args...).Scan(dest); err != nil {
		t.Fatalf("querying %q: %v", query, err)
	}
}
