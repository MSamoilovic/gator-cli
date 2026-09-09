package feeds

import (
	"context"
	"fmt"
	"time"

	"gator-cli/internal/database"
)

const DefaultRetention = 72 * time.Hour

func Prune(ctx context.Context, q *database.Queries, retention time.Duration) (int64, error) {
	if retention <= 0 {
		return 0, fmt.Errorf("retention must be positive, got %s", retention)
	}

	n, err := q.PrunePosts(ctx, time.Now().Add(-retention))
	if err != nil {
		return 0, fmt.Errorf("pruning posts older than %s: %w", retention, err)
	}
	return n, nil
}
