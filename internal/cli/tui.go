package cli

import (
	"context"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/tui"
)

func handlerTUI(ctx context.Context, s *state, _ command, user store.User) error {
	return tui.Run(ctx, s.Store, user)
}
