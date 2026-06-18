package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"time"

	"gator-ci/internal/database"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

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

	_, err = s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("error following feed: %v", err)
	}

	fmt.Printf("%+v\n", feed)
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

	follow, err := s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("error following feed: %v", err)
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
	page := fs.Int("page", 1, "page number (1-based)")
	feed := fs.String("feed", "", "filter by feed name (substring match)")
	sortDir := fs.String("sort", "desc", "sort by publish date: asc or desc")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	if *sortDir != "asc" && *sortDir != "desc" {
		return fmt.Errorf("invalid sort %q: use 'asc' or 'desc'", *sortDir)
	}
	if *limit < 1 {
		return fmt.Errorf("limit must be at least 1")
	}
	if *page < 1 {
		return fmt.Errorf("page must be at least 1")
	}

	offset := (*page - 1) * *limit

	posts, err := s.Db.GetPostsForUserFiltered(context.Background(), database.GetPostsForUserFilteredParams{
		UserID:     user.ID,
		FeedName:   *feed,
		SortDir:    *sortDir,
		PostOffset: int32(offset),
		PostLimit:  int32(*limit),
	})
	if err != nil {
		return fmt.Errorf("error fetching posts: %v", err)
	}

	fmt.Printf("Page %d (showing up to %d posts)\n\n", *page, *limit)

	for _, p := range posts {
		fmt.Printf("--- %s ---\n", p.Title)
		fmt.Printf("URL: %s\n", p.Url)
		if p.Description.Valid {
			fmt.Printf("%s\n", p.Description.String)
		}
		fmt.Println()
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

func scrapeFeeds(s *state) {
	feed, err := s.Db.GetNextFeedToFetch(context.Background())
	if err != nil {
		fmt.Println("error getting next feed:", err)
		return
	}

	if err := s.Db.MarkFeedFetched(context.Background(), feed.ID); err != nil {
		fmt.Println("error marking feed fetched:", err)
		return
	}

	rssFeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		fmt.Println("error fetching feed:", err)
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
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				continue
			}
			fmt.Printf("error saving post %q: %v\n", item.Title, err)
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

	fmt.Printf("Collecting feeds every %s\n", timeBetweenReqs)

	ticker := time.NewTicker(timeBetweenReqs)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}