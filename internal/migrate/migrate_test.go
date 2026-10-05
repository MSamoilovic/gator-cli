package migrate

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

const schemaName = "gator_test_migrate"

func open(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := os.Getenv("GATOR_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("GATOR_TEST_DB_URL is not set")
	}

	admin, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer admin.Close()

	if err := admin.Ping(); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	if _, err := admin.Exec("DROP SCHEMA IF EXISTS " + schemaName + " CASCADE"); err != nil {
		t.Fatalf("dropping schema: %v", err)
	}
	if _, err := admin.Exec("CREATE SCHEMA " + schemaName); err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	sep := "?"
	if strings.Contains(dbURL, "?") {
		sep = "&"
	}
	db, err := sql.Open("postgres", dbURL+sep+"search_path="+schemaName)
	if err != nil {
		t.Fatalf("opening scoped: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpAppliesEveryMigrationThenIsIdempotent(t *testing.T) {
	db := open(t)
	fsys := os.DirFS("../..")

	first, err := Up(t.Context(), db, fsys)
	if err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if first.From != 0 {
		t.Errorf("starting version = %d, want 0 on a fresh schema", first.From)
	}
	if first.Applied() == 0 {
		t.Fatal("first Up applied nothing")
	}

	second, err := Up(t.Context(), db, fsys)
	if err != nil {
		t.Fatalf("second Up: %v", err)
	}
	if second.Applied() != 0 {
		t.Errorf("second Up applied %d migration(s), want 0", second.Applied())
	}
	if second.To != first.To {
		t.Errorf("version moved from %d to %d without applying anything", first.To, second.To)
	}
}

func TestUpCreatesTheTablesTheAppNeeds(t *testing.T) {
	db := open(t)

	if _, err := Up(t.Context(), db, os.DirFS("../..")); err != nil {
		t.Fatalf("Up: %v", err)
	}

	for _, table := range []string{"users", "feeds", "feed_follows", "posts", "bookmarks", "post_reads"} {
		var n int
		err := db.QueryRowContext(t.Context(),
			"SELECT count(*) FROM pg_tables WHERE schemaname = $1 AND tablename = $2", schemaName, table).Scan(&n)
		if err != nil {
			t.Fatalf("looking for %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %s was not created", table)
		}
	}
}

func TestPendingIsEmptyOnceUpHasRun(t *testing.T) {
	db := open(t)
	fsys := os.DirFS("../..")

	before, err := Pending(t.Context(), db, fsys)
	if err != nil {
		t.Fatalf("Pending before Up: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("a fresh schema reports no pending migrations")
	}

	if _, err := Up(t.Context(), db, fsys); err != nil {
		t.Fatalf("Up: %v", err)
	}

	after, err := Pending(t.Context(), db, fsys)
	if err != nil {
		t.Fatalf("Pending after Up: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("still pending after Up: %v", after)
	}
}

func TestVersionMatchesTheMigrationCount(t *testing.T) {
	db := open(t)
	fsys := os.DirFS("../..")

	pending, err := Pending(t.Context(), db, fsys)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	want := int64(len(pending))

	if _, err := Up(t.Context(), db, fsys); err != nil {
		t.Fatalf("Up: %v", err)
	}

	got, err := Version(t.Context(), db, fsys)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if got != want {
		t.Errorf("version = %d, want %d (one per migration file)", got, want)
	}
}

func TestUpRefusesWithoutASchema(t *testing.T) {
	if _, err := Up(t.Context(), nil, nil); err == nil {
		t.Error("Up with no embedded schema returned no error")
	}
}
