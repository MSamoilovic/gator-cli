package cli

import (
	"context"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

func middlewareLoggedIn(handler func(context.Context, *state, command, database.User) error) handlerFunc {
	return func(ctx context.Context, s *state, cmd command) error {
		user, err := s.Db.GetUser(ctx, s.Cfg.CurrentUserName)
		if err != nil {
			return fmt.Errorf("user not logged in: %w", err)
		}
		return handler(ctx, s, cmd, user)
	}
}
