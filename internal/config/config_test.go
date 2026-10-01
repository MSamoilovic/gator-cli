package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteRoundTrip(t *testing.T) {
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

func TestWriteIsNotReadableByOthers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := write(Config{DBURL: "postgres://localhost/gator"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := os.Stat(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config permissions = %04o, want 0600", perm)
	}
}

func TestWriteTightensPermissionsOnAnExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, configFileName)

	if err := os.WriteFile(path, []byte(`{"db_url":"postgres://localhost/gator"}`), 0644); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	cfg, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if err := cfg.SetUsername("allan"); err != nil {
		t.Fatalf("SetUsername: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config permissions = %04o, want 0600", perm)
	}
}

func TestWriteLeavesNoTempFilesBehind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := write(Config{DBURL: "postgres://localhost/gator"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != configFileName {
			t.Errorf("write left %q behind", e.Name())
		}
	}
}

func TestSetUsername(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

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
	if reread.DBURL != "postgres://localhost/gator" {
		t.Errorf("DBURL changed: got %q", reread.DBURL)
	}
}
