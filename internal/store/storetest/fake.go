package storetest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/store"

	"github.com/google/uuid"
)

type Fake struct {
	Identity   store.User
	AllPosts   []store.Post
	Subs       []store.Subscription
	FeedStats  []store.FeedStat
	Unread     []store.UnreadCount
	Bookmarked map[uuid.UUID]bool
	Read       map[uuid.UUID]bool
	FullTexts  map[uuid.UUID]string
	Refreshed  store.RefreshResult

	Err error

	Asked []store.PostQuery
	Added []feeds.Entry

	mu sync.Mutex
}

var _ store.Store = (*Fake)(nil)

func New(name string, posts ...store.Post) *Fake {
	return &Fake{
		Identity:   store.User{ID: uuid.New(), Name: name},
		AllPosts:   posts,
		Bookmarked: make(map[uuid.UUID]bool),
		Read:       make(map[uuid.UUID]bool),
		FullTexts:  make(map[uuid.UUID]string),
	}
}

func (f *Fake) Post(title, url string) store.Post {
	f.mu.Lock()
	defer f.mu.Unlock()

	p := store.Post{ID: uuid.New(), Title: title, Url: url, FeedID: uuid.New()}
	f.AllPosts = append(f.AllPosts, p)
	return p
}

func (f *Fake) Sub(name, url, category string) store.Subscription {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.sub(name, url, category)
}

func (f *Fake) sub(name, url, category string) store.Subscription {
	s := store.Subscription{
		ID:       uuid.New(),
		UserID:   f.Identity.ID,
		FeedID:   uuid.New(),
		FeedName: name,
		FeedUrl:  url,
		Category: category,
	}
	f.Subs = append(f.Subs, s)
	return s
}

func (f *Fake) Me(context.Context) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return store.User{}, f.Err
	}
	return f.Identity, nil
}

func (f *Fake) Posts(_ context.Context, q store.PostQuery) ([]store.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Asked = append(f.Asked, q)
	if f.Err != nil {
		return nil, f.Err
	}

	var out []store.Post
	for _, p := range f.AllPosts {
		if q.Query != "" && !strings.Contains(strings.ToLower(p.Title), strings.ToLower(q.Query)) {
			continue
		}
		if q.FeedID != uuid.Nil && p.FeedID != q.FeedID {
			continue
		}
		if q.UnreadOnly && f.Read[p.ID] {
			continue
		}
		out = append(out, p)
	}

	if int(q.Offset) >= len(out) {
		return nil, nil
	}
	out = out[q.Offset:]
	if q.Limit > 0 && int(q.Limit) < len(out) {
		out = out[:q.Limit]
	}
	return out, nil
}

func (f *Fake) BookmarkedPosts(context.Context) ([]store.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.selected(f.Bookmarked)
}

func (f *Fake) ReadPosts(context.Context) ([]store.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.selected(f.Read)
}

func (f *Fake) selected(set map[uuid.UUID]bool) ([]store.Post, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	var out []store.Post
	for _, p := range f.AllPosts {
		if set[p.ID] {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *Fake) PostByURL(_ context.Context, url string) (store.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return store.Post{}, f.Err
	}
	for _, p := range f.AllPosts {
		if p.Url == url {
			return p, nil
		}
	}
	return store.Post{}, fmt.Errorf("no post with url %s", url)
}

func (f *Fake) BookmarkedIDs(context.Context) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.ids(f.Bookmarked)
}

func (f *Fake) ReadIDs(context.Context) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.ids(f.Read)
}

func (f *Fake) ids(set map[uuid.UUID]bool) ([]uuid.UUID, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	var out []uuid.UUID
	for _, p := range f.AllPosts {
		if set[p.ID] {
			out = append(out, p.ID)
		}
	}
	return out, nil
}

func (f *Fake) UnreadCounts(context.Context) ([]store.UnreadCount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}
	return f.Unread, nil
}

func (f *Fake) FullText(_ context.Context, post store.Post) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return "", f.Err
	}
	body, ok := f.FullTexts[post.ID]
	if !ok {
		return "", fmt.Errorf("%s: %w", post.Url, store.ErrNoMoreText)
	}
	return body, nil
}

