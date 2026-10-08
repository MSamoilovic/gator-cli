package local

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/article"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/text"

	"github.com/google/uuid"
)

type Store struct {
	q        *database.Queries
	username string

	mu   sync.Mutex
	user store.User
}

var _ store.Store = (*Store)(nil)

func New(q *database.Queries, username string) *Store {
	return &Store{q: q, username: username}
}

func (s *Store) Me(ctx context.Context) (store.User, error) {
	return s.resolve(ctx)
}

func (s *Store) resolve(ctx context.Context) (store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.user.ID != uuid.Nil {
		return s.user, nil
	}

	user, err := s.q.GetUser(ctx, s.username)
	if err != nil {
		return store.User{}, err
	}
	s.user = user
	return user, nil
}

func (s *Store) Posts(ctx context.Context, q store.PostQuery) ([]store.Post, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}

	if q.Query != "" {
		return s.q.SearchPostsForUser(ctx, database.SearchPostsForUserParams{
			UserID:    user.ID,
			Query:     q.Query,
			PostLimit: q.Limit,
		})
	}

	return s.q.GetPostsForUserFiltered(ctx, database.GetPostsForUserFilteredParams{
		UserID:     user.ID,
		FeedID:     q.FeedID,
		FeedName:   q.FeedName,
		SortDir:    q.SortDir,
		UnreadOnly: q.UnreadOnly,
		Since:      q.Since,
		PostLimit:  q.Limit,
		PostOffset: q.Offset,
	})
}

func (s *Store) BookmarkedPosts(ctx context.Context) ([]store.Post, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetBookmarksForUser(ctx, user.ID)
}

func (s *Store) ReadPosts(ctx context.Context) ([]store.Post, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetReadPostsForUser(ctx, user.ID)
}

func (s *Store) PostByURL(ctx context.Context, url string) (store.Post, error) {
	return s.q.GetPostByUrl(ctx, url)
}

func (s *Store) BookmarkedIDs(ctx context.Context) ([]uuid.UUID, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetBookmarkedPostIDs(ctx, user.ID)
}

func (s *Store) ReadIDs(ctx context.Context) ([]uuid.UUID, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetReadPostIDs(ctx, user.ID)
}

func (s *Store) UnreadCounts(ctx context.Context) ([]store.UnreadCount, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetUnreadCountsForUser(ctx, user.ID)
}

func (s *Store) FullText(ctx context.Context, post store.Post) (string, error) {
	got, err := article.Fetch(ctx, post.Url)
	if err != nil {
		return "", err
	}

	if !article.Improves(got.Text, text.StripHTML(post.Description.String)) {
		return "", fmt.Errorf("%s: %w", post.Url, store.ErrNoMoreText)
	}

	if err := s.q.SetPostFullText(ctx, database.SetPostFullTextParams{
		ID:       post.ID,
		FullText: got.Text,
	}); err != nil {
		return "", fmt.Errorf("saving article text: %w", err)
	}
	return got.Text, nil
}

func (s *Store) SetRead(ctx context.Context, postID uuid.UUID, read bool) error {
	user, err := s.resolve(ctx)
	if err != nil {
		return err
	}

	if !read {
		return s.q.MarkPostUnread(ctx, database.MarkPostUnreadParams{
			UserID: user.ID,
			PostID: postID,
		})
	}

	return s.q.MarkPostRead(ctx, database.MarkPostReadParams{
		UserID: user.ID,
		PostID: postID,
		ReadAt: time.Now(),
	})
}

func (s *Store) SetAllRead(ctx context.Context, postIDs []uuid.UUID) error {
	if len(postIDs) == 0 {
		return nil
	}

	user, err := s.resolve(ctx)
	if err != nil {
		return err
	}

	return s.q.MarkPostsRead(ctx, database.MarkPostsReadParams{
		UserID:  user.ID,
		PostIds: postIDs,
		ReadAt:  time.Now(),
	})
}

func (s *Store) Bookmark(ctx context.Context, postID uuid.UUID) (bool, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return false, err
	}

	rows, err := s.q.CreateBookmark(ctx, database.CreateBookmarkParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UserID:    user.ID,
		PostID:    postID,
	})
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (s *Store) Unbookmark(ctx context.Context, postID uuid.UUID) error {
	user, err := s.resolve(ctx)
	if err != nil {
		return err
	}
	return s.q.DeleteBookmark(ctx, database.DeleteBookmarkParams{
		UserID: user.ID,
		PostID: postID,
	})
}

