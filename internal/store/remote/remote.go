package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

const (
	requestTimeout = 60 * time.Second
	maxResponse    = 32 << 20
	maxParallelAdd = 8
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Is(target error) bool {
	switch e.Code {
	case wire.CodeFeedNotFound:
		return target == store.ErrFeedNotFound
	case wire.CodeNotFollowed:
		return target == store.ErrNotFollowed
	case wire.CodeNoMoreText:
		return target == store.ErrNoMoreText
	}
	return false
}

type Store struct {
	base   string
	token  string
	client *http.Client

	mu   sync.Mutex
	user store.User
}

var _ store.Store = (*Store)(nil)

func New(serverURL, token string) *Store {
	return &Store{
		base:   strings.TrimRight(serverURL, "/"),
		token:  token,
		client: &http.Client{Timeout: requestTimeout},
	}
}

func Login(ctx context.Context, serverURL, login, password string) (wire.TokenResponse, error) {
	return exchange(ctx, serverURL, "/v1/tokens", wire.TokenRequest{Login: login, Password: password})
}

func Register(ctx context.Context, serverURL, name, email, password string) (wire.TokenResponse, error) {
	return exchange(ctx, serverURL, "/v1/users", wire.RegisterRequest{Name: name, Email: email, Password: password})
}

func exchange(ctx context.Context, serverURL, path string, body any) (wire.TokenResponse, error) {
	var out wire.TokenResponse
	err := New(serverURL, "").do(ctx, http.MethodPost, path, nil, body, &out)
	return out, err
}

func (s *Store) Logout(ctx context.Context) error {
	return s.do(ctx, http.MethodDelete, "/v1/tokens/current", nil, nil, nil)
}

func (s *Store) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	target := s.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("reaching %s: %w", s.base, err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponse))
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if res.StatusCode >= 400 {
		e := &Error{Status: res.StatusCode, Message: res.Status}
		var payload wire.Error
		if json.Unmarshal(data, &payload) == nil && payload.Error != "" {
			e.Code, e.Message = payload.Code, payload.Error
		}
		return e
	}

	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", s.base, err)
	}
	return nil
}

func (s *Store) Me(ctx context.Context) (store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.user.ID != uuid.Nil {
		return s.user, nil
	}

	var u wire.User
	if err := s.do(ctx, http.MethodGet, "/v1/me", nil, nil, &u); err != nil {
		return store.User{}, err
	}
	s.user = u.ToDatabase()
	return s.user, nil
}

func (s *Store) Posts(ctx context.Context, q store.PostQuery) ([]store.Post, error) {
	v := url.Values{}
	set := func(key, val string) {
		if val != "" {
			v.Set(key, val)
		}
	}
	if q.FeedID != uuid.Nil {
		set("feed_id", q.FeedID.String())
	}
	set("feed_name", q.FeedName)
	set("q", q.Query)
	set("sort", q.SortDir)
	if q.UnreadOnly {
		set("unread", "true")
	}
	if !q.Since.IsZero() {
		set("since", q.Since.UTC().Format(time.RFC3339Nano))
	}
	set("limit", strconv.Itoa(int(q.Limit)))
	if q.Offset > 0 {
		set("offset", strconv.Itoa(int(q.Offset)))
	}

	return s.posts(ctx, "/v1/posts", v)
}

func (s *Store) posts(ctx context.Context, path string, query url.Values) ([]store.Post, error) {
	var out wire.Posts
	if err := s.do(ctx, http.MethodGet, path, query, nil, &out); err != nil {
		return nil, err
	}
	return wire.ToPosts(out.Posts), nil
}

func (s *Store) BookmarkedPosts(ctx context.Context) ([]store.Post, error) {
	return s.posts(ctx, "/v1/posts/bookmarked", nil)
}

func (s *Store) ReadPosts(ctx context.Context) ([]store.Post, error) {
	return s.posts(ctx, "/v1/posts/read", nil)
}

func (s *Store) PostByURL(ctx context.Context, postURL string) (store.Post, error) {
	var out wire.Post
	if err := s.do(ctx, http.MethodGet, "/v1/posts/by-url", url.Values{"url": {postURL}}, nil, &out); err != nil {
		return store.Post{}, err
	}
	return out.ToDatabase(), nil
}

func (s *Store) ids(ctx context.Context, path string) ([]uuid.UUID, error) {
	var out wire.IDs
	if err := s.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return nilIfEmpty(out.IDs), nil
}

func (s *Store) BookmarkedIDs(ctx context.Context) ([]uuid.UUID, error) {
	return s.ids(ctx, "/v1/posts/bookmarked/ids")
}

func (s *Store) ReadIDs(ctx context.Context) ([]uuid.UUID, error) {
	return s.ids(ctx, "/v1/posts/read/ids")
}

func (s *Store) UnreadCounts(ctx context.Context) ([]store.UnreadCount, error) {
	var out wire.Unread
	if err := s.do(ctx, http.MethodGet, "/v1/unread", nil, nil, &out); err != nil {
		return nil, err
	}
	return nilIfEmpty(out.Unread), nil
}