func (f *Fake) SetRead(_ context.Context, postID uuid.UUID, read bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}
	if read {
		f.Read[postID] = true
	} else {
		delete(f.Read, postID)
	}
	return nil
}

func (f *Fake) SetAllRead(_ context.Context, postIDs []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}
	for _, id := range postIDs {
		f.Read[id] = true
	}
	return nil
}

func (f *Fake) Bookmark(_ context.Context, postID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return false, f.Err
	}
	if f.Bookmarked[postID] {
		return false, nil
	}
	f.Bookmarked[postID] = true
	return true, nil
}

func (f *Fake) Unbookmark(_ context.Context, postID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}
	delete(f.Bookmarked, postID)
	return nil
}

func (f *Fake) Subscriptions(context.Context) ([]store.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}
	return f.Subs, nil
}

func (f *Fake) Stats(context.Context, time.Time) ([]store.FeedStat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return nil, f.Err
	}
	return f.FeedStats, nil
}

func (f *Fake) AddFeed(_ context.Context, name, url string) (store.Feed, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.addFeed(name, url)
}

func (f *Fake) addFeed(name, url string) (store.Feed, bool, error) {
	if f.Err != nil {
		return store.Feed{}, false, f.Err
	}
	for _, s := range f.Subs {
		if s.FeedUrl == url {
			return store.Feed{ID: s.FeedID, Name: s.FeedName, Url: url}, false, nil
		}
	}
	if name == "" {
		name = url
	}
	f.sub(name, url, "")
	f.Added = append(f.Added, feeds.Entry{Name: name, URL: url})
	return store.Feed{ID: f.Subs[len(f.Subs)-1].FeedID, Name: name, Url: url}, true, nil
}

func (f *Fake) AddFeeds(_ context.Context, entries []feeds.Entry, onResult func(feeds.AddResult)) []feeds.AddResult {
	f.mu.Lock()
	defer f.mu.Unlock()

	results := make([]feeds.AddResult, len(entries))
	for i, e := range entries {
		feed, created, err := f.addFeed(e.Name, e.URL)
		results[i] = feeds.AddResult{Entry: e, Feed: feed, Created: created, Err: err}
		if onResult != nil {
			onResult(results[i])
		}
	}
	return results
}

func (f *Fake) FollowURL(_ context.Context, url string) (store.Follow, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return store.Follow{}, false, f.Err
	}
	for _, s := range f.Subs {
		if s.FeedUrl == url {
			return store.Follow{
				FeedID:   s.FeedID,
				FeedName: s.FeedName,
				UserID:   f.Identity.ID,
				UserName: f.Identity.Name,
			}, false, nil
		}
	}
	return store.Follow{}, false, fmt.Errorf("%w: %s", store.ErrFeedNotFound, url)
}

func (f *Fake) UnfollowURL(_ context.Context, url string) (store.Feed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return store.Feed{}, f.Err
	}
	for i, s := range f.Subs {
		if s.FeedUrl == url {
			f.Subs = append(f.Subs[:i], f.Subs[i+1:]...)
			return store.Feed{ID: s.FeedID, Name: s.FeedName, Url: url}, nil
		}
	}
	return store.Feed{}, fmt.Errorf("%w: %s", store.ErrFeedNotFound, url)
}

func (f *Fake) Unfollow(_ context.Context, feedID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return f.Err
	}
	for i, s := range f.Subs {
		if s.FeedID == feedID {
			f.Subs = append(f.Subs[:i], f.Subs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *Fake) Categorize(_ context.Context, url, category string) (store.Feed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return store.Feed{}, f.Err
	}
	for i, s := range f.Subs {
		if s.FeedUrl == url {
			f.Subs[i].Category = category
			return store.Feed{ID: s.FeedID, Name: s.FeedName, Url: url}, nil
		}
	}
	return store.Feed{Url: url, Name: url}, fmt.Errorf("%s: %w", url, store.ErrNotFollowed)
}

func (f *Fake) Refresh(context.Context) (store.RefreshResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Err != nil {
		return store.RefreshResult{}, f.Err
	}
	return f.Refreshed, nil
}
