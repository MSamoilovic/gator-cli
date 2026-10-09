package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

func TestLocalDatabaseNameAcceptsLoopback(t *testing.T) {
	for _, dbURL := range []string{
		"postgres://postgres:pw@localhost:5433/gator?sslmode=disable",
		"postgres://postgres@127.0.0.1:5432/gator",
		"postgres://postgres@[::1]:5432/gator",
		"postgres:///gator",
	} {
		name, err := localDatabaseName(dbURL)
		if err != nil {
			t.Errorf("localDatabaseName(%q) = error %v, want it accepted", dbURL, err)
			continue
		}
		if name != "gator" {
			t.Errorf("localDatabaseName(%q) = %q, want %q", dbURL, name, "gator")
		}
	}
}

func TestLocalDatabaseNameRefusesRemoteHosts(t *testing.T) {
	for _, dbURL := range []string{
		"postgres://user:pw@db.production.example:5432/gator",
		"postgres://user:pw@10.0.0.5:5432/gator",
		"postgres://user:pw@192.168.1.20:5432/gator",
		"postgres://user:pw@ep-cool-name.eu-central-1.aws.neon.tech/gator",
	} {
		if _, err := localDatabaseName(dbURL); err == nil {
			t.Errorf("localDatabaseName(%q) accepted a remote host", dbURL)
		}
	}
}

func TestLocalDatabaseNameRefusesAUrlWithNoDatabase(t *testing.T) {
	if _, err := localDatabaseName("postgres://localhost:5432/"); err == nil {
		t.Error("a db_url naming no database was accepted")
	}
}

func TestConfirmAcceptsTheExactDatabaseName(t *testing.T) {
	var out strings.Builder
	if err := confirm(strings.NewReader("gator\n"), &out, "gator"); err != nil {
		t.Errorf("confirm rejected the correct name: %v", err)
	}
	if !strings.Contains(out.String(), "gator") {
		t.Errorf("prompt does not name the database: %q", out.String())
	}
}

func TestConfirmRejectsAnythingElse(t *testing.T) {
	for _, typed := range []string{"y\n", "yes\n", "\n", "Gator\n", "gator_prod\n", "gato\n", ""} {
		var out strings.Builder
		err := confirm(strings.NewReader(typed), &out, "gator")
		if !errors.Is(err, errResetAborted) {
			t.Errorf("confirm(%q) = %v, want errResetAborted", typed, err)
		}
	}
}

func TestConfirmToleratesAMissingNewline(t *testing.T) {
	var out strings.Builder
	if err := confirm(strings.NewReader("gator"), &out, "gator"); err != nil {
		t.Errorf("confirm rejected input without a trailing newline: %v", err)
	}
}

func TestTotalCountsEveryTable(t *testing.T) {
	got := total(database.CountAllRowsRow{
		Users:       1,
		Feeds:       2,
		Posts:       4,
		FeedFollows: 8,
		Bookmarks:   16,
		PostReads:   32,
		ApiTokens:   64,
	})
	if got != 127 {
		t.Errorf("total = %d, want 127 — a table is missing from the sum", got)
	}
}

func TestPrintCountsNamesEveryTable(t *testing.T) {
	var out strings.Builder
	printCounts(&out, "gator", database.CountAllRowsRow{Users: 3, Feeds: 160, Posts: 524})

	for _, want := range []string{
		"gator", "users", "feeds", "posts", "feed_follows", "bookmarks", "post_reads", "api_tokens", "160", "524",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("printCounts output does not mention %q:\n%s", want, out.String())
		}
	}
}
