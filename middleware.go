package main

import (
	"context"
	"fmt"

	"gator-ci/internal/database"
)

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.Db.GetUser(context.Background(), s.Cfg.CurrentUserName)
		if err != nil {
			return fmt.Errorf("user not logged in: %v", err)
		}
		return handler(s, cmd, user)
	}
}
