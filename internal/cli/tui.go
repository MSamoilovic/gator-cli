package cli

import (
	"context"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/tui"
)

func handlerTUI(s *state, _ command, user database.User) error {
	return tui.Run(context.Background(), s.Db, user)
}
