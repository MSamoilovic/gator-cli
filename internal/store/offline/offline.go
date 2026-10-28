package offline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

	mu       sync.Mutex
	snap     snapshot
	queue    *queue
	online   bool
	flushing bool
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
		queue:  loadQueue(filepath.Join(dir, QueueFileName)),
		online: true,
	}
}

func (s *Store) State() store.SyncState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return store.SyncState{
		Online:   s.online,
		LastSync: s.snap.SyncedAt,
		Pending:  len(s.queue.pending),
		Dropped:  s.snap.Dropped,
	}
}

func (s *Store) Sync(ctx context.Context) (store.SyncResult, error) {
	res := store.SyncResult{Sent: s.drain(ctx)}

	state := s.State()
	res.Pending, res.Dropped = state.Pending, state.Dropped

	if _, err := s.Me(ctx); err != nil {
		return res, err
	}
	subs, err := s.Subscriptions(ctx)
	if err != nil {
		return res, err
	}
	if _, err := s.Posts(ctx, store.PostQuery{Limit: maxPosts}); err != nil {
		return res, err
	}
	if _, err := s.BookmarkedPosts(ctx); err != nil {
		return res, err
	}
	if _, err := s.BookmarkedIDs(ctx); err != nil {
		return res, err
	}
	if _, err := s.ReadIDs(ctx); err != nil {
		return res, err
	}
	if _, err := s.UnreadCounts(ctx); err != nil {
		return res, err
	}

	if !s.State().Online {
		return res, fmt.Errorf("cannot reach %s: %w", s.server, store.ErrUnreachable)
	}

	s.mu.Lock()
	res.Feeds, res.Posts = len(subs), len(s.snap.Posts)
	s.snap.Dropped = 0
	_ = s.snap.save(s.path)
	s.mu.Unlock()

	return res, nil
}

func (s *Store) answered(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}

	reached := !errors.Is(err, store.ErrUnreachable)

	s.mu.Lock()
	s.online = reached
	s.mu.Unlock()

	if reached && err == nil {
		s.drain(ctx)
	}
	return reached
}

func (s *Store) drain(ctx context.Context) int {
	s.mu.Lock()
	owed := len(s.queue.pending) > 0 && !s.flushing
	s.flushing = s.flushing || owed
	s.mu.Unlock()

	if !owed {
		return 0
	}

	done := s.flush(ctx)

	s.mu.Lock()
	s.flushing = false
	s.mu.Unlock()
	return done
}

func (s *Store) flush(ctx context.Context) int {
	s.mu.Lock()
	pending := slices.Clone(s.queue.pending)
	s.mu.Unlock()

	done := 0
	for _, c := range pending {
		if ctx.Err() != nil {
			break
		}
		err := s.send(ctx, c)
		if err != nil && !gone(err) {
			if errors.Is(err, store.ErrUnreachable) {
				s.mu.Lock()
				s.online = false
				s.mu.Unlock()
			}
			break
		}
		done++
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.queue.sent(done)
	return done
}

func (s *Store) send(ctx context.Context, c change) error {
	switch c.Op {
	case opRead:
		return s.inner.SetRead(ctx, c.PostID, c.Value)
	case opReadAll:
		return s.inner.SetAllRead(ctx, c.PostIDs)
	case opBookmark:
		if !c.Value {
			return s.inner.Unbookmark(ctx, c.PostID)
		}
		_, err := s.inner.Bookmark(ctx, c.PostID)
		return err
	case opCategorize:
		_, err := s.inner.Categorize(ctx, c.URL, c.Category)
		return err
	}
	return nil
}

func gone(err error) bool {
	return errors.Is(err, store.ErrFeedNotFound) || errors.Is(err, store.ErrNotFollowed)
}

func (s *Store) enqueue(c change) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dropped, err := s.queue.add(c)
	if err != nil {
		return fmt.Errorf("saving that change until %s answers: %w", s.server, err)
	}
	s.snap.Dropped += dropped
	s.snap.apply(c)
	_ = s.snap.save(s.path)
	return nil
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
	if s.answered(ctx, err) {
		return err
	}
	return s.enqueue(change{Op: opRead, PostID: postID, Value: read, At: time.Now()})
}

func (s *Store) SetAllRead(ctx context.Context, postIDs []uuid.UUID) error {
	err := s.inner.SetAllRead(ctx, postIDs)
	if s.answered(ctx, err) {
		return err
	}
	return s.enqueue(change{Op: opReadAll, PostIDs: postIDs, At: time.Now()})
}

func (s *Store) Bookmark(ctx context.Context, postID uuid.UUID) (bool, error) {
	created, err := s.inner.Bookmark(ctx, postID)
	if s.answered(ctx, err) {
		return created, err
	}

	saved := slices.Contains(s.cached().Saved, postID)
	if err := s.enqueue(change{Op: opBookmark, PostID: postID, Value: true, At: time.Now()}); err != nil {
		return false, err
	}
	return !saved, nil
}

func (s *Store) Unbookmark(ctx context.Context, postID uuid.UUID) error {
	err := s.inner.Unbookmark(ctx, postID)
	if s.answered(ctx, err) {
		return err
	}
	return s.enqueue(change{Op: opBookmark, PostID: postID, At: time.Now()})
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
	if s.answered(ctx, err) {
		return feed, err
	}

	subs := s.cached().Subs
	i := slices.IndexFunc(subs, func(sub store.Subscription) bool { return sub.FeedUrl == url })
	if i < 0 {
		return store.Feed{Url: url, Name: url}, fmt.Errorf("%s: %w", url, store.ErrNotFollowed)
	}

	if err := s.enqueue(change{Op: opCategorize, URL: url, Category: category, At: time.Now()}); err != nil {
		return store.Feed{}, err
	}
	return store.Feed{ID: subs[i].FeedID, Name: subs[i].FeedName, Url: url}, nil
}

func (s *Store) Refresh(ctx context.Context, onProgress func(feeds.Progress)) (store.RefreshResult, error) {
	return s.inner.Refresh(ctx, onProgress)
}
