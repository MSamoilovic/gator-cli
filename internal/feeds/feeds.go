package feeds

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gator-cli/internal/database"
	"gator-cli/internal/rss"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// pqUniqueViolation je Postgres kod za prekrsen unique constraint.
const pqUniqueViolation = "23505"

func Follow(ctx context.Context, q *database.Queries, userID, feedID uuid.UUID) (row database.CreateFeedFollowRow, created bool, err error) {
	rows, err := q.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    userID,
		FeedID:    feedID,
	})
	if err != nil {
		return database.CreateFeedFollowRow{}, false, err
	}
	if len(rows) == 0 {
		return database.CreateFeedFollowRow{}, false, nil
	}
	return rows[0], true, nil
}

func Add(ctx context.Context, q *database.Queries, userID uuid.UUID, name, url string) (feed database.Feed, created bool, err error) {
	rssFeed, err := rss.FetchFeed(ctx, url)
	if err != nil {
		return database.Feed{}, false, fmt.Errorf("not a usable RSS feed: %w", err)
	}

	if name = strings.TrimSpace(name); name == "" {
		name = strings.TrimSpace(rssFeed.Channel.Title)
	}
	if name == "" {
		name = url
	}

	feed, err = q.CreateFeed(ctx, database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
		Url:       url,
		UserID:    userID,
	})
	switch {
	case err == nil:
		created = true
	case isDuplicate(err):
		if feed, err = q.GetFeedByUrl(ctx, url); err != nil {
			return database.Feed{}, false, fmt.Errorf("looking up existing feed: %w", err)
		}
	default:
		return database.Feed{}, false, fmt.Errorf("creating feed: %w", err)
	}

	if _, _, err := Follow(ctx, q, userID, feed.ID); err != nil {
		return database.Feed{}, false, fmt.Errorf("following %s: %w", feed.Name, err)
	}
	return feed, created, nil
}

type Result struct {
	Feed  database.Feed
	Items int
	Saved int
	Err   error
}

func ScrapeAll(ctx context.Context, q *database.Queries, onResult func(Result)) ([]Result, error) {
	feeds, err := q.GetFeedsToFetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting feeds: %w", err)
	}

	var (
		mu      sync.Mutex
		results = make([]Result, 0, len(feeds))
		wg      sync.WaitGroup
	)

	for _, feed := range feeds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := Scrape(ctx, q, feed)

			mu.Lock()
			defer mu.Unlock()
			results = append(results, res)
			if onResult != nil {
				onResult(res)
			}
		}()
	}
	wg.Wait()

	return results, nil
}

func Scrape(ctx context.Context, q *database.Queries, feed database.Feed) Result {
	res := Result{Feed: feed}

	if err := q.MarkFeedFetched(ctx, feed.ID); err != nil {
		res.Err = fmt.Errorf("marking %s fetched: %w", feed.Name, err)
		return res
	}

	rssFeed, err := rss.FetchFeed(ctx, feed.Url)
	if err != nil {
		res.Err = fmt.Errorf("fetching %s: %w", feed.Name, err)
		return res
	}
	res.Items = len(rssFeed.Channel.Item)

	for _, item := range rssFeed.Channel.Item {
		_, err := q.CreatePost(ctx, database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       item.Title,
			Url:         item.Link,
			Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
			PublishedAt: ParsePubDate(item.PubDate),
			FeedID:      feed.ID,
		})
		switch {
		case err == nil:
			res.Saved++
		case isDuplicate(err):
			continue
		default:
			res.Err = fmt.Errorf("saving %q: %w", item.Title, err)
			return res
		}
	}

	return res
}

func isDuplicate(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == pqUniqueViolation
}

var pubDateFormats = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	"02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 MST",
}

func ParsePubDate(s string) sql.NullTime {
	for _, layout := range pubDateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return sql.NullTime{Time: t, Valid: true}
		}
	}
	return sql.NullTime{}
}
