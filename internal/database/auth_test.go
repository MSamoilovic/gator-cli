package database_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/testdb"

	"github.com/google/uuid"
)

func TestRegisteredUserDefaultsToTheLocalOnlyPasswordHash(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")

	if alice.PasswordHash != "" {
		t.Errorf("password_hash for a user created without credentials = %q, want empty", alice.PasswordHash)
	}
	if alice.Email.Valid {
		t.Errorf("email for a user created without credentials = %v, want NULL", alice.Email)
	}
}

func TestEmailUniquenessIsCaseInsensitive(t *testing.T) {
	db := testdb.Open(t)

	_, err := db.CreateUserWithCredentials(t.Context(), database.CreateUserWithCredentialsParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      "alice",
		Email:     sql.NullString{String: "Alice@Example.com", Valid: true},
	})
	if err != nil {
		t.Fatalf("creating the first user: %v", err)
	}

	_, err = db.CreateUserWithCredentials(t.Context(), database.CreateUserWithCredentialsParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      "bob",
		Email:     sql.NullString{String: "alice@example.com", Valid: true},
	})
	if err == nil {
		t.Error("a second user registered with the same email in different case, want a unique violation")
	}

	got, err := db.GetUserByEmail(t.Context(), "ALICE@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.Name != "alice" {
		t.Errorf("GetUserByEmail(different case) found %q, want %q", got.Name, "alice")
	}
}

func TestMultipleUsersWithoutAnEmailAreAllowed(t *testing.T) {
	db := testdb.Open(t)

	db.User(t, "alice")
	db.User(t, "bob")
}

func TestDeletingAUserTakesTheirAPITokensWithIt(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")
	_, err := db.CreateAPIToken(t.Context(), database.CreateAPITokenParams{
		ID:        uuid.New(),
		UserID:    alice.ID,
		TokenHash: "deadbeef",
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	db.Exec(t, "DELETE FROM users WHERE id = $1", alice.ID)

	if n := db.Count(t, "SELECT count(*) FROM api_tokens WHERE user_id = $1", alice.ID); n != 0 {
		t.Errorf("api_tokens left after deleting their owner = %d, want 0", n)
	}
}

func TestGetUserByTokenHashResolvesTheOwner(t *testing.T) {
	db := testdb.Open(t)

	alice := db.User(t, "alice")
	db.User(t, "bob")

	if _, err := db.CreateAPIToken(t.Context(), database.CreateAPITokenParams{
		ID:        uuid.New(),
		UserID:    alice.ID,
		TokenHash: "alice-token-hash",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	got, err := db.GetUserByTokenHash(t.Context(), "alice-token-hash")
	if err != nil {
		t.Fatalf("GetUserByTokenHash: %v", err)
	}
	if got.ID != alice.ID {
		t.Errorf("GetUserByTokenHash resolved %q, want alice", got.Name)
	}
}
