package cli

import (
	"context"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/store"
)

func middlewareLoggedIn(handler func(context.Context, *state, command, store.User) error) handlerFunc {
	return func(ctx context.Context, s *state, cmd command) error {
		user, err := s.Store.Me(ctx)
		if err != nil {
			return fmt.Errorf("user not logged in: %w", err)
		}
		return handler(ctx, s, cmd, user)
	}
}