func (s *Store) Subscriptions(ctx context.Context) ([]store.Subscription, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetFeedFollowsForUser(ctx, user.ID)
}

func (s *Store) Stats(ctx context.Context, since time.Time) ([]store.FeedStat, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return s.q.GetFeedStatsForUser(ctx, database.GetFeedStatsForUserParams{
		UserID: user.ID,
		Since:  since,
	})
}

func (s *Store) AddFeed(ctx context.Context, name, url string) (store.Feed, bool, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return store.Feed{}, false, err
	}
	return feeds.Add(ctx, s.q, user.ID, name, url)
}

func (s *Store) AddFeeds(ctx context.Context, entries []feeds.Entry, onResult func(feeds.AddResult)) []feeds.AddResult {
	user, err := s.resolve(ctx)
	if err != nil {
		results := make([]feeds.AddResult, len(entries))
		for i, e := range entries {
			results[i] = feeds.AddResult{Entry: e, Err: err}
			if onResult != nil {
				onResult(results[i])
			}
		}
		return results
	}
	return feeds.AddMany(ctx, s.q, user.ID, entries, onResult)
}

func (s *Store) FollowURL(ctx context.Context, url string) (store.Follow, bool, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return store.Follow{}, false, err
	}

	feed, err := s.feedByURL(ctx, url)
	if err != nil {
		return store.Follow{}, false, err
	}

	follow, created, err := feeds.Follow(ctx, s.q, user.ID, feed.ID)
	if err != nil {
		return store.Follow{}, false, err
	}
	if !created {
		follow.FeedID, follow.FeedName = feed.ID, feed.Name
		follow.UserID, follow.UserName = user.ID, user.Name
	}
	return follow, created, nil
}

func (s *Store) feedByURL(ctx context.Context, url string) (store.Feed, error) {
	feed, err := s.q.GetFeedByUrl(ctx, url)
	if err != nil {
		return store.Feed{}, fmt.Errorf("%w: %w", store.ErrFeedNotFound, err)
	}
	return feed, nil
}

func (s *Store) UnfollowURL(ctx context.Context, url string) (store.Feed, error) {
	feed, err := s.feedByURL(ctx, url)
	if err != nil {
		return store.Feed{}, err
	}
	if err := s.Unfollow(ctx, feed.ID); err != nil {
		return store.Feed{}, err
	}
	return feed, nil
}

func (s *Store) Unfollow(ctx context.Context, feedID uuid.UUID) error {
	user, err := s.resolve(ctx)
	if err != nil {
		return err
	}
	return s.q.DeleteFeedFollow(ctx, database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feedID,
	})
}

func (s *Store) Categorize(ctx context.Context, url, category string) (store.Feed, error) {
	user, err := s.resolve(ctx)
	if err != nil {
		return store.Feed{}, err
	}

	feed, err := s.feedByURL(ctx, url)
	if err != nil {
		return store.Feed{}, err
	}

	follows, err := s.q.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return store.Feed{}, fmt.Errorf("error fetching follows: %w", err)
	}
	if !followsFeed(follows, feed.ID) {
		return feed, fmt.Errorf("%s: %w", feed.Name, store.ErrNotFollowed)
	}

	if err := s.q.SetFeedFollowCategory(ctx, database.SetFeedFollowCategoryParams{
		UserID:   user.ID,
		FeedID:   feed.ID,
		Category: category,
	}); err != nil {
		return store.Feed{}, fmt.Errorf("error setting category: %w", err)
	}
	return feed, nil
}

func followsFeed(rows []store.Subscription, feedID uuid.UUID) bool {
	for _, r := range rows {
		if r.FeedID == feedID {
			return true
		}
	}
	return false
}

func (s *Store) Refresh(ctx context.Context) (store.RefreshResult, error) {
	results, err := feeds.ScrapeAll(ctx, s.q, nil)
	if err != nil {
		return store.RefreshResult{}, err
	}

	out := store.RefreshResult{Feeds: len(results)}
	for _, r := range results {
		switch {
		case r.Err != nil:
			out.Failed++
		case r.NotModified:
			out.Unchanged++
		default:
			out.Saved += r.Saved
		}
	}
	return out, nil
}
