package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/store"
)

func middlewareLoggedIn(handler func(context.Context, *state, command, store.User) error) handlerFunc {
	return func(ctx context.Context, s *state, cmd command) error {
		user, err := s.Store.Me(ctx)
		if err != nil {
			return signInError(s, err)
		}
		return handler(ctx, s, cmd, user)
	}
}

func signInError(s *state, err error) error {
	server := ""
	if s.Cfg != nil {
		server = s.Cfg.ServerURL
	}
	if server == "" {
		return fmt.Errorf("user not logged in: %w", err)
	}

	switch {
	case errors.Is(err, store.ErrNeverSynced):
		return fmt.Errorf("never synced with %s — connect once so gator can work offline later", server)
	case errors.Is(err, store.ErrUnauthorized):
		return fmt.Errorf("not signed in to %s — run: gator login <your-name>", server)
	case errors.Is(err, store.ErrUnreachable):
		return fmt.Errorf("cannot reach %s — check the connection, or run gator logout to go back to the local database", server)
	}
	return fmt.Errorf("user not logged in: %w", err)
}
