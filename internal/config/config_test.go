package config

import (
	"testing"
)

func TestReadWriteRoundTrip(t *testing.T) {
	// Izolujemo HOME na privremeni direktorijum da ne diramo pravi ~/.gatorconfig.json
	t.Setenv("HOME", t.TempDir())

	want := Config{
		DBURL:           "postgres://localhost:5432/gator",
		CurrentUserName: "marko",
	}
	if err := write(want); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got != want {
		t.Errorf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSetUsername(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Pocetni config na disku
	if err := write(Config{DBURL: "postgres://localhost/gator"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if err := cfg.SetUsername("allan"); err != nil {
		t.Fatalf("SetUsername: %v", err)
	}

	// SetUsername mora i da izmeni struct i da upise na disk
	if cfg.CurrentUserName != "allan" {
		t.Errorf("in-memory username = %q, want %q", cfg.CurrentUserName, "allan")
	}

	reread, err := Read()
	if err != nil {
		t.Fatalf("re-Read: %v", err)
	}
	if reread.CurrentUserName != "allan" {
		t.Errorf("persisted username = %q, want %q", reread.CurrentUserName, "allan")
	}
	// DBURL mora ostati netaknut
	if reread.DBURL != "postgres://localhost/gator" {
		t.Errorf("DBURL changed: got %q", reread.DBURL)
	}
}
