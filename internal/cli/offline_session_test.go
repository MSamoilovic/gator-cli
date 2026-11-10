package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/store/offline"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

const offlineServer = "https://gator.example.test"

func leaveOfflineData(t *testing.T, home, owner string) {
	t.Helper()

	cache := `{"server":"` + offlineServer + `","owner":"` + owner + `"}`
	queue := `{"op":"read","post_id":"` + uuid.NewString() + `","value":true,"at":"2026-11-10T10:00:00Z"}` + "\n"
	for name, body := range map[string]string{offline.FileName: cache, offline.QueueFileName: queue} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func offlineFilesLeft(t *testing.T, home string) []string {
	t.Helper()

	var left []string
	for _, name := range []string{offline.FileName, offline.QueueFileName} {
		_, err := os.Stat(filepath.Join(home, name))
		switch {
		case err == nil:
			left = append(left, name)
		case !errors.Is(err, os.ErrNotExist):
			t.Fatal(err)
		}
	}
	return left
}

func TestSigningInAsSomeoneElseDropsThePreviousAccountsOfflineData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	leaveOfflineData(t, home, "alice")

	if _, err := capture(t, func() error {
		return saveSession(offlineServer, wire.TokenResponse{Token: "gat_bob", User: wire.User{Name: "bob"}})
	}); err != nil {
		t.Fatal(err)
	}
	if left := offlineFilesLeft(t, home); len(left) != 0 {
		t.Errorf("bob's session kept alice's %v", left)
	}
}

func TestSigningInAgainAsTheSameUserKeepsTheQueue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	leaveOfflineData(t, home, "alice")

	if _, err := capture(t, func() error {
		return saveSession(offlineServer, wire.TokenResponse{Token: "gat_alice", User: wire.User{Name: "alice"}})
	}); err != nil {
		t.Fatal(err)
	}
	if left := offlineFilesLeft(t, home); len(left) != 2 {
		t.Errorf("signing in again left only %v; the queued change would be lost", left)
	}
}

func TestLogoutLeavesNoOfflineDataBehind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	leaveOfflineData(t, home, "alice")
	cfg := config.Config{ServerURL: offlineServer, CurrentUserName: "alice"}
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}

	if _, err := capture(t, func() error { return handlerLogout(t.Context(), nil, command{Name: "logout"}) }); err != nil {
		t.Fatal(err)
	}
	if left := offlineFilesLeft(t, home); len(left) != 0 {
		t.Errorf("logout left %v behind", left)
	}
}
