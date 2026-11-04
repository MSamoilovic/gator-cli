package cli

import (
	"strings"
	"testing"
)

func TestRegisterRefusesToCreateAUserWithoutAPassword(t *testing.T) {
	_, err := capture(t, func() error {
		return handlerRegister(t.Context(), &state{}, command{Name: "register", Args: []string{"--email=pera@example.com", "pera"}})
	})
	if err == nil || !strings.Contains(err.Error(), "password is required") {
		t.Fatalf("register without a password: err = %v, want a password-is-required error", err)
	}
}
