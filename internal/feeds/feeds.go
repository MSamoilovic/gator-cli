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

// resolve povlaci zadatu adresu i vrati feed zajedno sa adresom na kojoj je
// stvarno nadjen. Kad korisnik zalepi adresu sajta umesto feeda — a to je
// uobicajeno, jer RSS adrese niko ne pamti — uzima se feed koji stranica sama
// oglasava.
//
// Uzima se prvi oglaseni: sajtovi glavni feed navode ispred sporednih, pa je
// kod WordPress-a to clanci a ne komentari, a kod sajtova koji nude i Atom i
// RSS isti sadrzaj u dva formata.
func resolve(ctx context.Context, url string) (*rss.RSSFeed, string, error) {
	// Bezuslovno: feed koji se tek dodaje nema sacuvane validatore, a i da ih
	// ima, ovde nam treba telo da bi se izvuklo ime iz <title>.
	feed, _, err := rss.FetchFeed(ctx, url, rss.Validators{})
	if err == nil {
		return feed, url, nil
	}

	var notFeed *rss.NotAFeedError
	if !errors.As(err, &notFeed) || len(notFeed.Links) == 0 {
		return nil, "", fmt.Errorf("not a usable RSS feed: %w", err)
	}

	discovered := notFeed.Links[0].URL
	feed, _, err = rss.FetchFeed(ctx, discovered, rss.Validators{})
	if err != nil {
		return nil, "", fmt.Errorf("%s advertises %s, which is not a usable feed: %w", url, discovered, err)
	}
	return feed, discovered, nil
}

func Add(ctx context.Context, q *database.Queries, userID uuid.UUID, name, url string) (feed database.Feed, created bool, err error) {
	rssFeed, url, err := resolve(ctx, url)
	if err != nil {
		return database.Feed{}, false, err
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

// Entry je jedan feed koji treba dodati. Ime je opciono, isto kao kod Add —
// prazno znaci "uzmi <title> iz samog feeda". Category je folder u koji ide
// pretplata; prazno znaci koren.
type Entry struct {
	Name     string
	URL      string
	Category string
}

type AddResult struct {
	Entry   Entry
	Feed    database.Feed
	Created bool
	Err     error
}


const maxParallelAdds = 8

// AddMany doda i zaprati sve unose. Jedan mrtav URL ne obara ostale — greska
// zavrsi u AddResult.Err, a posao ide dalje.
func AddMany(ctx context.Context, q *database.Queries, userID uuid.UUID, entries []Entry, onResult func(AddResult)) []AddResult {
	return addAll(ctx, entries, maxParallelAdds, onResult, func(ctx context.Context, e Entry) AddResult {
		feed, created, err := Add(ctx, q, userID, e.Name, e.URL)
		if err == nil && e.Category != "" {
			// Kategorija je organizacija, ne sadrzaj: neuspeh njenog upisa ne
			// sme da pretvori uspesno dodat feed u gresku.
			_ = q.SetFeedFollowCategory(ctx, database.SetFeedFollowCategoryParams{
				UserID:   userID,
				FeedID:   feed.ID,
				Category: e.Category,
			})
		}
		return AddResult{Entry: e, Feed: feed, Created: created, Err: err}
	})
}

// addAll pusti add nad svakim unosom paralelno, ali upisuje rezultat po
// indeksu — izlaz prati redosled ulaza. (ScrapeAll koristi append pod mutexom,
// pa je tamo redosled nedeterministican.) Mutex ovde cuva samo onResult.
func addAll(ctx context.Context, entries []Entry, limit int, onResult func(AddResult), add func(context.Context, Entry) AddResult) []AddResult {
	if limit < 1 {
		limit = 1
	}

	var (
		results = make([]AddResult, len(entries))
		sem     = make(chan struct{}, limit)
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for i, e := range entries {
		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			res := add(ctx, e)
			results[i] = res

			if onResult != nil {
				mu.Lock()
				defer mu.Unlock()
				onResult(res)
			}
		}()
	}
	wg.Wait()

	return results
}

type Result struct {
	Feed database.Feed
	// NotModified znaci da je server vratio 304 — nista nije preuzeto ni
	// upisano, pa su Items i Saved nula, a to nije greska.
	NotModified bool
	Items       int
	Saved       int
	Err         error
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

	prev := rss.Validators{ETag: feed.Etag, LastModified: feed.LastModified}

	rssFeed, next, err := rss.FetchFeed(ctx, feed.Url, prev)
	switch {
	case errors.Is(err, rss.ErrNotModified):
		// 304 je uredan odgovor servera, dakle feed je ziv.
		markHealthy(ctx, q, feed)
		res.NotModified = true
		return res
	case err != nil:
		markFailed(ctx, q, feed, err)
		res.Err = fmt.Errorf("fetching %s: %w", feed.Name, err)
		return res
	}
	markHealthy(ctx, q, feed)

	// Otisci se pamte tek posle uspesnog parsiranja: da su upisani ranije, feed
	// koji vrati neispravan XML bi sledeci put dobio 304 i nikad se ne bi
	// oporavio.
	if next != prev {
		if err := q.SaveFeedValidators(ctx, database.SaveFeedValidatorsParams{
			ID:           feed.ID,
			Etag:         next.ETag,
			LastModified: next.LastModified,
		}); err != nil {
			res.Err = fmt.Errorf("saving validators for %s: %w", feed.Name, err)
			return res
		}
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

// markFailed i markHealthy su knjigovodstvo o zdravlju feeda. Namerno se
// pozivaju samo oko FetchFeed: pad upisa u bazu nije krivica feeda, pa ne sme
// da ga obelezi kao pokvaren. Iz istog razloga se greska pri samom upisu
// zanemaruje — ne sme da zameni stvarni ishod povlacenja.
func markFailed(ctx context.Context, q *database.Queries, feed database.Feed, cause error) {
	_ = q.MarkFeedFailed(ctx, database.MarkFeedFailedParams{
		ID:        feed.ID,
		LastError: cause.Error(),
	})
}

func markHealthy(ctx context.Context, q *database.Queries, feed database.Feed) {
	_ = q.MarkFeedHealthy(ctx, feed.ID)
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
	// RSS 1.0 feedovi cesto salju samo datum u dc:date (Nature).
	time.DateOnly,
}

func ParsePubDate(s string) sql.NullTime {
	for _, layout := range pubDateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return sql.NullTime{Time: t, Valid: true}
		}
	}
	return sql.NullTime{}
}