func (s *Store) FullText(ctx context.Context, post store.Post) (string, error) {
	var out wire.Text
	if err := s.do(ctx, http.MethodGet, "/v1/posts/"+post.ID.String()+"/fulltext", nil, nil, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

func (s *Store) SetRead(ctx context.Context, postID uuid.UUID, read bool) error {
	method := http.MethodPost
	if !read {
		method = http.MethodDelete
	}
	return s.do(ctx, method, "/v1/posts/"+postID.String()+"/read", nil, nil, nil)
}

func (s *Store) SetAllRead(ctx context.Context, postIDs []uuid.UUID) error {
	if len(postIDs) == 0 {
		return nil
	}
	return s.do(ctx, http.MethodPost, "/v1/posts/read", nil, wire.MarkReadRequest{PostIDs: postIDs}, nil)
}

func (s *Store) Bookmark(ctx context.Context, postID uuid.UUID) (bool, error) {
	var out wire.Created
	if err := s.do(ctx, http.MethodPost, "/v1/posts/"+postID.String()+"/bookmark", nil, nil, &out); err != nil {
		return false, err
	}
	return out.Created, nil
}

func (s *Store) Unbookmark(ctx context.Context, postID uuid.UUID) error {
	return s.do(ctx, http.MethodDelete, "/v1/posts/"+postID.String()+"/bookmark", nil, nil, nil)
}

func (s *Store) Subscriptions(ctx context.Context) ([]store.Subscription, error) {
	var out wire.Subscriptions
	if err := s.do(ctx, http.MethodGet, "/v1/subscriptions", nil, nil, &out); err != nil {
		return nil, err
	}
	return nilIfEmpty(out.Subscriptions), nil
}

func (s *Store) Stats(ctx context.Context, since time.Time) ([]store.FeedStat, error) {
	var out wire.Stats
	q := url.Values{"since": {since.UTC().Format(time.RFC3339Nano)}}
	if err := s.do(ctx, http.MethodGet, "/v1/subscriptions/stats", q, nil, &out); err != nil {
		return nil, err
	}
	return nilIfEmpty(out.Stats), nil
}

func (s *Store) AddFeed(ctx context.Context, name, feedURL string) (store.Feed, bool, error) {
	return s.addEntry(ctx, feeds.Entry{Name: name, URL: feedURL})
}

func (s *Store) addEntry(ctx context.Context, e feeds.Entry) (store.Feed, bool, error) {
	var out wire.AddFeedResponse
	req := wire.AddFeedRequest{Name: e.Name, URL: e.URL, Category: e.Category}
	if err := s.do(ctx, http.MethodPost, "/v1/subscriptions", nil, req, &out); err != nil {
		return store.Feed{}, false, err
	}
	return out.Feed.ToDatabase(), out.Created, nil
}

func (s *Store) AddFeeds(ctx context.Context, entries []feeds.Entry, onResult func(feeds.AddResult)) []feeds.AddResult {
	results := make([]feeds.AddResult, len(entries))

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		sem = make(chan struct{}, maxParallelAdd)
	)
	for i, e := range entries {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			feed, created, err := s.addEntry(ctx, e)
			r := feeds.AddResult{Entry: e, Feed: feed, Created: created, Err: err}
			results[i] = r

			if onResult != nil {
				mu.Lock()
				onResult(r)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}

func (s *Store) FollowURL(ctx context.Context, feedURL string) (store.Follow, bool, error) {
	var out wire.FollowResponse
	if err := s.do(ctx, http.MethodPost, "/v1/subscriptions/follow", nil, wire.FollowRequest{URL: feedURL}, &out); err != nil {
		return store.Follow{}, false, err
	}
	return out.Follow, out.Created, nil
}

func (s *Store) UnfollowURL(ctx context.Context, feedURL string) (store.Feed, error) {
	var out wire.FeedResponse
	if err := s.do(ctx, http.MethodDelete, "/v1/subscriptions", url.Values{"url": {feedURL}}, nil, &out); err != nil {
		return store.Feed{}, err
	}
	return out.Feed.ToDatabase(), nil
}

func (s *Store) Unfollow(ctx context.Context, feedID uuid.UUID) error {
	return s.do(ctx, http.MethodDelete, "/v1/subscriptions/"+feedID.String(), nil, nil, nil)
}

func (s *Store) Categorize(ctx context.Context, feedURL, category string) (store.Feed, error) {
	var out wire.FeedResponse
	req := wire.CategorizeRequest{URL: feedURL, Category: category}
	if err := s.do(ctx, http.MethodPatch, "/v1/subscriptions", nil, req, &out); err != nil {
		return store.Feed{}, err
	}
	return out.Feed.ToDatabase(), nil
}

func (s *Store) Refresh(context.Context) (store.RefreshResult, error) {
	return store.RefreshResult{}, fmt.Errorf("feeds are refreshed by the server's aggregator: %w", errors.ErrUnsupported)
}

func nilIfEmpty[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return s
}
