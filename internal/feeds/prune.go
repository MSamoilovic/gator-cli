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

type Policy struct {
	Retention   time.Duration
	KeepPerFeed int32
}

func DefaultPolicy() Policy {
	return Policy{Retention: DefaultRetention, KeepPerFeed: DefaultKeepPerFeed}
}

func (p Policy) valid() error {
	if p.Retention <= 0 {
		return fmt.Errorf("retention must be positive, got %s", p.Retention)
	}
	if p.KeepPerFeed < 0 {
		return fmt.Errorf("keepPerFeed must not be negative, got %d", p.KeepPerFeed)
	}
	return nil
}

func Prune(ctx context.Context, q *database.Queries, p Policy) (int64, error) {
	if err := p.valid(); err != nil {
		return 0, err
	}

	n, err := q.PrunePosts(ctx, database.PrunePostsParams{
		Before:      time.Now().Add(-p.Retention),
		KeepPerFeed: p.KeepPerFeed,
	})
	if err != nil {
		return 0, fmt.Errorf("pruning posts older than %s beyond the newest %d per feed: %w", p.Retention, p.KeepPerFeed, err)
	}
	return n, nil
}
