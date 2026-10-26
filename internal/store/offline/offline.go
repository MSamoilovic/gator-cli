package offline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

type Store struct {
	inner  store.Store
	server string
	path   string

	mu     sync.Mutex
	snap   snapshot
	online bool
}

type State struct {
	Online   bool
	LastSync time.Time
}

var _ store.Store = (*Store)(nil)

func Open(inner store.Store, server string) (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("finding the home directory: %w", err)
	}
	return Wrap(inner, server, home), nil
}

func Wrap(inner store.Store, server, dir string) *Store {
	path := filepath.Join(dir, FileName)
	return &Store{
		inner:  inner,
		server: server,
		path:   path,
		snap:   load(path, server),
		online: true,
	}
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return State{Online: s.online, LastSync: s.snap.SyncedAt}
}

func (s *Store) answered(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}

	reached := !errors.Is(err, store.ErrUnreachable)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.online = reached
	return reached
}

func (s *Store) update(fn func(*snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn(&s.snap)
	s.snap.Server = s.server
	s.snap.SyncedAt = time.Now()
	_ = s.snap.save(s.path)
}

func (s *Store) cached() snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.snap
}

func (s *Store) Me(ctx context.Context) (store.User, error) {
	user, err := s.inner.Me(ctx)
	if s.answered(ctx, err) {
		if err != nil {
			return store.User{}, err
		}
		s.update(func(snap *snapshot) {
			cached := wire.FromUser(user)
			snap.User = &cached
		})
		return user, nil
	}

	snap := s.cached()
	if snap.User == nil {
		return store.User{}, fmt.Errorf("never synced with %s: %w: %w", s.server, store.ErrNeverSynced, store.ErrUnreachable)
	}
	return snap.User.ToDatabase(), nil
}

func (s *Store) Posts(ctx context.Context, q store.PostQuery) ([]store.Post, error) {
	posts, err := s.inner.Posts(ctx, q)
	if s.answered(ctx, err) {
		if err != nil {
			return nil, err
		}
		s.update(func(snap *snapshot) { snap.mergePosts(posts) })
		return posts, nil
	}
	return s.cached().selectPosts(q), nil
}

func (s *Store) BookmarkedPosts(ctx context.Context) ([]store.Post, error) {
	posts, err := s.inner.BookmarkedPosts(ctx)
	if s.answered(ctx, err) {
		if err != nil {
			return nil, err
		}
		s.update(func(snap *snapshot) { snap.Bookmarked = wire.FromPosts(posts) })
		return posts, nil
	}
	return wire.ToPosts(s.cached().Bookmarked), nil
}

func (s *Store) PostByURL(ctx context.Context, url string) (store.Post, error) {
	post, err := s.inner.PostByURL(ctx, url)
	if s.answered(ctx, err) {
		if err != nil {
			return store.Post{}, err
		}
		s.update(func(snap *snapshot) { snap.mergePosts([]store.Post{post}) })
		return post, nil
	}

	snap := s.cached()
	for _, cached := range append(wire.ToPosts(snap.Posts), wire.ToPosts(snap.Bookmarked)...) {
		if cached.Url == url {
			return cached, nil
		}
	}
	return store.Post{}, fmt.Errorf("%s is not in the offline cache: %w", url, store.ErrUnreachable)
}

func (s *Store) BookmarkedIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := s.inner.BookmarkedIDs(ctx)
	if s.answered(ctx, err) {
		if err != nil {
			return nil, err
		}
		s.update(func(snap *snapshot) { snap.Saved = ids })
		return ids, nil
	}
	return s.cached().Saved, nil
}

func (s *Store) ReadIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := s.inner.ReadIDs(ctx)
	if s.answered(ctx, err) {
		if err != nil {
			return nil, err
		}
		s.update(func(snap *snapshot) { snap.Read = ids })
		return ids, nil
	}
	return s.cached().Read, nil
}

func (s *Store) UnreadCounts(ctx context.Context) ([]store.UnreadCount, error) {
	counts, err := s.inner.UnreadCounts(ctx)
	if s.answered(ctx, err) {
		if err != nil {
			return nil, err
		}
		s.update(func(snap *snapshot) { snap.Unread = counts })
		return counts, nil
	}
	return s.cached().Unread, nil
}

func (s *Store) Subscriptions(ctx context.Context) ([]store.Subscription, error) {
	subs, err := s.inner.Subscriptions(ctx)
	if s.answered(ctx, err) {
		if err != nil {
			return nil, err
		}
		s.update(func(snap *snapshot) { snap.Subs = subs })
		return subs, nil
	}
	return s.cached().Subs, nil
}

func (s *Store) FullText(ctx context.Context, post store.Post) (string, error) {
	text, err := s.inner.FullText(ctx, post)
	s.answered(ctx, err)
	return text, err
}

func (s *Store) Stats(ctx context.Context, since time.Time) ([]store.FeedStat, error) {
	stats, err := s.inner.Stats(ctx, since)
	s.answered(ctx, err)
	return stats, err
}

func (s *Store) SetRead(ctx context.Context, postID uuid.UUID, read bool) error {
	err := s.inner.SetRead(ctx, postID, read)
	s.answered(ctx, err)
	return err
}

func (s *Store) SetAllRead(ctx context.Context, postIDs []uuid.UUID) error {
	err := s.inner.SetAllRead(ctx, postIDs)
	s.answered(ctx, err)
	return err
}

func (s *Store) Bookmark(ctx context.Context, postID uuid.UUID) (bool, error) {
	created, err := s.inner.Bookmark(ctx, postID)
	s.answered(ctx, err)
	return created, err
}

func (s *Store) Unbookmark(ctx context.Context, postID uuid.UUID) error {
	err := s.inner.Unbookmark(ctx, postID)
	s.answered(ctx, err)
	return err
}

func (s *Store) AddFeed(ctx context.Context, name, url string) (store.Feed, bool, error) {
	feed, created, err := s.inner.AddFeed(ctx, name, url)
	s.answered(ctx, err)
	return feed, created, err
}

func (s *Store) AddFeeds(ctx context.Context, entries []feeds.Entry, onResult func(feeds.AddResult)) []feeds.AddResult {
	return s.inner.AddFeeds(ctx, entries, onResult)
}

func (s *Store) FollowURL(ctx context.Context, url string) (store.Follow, bool, error) {
	follow, created, err := s.inner.FollowURL(ctx, url)
	s.answered(ctx, err)
	return follow, created, err
}

func (s *Store) UnfollowURL(ctx context.Context, url string) (store.Feed, error) {
	feed, err := s.inner.UnfollowURL(ctx, url)
	s.answered(ctx, err)
	return feed, err
}

func (s *Store) Unfollow(ctx context.Context, feedID uuid.UUID) error {
	err := s.inner.Unfollow(ctx, feedID)
	s.answered(ctx, err)
	return err
}

func (s *Store) Categorize(ctx context.Context, url, category string) (store.Feed, error) {
	feed, err := s.inner.Categorize(ctx, url, category)
	s.answered(ctx, err)
	return feed, err
}

func (s *Store) Refresh(ctx context.Context, onProgress func(feeds.Progress)) (store.RefreshResult, error) {
	return s.inner.Refresh(ctx, onProgress)
}
