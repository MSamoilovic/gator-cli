package database_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func tokenExpiryUp(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("../../sql/schema/019_token_expiry.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, up, _ := strings.Cut(string(body), "-- +goose Up")
	up, _, _ = strings.Cut(up, "-- +goose Down")
	return up
}

func TestTokensThatExistBeforeTheExpiryMigrationStayValidForNinetyDays(t *testing.T) {
	db := testdb.Open(t)
	up := tokenExpiryUp(t)
	alice := db.User(t, "alice")

	db.Exec(t, "ALTER TABLE api_tokens DROP COLUMN expires_at")
	migrated := false
	defer func() {
		if !migrated {
			db.Exec(t, up)
		}
	}()
	db.Exec(t, "INSERT INTO api_tokens (id, user_id, token_hash, created_at) VALUES ($1, $2, 'old-token', $3)",
		uuid.New(), alice.ID, time.Now().AddDate(-1, 0, 0))

	db.Exec(t, up)
	migrated = true

	if _, err := db.GetUserByTokenHash(t.Context(), "old-token"); err != nil {
		t.Fatalf("a token issued before the migration no longer signs in: %v", err)
	}
	var days float64
	db.QueryRow(t, &days, "SELECT extract(epoch FROM expires_at - now()) / 86400 FROM api_tokens WHERE token_hash = 'old-token'")
	if days < 89.9 || days > 90.1 {
		t.Errorf("a year-old token was given %.2f days, want 90 from the migration", days)
	}
}
