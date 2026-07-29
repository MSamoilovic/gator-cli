package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"gator-cli/internal/database"
	"gator-cli/internal/rss"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// followFeed je idempotentan: ako korisnik vec prati feed, vraca created=false
// bez greske (upit radi ON CONFLICT DO NOTHING).
func followFeed(s *state, userID, feedID uuid.UUID) (row database.CreateFeedFollowRow, created bool, err error) {
	rows, err := s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
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

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 2 {
		return fmt.Errorf("usage: addfeed <name> <url>")
	}

	feed, err := s.Db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.Args[0],
		Url:       cmd.Args[1],
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("error creating feed: %v", err)
	}

	if _, _, err := followFeed(s, user.ID, feed.ID); err != nil {
		return fmt.Errorf("error following feed: %w", err)
	}

	fmt.Printf("Added feed %q (%s)\n", feed.Name, feed.Url)
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: follow <url>")
	}

	feed, err := s.Db.GetFeedByUrl(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("feed not found: %v", err)
	}

	follow, created, err := followFeed(s, user.ID, feed.ID)
	if err != nil {
		return fmt.Errorf("error following feed: %w", err)
	}
	if !created {
		fmt.Printf("Already following %s\n", feed.Name)
		return nil
	}

	fmt.Printf("Feed: %s\nUser: %s\n", follow.FeedName, follow.UserName)
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: unfollow <url>")
	}

	feed, err := s.Db.GetFeedByUrl(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("feed not found: %v", err)
	}

	if err := s.Db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		UserID: user.ID,
		FeedID: feed.ID,
	}); err != nil {
		return fmt.Errorf("error unfollowing feed: %v", err)
	}

	fmt.Printf("Unfollowed %s\n", feed.Name)
	return nil
}

func handlerFollowing(s *state, _ command, user database.User) error {
	follows, err := s.Db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error fetching follows: %v", err)
	}

	for _, f := range follows {
		fmt.Println(f.FeedName)
	}
	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	fs := flag.NewFlagSet("browse", flag.ContinueOnError)
	limit := fs.Int("limit", 2, "number of posts to show")
	feed := fs.String("feed", "", "filter by feed name (substring match)")
	sortDir := fs.String("sort", "desc", "sort by publish date: asc or desc")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	if *sortDir != "asc" && *sortDir != "desc" {
		return fmt.Errorf("invalid sort %q: use 'asc' or 'desc'", *sortDir)
	}

	posts, err := s.Db.GetPostsForUserFiltered(context.Background(), database.GetPostsForUserFilteredParams{
		UserID:    user.ID,
		FeedName:  *feed,
		SortDir:   *sortDir,
		PostLimit: int32(*limit),
	})
	if err != nil {
		return fmt.Errorf("error fetching posts: %v", err)
	}

	for _, p := range posts {
		printPost(p)
	}
	return nil
}

func handlerSearch(s *state, cmd command, user database.User) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 10, "number of results to show")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	query := strings.Join(fs.Args(), " ")
	if query == "" {
		return fmt.Errorf("usage: search <query> [--limit N]")
	}

	posts, err := s.Db.SearchPostsForUser(context.Background(), database.SearchPostsForUserParams{
		UserID:    user.ID,
		Query:     query,
		PostLimit: int32(*limit),
	})
	if err != nil {
		return fmt.Errorf("error searching posts: %v", err)
	}

	if len(posts) == 0 {
		fmt.Printf("No posts found matching %q\n", query)
		return nil
	}

	for _, p := range posts {
		printPost(p)
	}
	return nil
}

func handlerFeeds(s *state, _ command) error {
	feeds, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("error fetching feeds: %v", err)
	}

	for _, f := range feeds {
		fmt.Printf("Name: %s\nURL: %s\nUser: %s\n\n", f.Name, f.Url, f.UserName)
	}
	return nil
}

func printPost(p database.Post) {
	fmt.Printf("--- %s ---\n", p.Title)
	fmt.Printf("URL: %s\n", p.Url)
	if p.Description.Valid {
		fmt.Printf("%s\n", p.Description.String)
	}
	fmt.Println()
}

var pubDateFormats = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	"02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 MST",
}

func parsePubDate(s string) sql.NullTime {
	for _, layout := range pubDateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return sql.NullTime{Time: t, Valid: true}
		}
	}
	return sql.NullTime{}
}

// pqUniqueViolation is the Postgres error code for a unique constraint conflict.
const pqUniqueViolation = "23505"

func scrapeFeeds(s *state) {
	feeds, err := s.Db.GetFeedsToFetch(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error getting feeds:", err)
		return
	}

	var wg sync.WaitGroup
	for _, feed := range feeds {
		wg.Add(1)
		go func(feed database.Feed) {
			defer wg.Done()
			scrapeFeed(s, feed)
		}(feed)
	}
	wg.Wait()
}

func scrapeFeed(s *state, feed database.Feed) {
	if err := s.Db.MarkFeedFetched(context.Background(), feed.ID); err != nil {
		fmt.Fprintln(os.Stderr, "error marking feed fetched:", err)
		return
	}

	rssFeed, err := rss.FetchFeed(context.Background(), feed.Url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching feed %s: %v\n", feed.Name, err)
		return
	}

	for _, item := range rssFeed.Channel.Item {
		_, err := s.Db.CreatePost(context.Background(), database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Title:       item.Title,
			Url:         item.Link,
			Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
			PublishedAt: parsePubDate(item.PubDate),
			FeedID:      feed.ID,
		})
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pqUniqueViolation {
				continue
			}
			fmt.Fprintf(os.Stderr, "error saving post %q: %v\n", item.Title, err)
		}
	}
	fmt.Printf("Fetched %d posts from %s\n", len(rssFeed.Channel.Item), feed.Name)
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: agg <time_between_reqs>")
	}

	timeBetweenReqs, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Collecting feeds every %s\n", timeBetweenReqs)
	fmt.Println("Press Ctrl+C to stop.")

	ticker := time.NewTicker(timeBetweenReqs)
	defer ticker.Stop()
	for {
		scrapeFeeds(s)
		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down...")
			return nil
		case <-ticker.C:
		}
	}
}
