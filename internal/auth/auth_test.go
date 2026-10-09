package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordRoundTrips(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Error("VerifyPassword rejected the password it was hashed from")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Error("VerifyPassword accepted the wrong password")
	}
}

func TestHashPasswordRejectsOver72BytesButNotExactly72(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, bcrypt.ErrPasswordTooLong) {
		t.Errorf("HashPassword(73 bytes) = %v, want bcrypt.ErrPasswordTooLong", err)
	}
	if _, err := HashPassword(strings.Repeat("a", 72)); err != nil {
		t.Errorf("HashPassword(72 bytes) = %v, want success", err)
	}
}

func TestVerifyPasswordNeverMatchesTheLocalUserDefault(t *testing.T) {
	if VerifyPassword("", "anything") {
		t.Error("VerifyPassword matched against an empty hash")
	}
}

func TestGenerateTokenHasThePrefixAndIsUnique(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if !strings.HasPrefix(token, TokenPrefix) {
		t.Errorf("token %q lacks prefix %q", token, TokenPrefix)
	}

	other, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == other {
		t.Error("two calls to GenerateToken produced the same token")
	}
}

func TestHashTokenIsDeterministicAndDistinct(t *testing.T) {
	a, b := "gat_aaaaaaaaaaaaaaaaaaaa", "gat_bbbbbbbbbbbbbbbbbbbb"

	if HashToken(a) != HashToken(a) {
		t.Error("HashToken is not deterministic")
	}
	if HashToken(a) == HashToken(b) {
		t.Error("two different tokens hashed to the same value")
	}
}
