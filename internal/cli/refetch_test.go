package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func TestRefetchRejectsExtraArguments(t *testing.T) {
	err := handlerRefetch(t.Context(), &state{}, command{Name: "refetch", Args: []string{"all"}})
	if err == nil {
		t.Fatal("a positional argument was accepted, want the usage error")
	}
	if !strings.Contains(err.Error(), "usage: refetch") {
		t.Errorf("error = %q, want the usage line", err)
	}
}

func TestRefetchRejectsANonPositiveThreshold(t *testing.T) {
	err := handlerRefetch(t.Context(), &state{}, command{Name: "refetch", Args: []string{"--below", "0"}})
	if err == nil {
		t.Fatal("--below 0 was accepted; it would clear nothing and read as a no-op")
	}
	if strings.Contains(err.Error(), "usage:") {
		t.Errorf("error = %q, want the threshold complaint rather than the usage line", err)
	}
}

func TestRefetchIsRegisteredLocalOnlyAndWithoutLogin(t *testing.T) {
	for _, e := range allCommands() {
		if e.name != "refetch" {
			continue
		}
		if e.needsLogin() {
			t.Error("refetch was registered behind a login; feed validators are not per-user")
		}
		if !e.localOnly {
			t.Error("refetch writes to the global feeds table, so it must be localOnly or it would be offered against a server")
		}
		if e.run == nil {
			t.Error("refetch has no run handler")
		}
		if e.summary == "" {
			t.Error("refetch has no summary, so help and the picker would show a blank line")
		}
		return
	}
	t.Fatal("refetch is missing from allCommands(); help and the picker will not show it")
}

func TestRefetchAllForgetsAFullFeedAndSaysSo(t *testing.T) {
	db := testdb.Open(t)
	user := db.User(t, "alice")

	thin := db.Feed(t, user.ID, "thin", "https://thin.test/rss")
	full := db.Feed(t, user.ID, "full", "https://full.test/rss")
	for _, f := range []uuid.UUID{thin.ID, full.ID} {
		db.Follow(t, user.ID, f)
		db.Exec(t, `UPDATE feeds SET etag = 'W/"tag"' WHERE id = $1`, f)
	}
	for i := range 20 {
		db.Post(t, full.ID, fmt.Sprintf("p%d", i), fmt.Sprintf("https://full.test/%d", i), time.Now())
	}

	s := &state{Db: db.Queries}

	out, err := capture(t, func() error {
		return handlerRefetch(t.Context(), s, command{Name: "refetch", Args: nil})
	})
	if err != nil {
		t.Fatalf("refetch: %v", err)
	}
	if !strings.Contains(out, "gator agg") {
		t.Errorf("output = %q, want it to name the command that acts on the cleared validators", out)
	}
	if db.Count(t, "SELECT count(*) FROM feeds WHERE etag <> ''") != 1 {
		t.Error("the default run cleared the full feed too, which costs a body every round for nothing")
	}

	out, err = capture(t, func() error {
		return handlerRefetch(t.Context(), s, command{Name: "refetch", Args: []string{"--all"}})
	})
	if err != nil {
		t.Fatalf("refetch --all: %v", err)
	}
	if got := db.Count(t, "SELECT count(*) FROM feeds WHERE etag <> ''"); got != 0 {
		t.Errorf("%d feeds still remember an ETag after --all; the flag never reached feeds.Forget", got)
	}

	out, err = capture(t, func() error {
		return handlerRefetch(t.Context(), s, command{Name: "refetch", Args: []string{"--all"}})
	})
	if err != nil {
		t.Fatalf("second refetch --all: %v", err)
	}
	if !strings.Contains(out, "Nothing to forget") {
		t.Errorf("output = %q, want the no-op to say so rather than claim it cleared 0 feeds", out)
	}
}
