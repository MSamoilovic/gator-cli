package feeds

import (
	"context"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

func Forget(ctx context.Context, q *database.Queries, below int32, all bool) (int64, error) {
	if !all && below < 1 {
		return 0, fmt.Errorf("below must be positive unless every feed is meant, got %d", below)
	}

	n, err := q.ForgetFeedValidators(ctx, database.ForgetFeedValidatorsParams{
		AllFeeds: all,
		Below:    below,
	})
	if err != nil {
		return 0, fmt.Errorf("clearing conditional-GET validators: %w", err)
	}
	return n, nil
}
