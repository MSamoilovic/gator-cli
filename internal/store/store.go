package store

import (
	"context"
	"errors"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/feeds"

	"github.com/google/uuid"
)

type (
	User         = database.User
	Post         = database.Post
	Feed         = database.Feed
	Follow       = database.CreateFeedFollowRow
	Subscription = database.GetFeedFollowsForUserRow
	UnreadCount  = database.GetUnreadCountsForUserRow
	FeedStat     = database.GetFeedStatsForUserRow
)

type ReadState string

const (
	ReadAny    ReadState = "all"
	ReadUnread ReadState = "unread"
	ReadOnly   ReadState = "read"
)

func (r ReadState) Valid() bool {
	switch r {
	case "", ReadAny, ReadUnread, ReadOnly:
		return true
	}
	return false
}

type PostQuery struct {
	FeedID   uuid.UUID
	FeedName string
	Query    string
	Read     ReadState
	Since    time.Time
	SortDir  string
	Limit    int32
	Offset   int32
}

type SyncState struct {
	Online   bool
	LastSync time.Time
	Pending  int
	Dropped  int
}

type SyncResult struct {
	Sent    int
	Pending int
	Dropped int
	Posts   int
	Feeds   int
}

type Syncer interface {
	State() SyncState
	Sync(ctx context.Context) (SyncResult, error)
}

type RefreshResult struct {
	Feeds     int
	Saved     int
	Failed    int
	Unchanged int
}

var (
	ErrNoMoreText   = errors.New("the article has no more text than the feed already gave")
	ErrNotFollowed  = errors.New("you do not follow that feed")
	ErrFeedNotFound = errors.New("feed not found")
	ErrUnauthorized = errors.New("not signed in to this server")
	ErrPlanLimit    = errors.New("your plan does not allow that")
	ErrUnreachable  = errors.New("cannot reach the server")
	ErrNeverSynced  = errors.New("never synced with the server")
)

type Store interface {
	Me(ctx context.Context) (User, error)

	Posts(ctx context.Context, q PostQuery) ([]Post, error)
	BookmarkedPosts(ctx context.Context) ([]Post, error)
	PostByURL(ctx context.Context, url string) (Post, error)
	BookmarkedIDs(ctx context.Context) ([]uuid.UUID, error)
	ReadIDs(ctx context.Context) ([]uuid.UUID, error)
	UnreadCounts(ctx context.Context) ([]UnreadCount, error)
	FullText(ctx context.Context, post Post) (string, error)

	SetRead(ctx context.Context, postID uuid.UUID, read bool) error
	SetAllRead(ctx context.Context, postIDs []uuid.UUID) error
	Bookmark(ctx context.Context, postID uuid.UUID) (bool, error)
	Unbookmark(ctx context.Context, postID uuid.UUID) error

	Subscriptions(ctx context.Context) ([]Subscription, error)
	Stats(ctx context.Context, since time.Time) ([]FeedStat, error)
	AddFeed(ctx context.Context, name, url string) (Feed, bool, error)
	AddFeeds(ctx context.Context, entries []feeds.Entry, onResult func(feeds.AddResult)) []feeds.AddResult
	FollowURL(ctx context.Context, url string) (Follow, bool, error)
	UnfollowURL(ctx context.Context, url string) (Feed, error)
	Unfollow(ctx context.Context, feedID uuid.UUID) error
	Categorize(ctx context.Context, url, category string) (Feed, error)

	Refresh(ctx context.Context, onProgress func(feeds.Progress)) (RefreshResult, error)
}
