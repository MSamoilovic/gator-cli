package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".gatorconfig.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestABadThemeIsRefusedAndNamesItsFile(t *testing.T) {
	path := writeConfig(t, `{"db_url":"postgres://nowhere/none","theme":"darkk"}`)

	_, _, err := open(nil)
	if err == nil {
		t.Fatal("open accepted an unknown theme")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not say which file to fix", err)
	}
	if !strings.Contains(err.Error(), "darkk") || !strings.Contains(err.Error(), "mono") {
		t.Errorf("error %q does not quote the bad value and the allowed ones", err)
	}
}

func TestAGoodThemeGetsPastTheConfigAndOnToTheDatabase(t *testing.T) {
	writeConfig(t, `{"db_url":"postgres://127.0.0.1:1/none?sslmode=disable&connect_timeout=1","theme":"mono"}`)

	_, _, err := open(nil)
	if err == nil {
		t.Skip("the throwaway database URL unexpectedly connected")
	}
	if strings.Contains(err.Error(), "theme") {
		t.Errorf("a valid theme was reported as an error: %v", err)
	}
}
