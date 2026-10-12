package feeds

import (
	"context"
	"fmt"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"
)

const (
	DefaultRetention   = 72 * time.Hour
	DefaultKeepPerFeed = 10
)

func Prune(ctx context.Context, q *database.Queries, retention time.Duration, keepPerFeed int32) (int64, error) {
	if retention <= 0 {
		return 0, fmt.Errorf("retention must be positive, got %s", retention)
	}
	if keepPerFeed < 0 {
		return 0, fmt.Errorf("keepPerFeed must not be negative, got %d", keepPerFeed)
	}

	n, err := q.PrunePosts(ctx, database.PrunePostsParams{
		Before:      time.Now().Add(-retention),
		KeepPerFeed: keepPerFeed,
	})
	if err != nil {
		return 0, fmt.Errorf("pruning posts older than %s beyond the newest %d per feed: %w", retention, keepPerFeed, err)
	}
	return n, nil
}
