package cli

import (
	"context"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/tui"
)

func handlerTUI(ctx context.Context, s *state, _ command, user database.User) error {
	return tui.Run(ctx, s.Db, user)
}
