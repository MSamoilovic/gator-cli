package credentials

import (
	"errors"
	"os"
	"testing"
)

func TestSaveThenLoadForTheSameServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Save(Credentials{ServerURL: "https://a.example", Token: "gat_x"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load("https://a.example")
	if err != nil || got != "gat_x" {
		t.Fatalf("Load = %q, %v; want gat_x", got, err)
	}
}

func TestLoadRefusesATokenSavedForAnotherServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	Save(Credentials{ServerURL: "https://a.example", Token: "gat_x"})

	if _, err := Load("https://b.example"); !errors.Is(err, ErrNoToken) {
		t.Errorf("Load for another server = %v, want ErrNoToken", err)
	}
}

func TestLoadWithoutAFileIsErrNoToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := Load("https://a.example"); !errors.Is(err, ErrNoToken) {
		t.Errorf("Load = %v, want ErrNoToken", err)
	}
}

func TestSavedFileIsPrivate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Save(Credentials{ServerURL: "https://a.example", Token: "gat_x"}); err != nil {
		t.Fatal(err)
	}
	path, _ := Path()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	Save(Credentials{ServerURL: "https://a.example", Token: "gat_x"})
	for range 2 {
		if err := Delete(); err != nil {
			t.Fatalf("Delete: %v", err)
		}
	}
	if _, err := Load("https://a.example"); !errors.Is(err, ErrNoToken) {
		t.Errorf("after Delete, Load = %v", err)
	}
}
