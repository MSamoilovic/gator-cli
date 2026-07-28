package main

import (
	"gator-cli/internal/database"
	"gator-cli/internal/tui"
)

func handlerTUI(s *state, _ command, user database.User) error {
	return tui.Run(s.Db, user.ID)
}
