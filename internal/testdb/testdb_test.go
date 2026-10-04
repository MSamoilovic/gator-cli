package testdb

import (
	"strings"
	"testing"
)

func TestCheckSafeAcceptsALocalTestDatabase(t *testing.T) {
	for _, dbURL := range []string{
		"postgres://postgres:pw@localhost:5433/gator_test?sslmode=disable",
		"postgres://postgres@127.0.0.1:5432/test",
		"postgres://postgres@[::1]:5432/my_test_db",
		"postgres:///gator_test",
	} {
		if err := checkSafe(dbURL); err != nil {
			t.Errorf("checkSafe(%q) = %v, want it accepted", dbURL, err)
		}
	}
}

func TestCheckSafeRefusesARemoteHost(t *testing.T) {
	for _, dbURL := range []string{
		"postgres://u:p@db.production.example/gator_test",
		"postgres://u:p@10.0.0.5:5432/gator_test",
		"postgres://u:p@ep-x.eu-central-1.aws.neon.tech/gator_test",
	} {
		if err := checkSafe(dbURL); err == nil {
			t.Errorf("checkSafe(%q) accepted a remote host", dbURL)
		}
	}
}

func TestCheckSafeRefusesADatabaseNotNamedTest(t *testing.T) {
	for _, dbURL := range []string{
		"postgres://postgres@localhost:5433/gator",
		"postgres://postgres@localhost:5433/production",
		"postgres://postgres@localhost:5433/",
	} {
		err := checkSafe(dbURL)
		if err == nil {
			t.Fatalf("checkSafe(%q) accepted a database whose name lacks \"test\"", dbURL)
		}
		if !strings.Contains(err.Error(), "test") {
			t.Errorf("checkSafe(%q) = %v, want the error to explain the naming rule", dbURL, err)
		}
	}
}

func TestSchemaNameIsAValidIdentifier(t *testing.T) {
	got := schemaName()

	if !strings.HasPrefix(got, "gator_test_") {
		t.Errorf("schemaName() = %q, want the gator_test_ prefix", got)
	}
	for _, r := range got {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			t.Errorf("schemaName() = %q contains %q, which needs quoting in SQL", got, r)
		}
	}
}

func TestUpSectionTakesOnlyTheUpHalf(t *testing.T) {
	got := upSection(`-- +goose Up
CREATE TABLE t (id INT);

-- +goose Down
DROP TABLE t;`)

	if !strings.Contains(got, "CREATE TABLE") {
		t.Errorf("upSection dropped the Up statements: %q", got)
	}
	if strings.Contains(got, "DROP TABLE") {
		t.Errorf("upSection leaked the Down statements: %q", got)
	}
}

func TestEveryMigrationHasAnUpSection(t *testing.T) {
	migrations, err := upSections()
	if err != nil {
		t.Fatalf("upSections: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("no migrations found")
	}

	for _, m := range migrations {
		if strings.TrimSpace(m.sql) == "" {
			t.Errorf("%s has an empty Up section", m.name)
		}
		if strings.Contains(m.sql, "+goose Down") {
			t.Errorf("%s leaked its Down section into the Up half", m.name)
		}
	}
}

func TestWithSearchPathAppendsCorrectly(t *testing.T) {
	cases := map[string]string{
		"postgres:///db?sslmode=disable": "postgres:///db?sslmode=disable&search_path=s",
		"postgres:///db":                 "postgres:///db?search_path=s",
	}
	for in, want := range cases {
		if got := withSearchPath(in, "s"); got != want {
			t.Errorf("withSearchPath(%q) = %q, want %q", in, got, want)
		}
	}
}
