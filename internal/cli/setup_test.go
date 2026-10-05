package cli

import (
	"strings"
	"testing"

	"gator-cli/internal/migrate"
)

func TestResolveDBURLPrefersTheFlag(t *testing.T) {
	var out strings.Builder

	if got := resolveDBURL("  postgres://given/db  ", strings.NewReader(""), &out); got != "postgres://given/db" {
		t.Errorf("got %q, want the trimmed flag value", got)
	}
	if out.Len() != 0 {
		t.Errorf("prompted even though --db-url was given: %q", out.String())
	}
}

func TestResolveDBURLTakesWhatWasTyped(t *testing.T) {
	var out strings.Builder

	if got := resolveDBURL("", strings.NewReader("postgres://typed/db\n"), &out); got != "postgres://typed/db" {
		t.Errorf("got %q, want the typed value", got)
	}
	if !strings.Contains(out.String(), defaultDBURL) {
		t.Errorf("prompt does not show the default: %q", out.String())
	}
}

func TestResolveDBURLFallsBackToTheDefaultOnEmptyInput(t *testing.T) {
	var out strings.Builder

	for _, typed := range []string{"\n", "   \n", ""} {
		if got := resolveDBURL("", strings.NewReader(typed), &out); got != defaultDBURL {
			t.Errorf("resolveDBURL with %q = %q, want the default", typed, got)
		}
	}
}

func TestDescribeTargetNamesTheDatabase(t *testing.T) {
	if got := describeTarget("postgres://u:p@localhost:5433/gator?sslmode=disable"); got != "gator" {
		t.Errorf("describeTarget = %q, want %q", got, "gator")
	}
}

func TestDescribeTargetSurvivesAnUnparseableURL(t *testing.T) {
	for _, bad := range []string{"", "://", "postgres://localhost/"} {
		if got := describeTarget(bad); got == "" {
			t.Errorf("describeTarget(%q) returned an empty description", bad)
		}
	}
}

func TestMigrateResultCountsWhatItApplied(t *testing.T) {
	cases := []struct {
		res  migrate.Result
		want int64
	}{
		{migrate.Result{From: 0, To: 14}, 14},
		{migrate.Result{From: 12, To: 14}, 2},
		{migrate.Result{From: 14, To: 14}, 0},
	}
	for _, c := range cases {
		if got := c.res.Applied(); got != c.want {
			t.Errorf("Result{%d,%d}.Applied() = %d, want %d", c.res.From, c.res.To, got, c.want)
		}
	}
}

func TestSetupCommandsAreReachableWithoutAConfig(t *testing.T) {
	e, ok := lookup("init")
	if !ok {
		t.Fatal("init is not in the command table")
	}
	if !e.noDB {
		t.Error("init requires a database, but it is what creates one")
	}
	if !e.guest {
		t.Error("init is not offered to a guest, but nobody is logged in before it runs")
	}

	if e, ok = lookup("migrate"); !ok {
		t.Fatal("migrate is not in the command table")
	}
	if e.noDB {
		t.Error("migrate is marked noDB, but it needs the connection from the config")
	}
	if e.needsLogin() {
		t.Error("migrate requires a logged-in user, but the schema is not per-user")
	}
}
