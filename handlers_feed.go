package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gator-cli/internal/database"
	"gator-cli/internal/feeds"
	"gator-cli/internal/tui"
)

func handlerAddFeed(s *state, cmd command, user database.User) error {
	// Ime je opciono: bez njega se uzima <title> iz samog feeda.
	var name, url string
	switch len(cmd.Args) {
	case 1:
		url = cmd.Args[0]
	case 2:
		name, url = cmd.Args[0], cmd.Args[1]
	default:
		return fmt.Errorf("usage: addfeed [name] <url>")
	}

	feed, created, err := feeds.Add(context.Background(), s.Db, user.ID, name, url)
	if err != nil {
		return fmt.Errorf("error adding feed: %w", err)
	}
	if !created {
		fmt.Printf("Following existing feed %q (%s)\n", feed.Name, feed.Url)
		return nil
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

	follow, created, err := feeds.Follow(context.Background(), s.Db, user.ID, feed.ID)
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
	page := fs.Int("page", 1, "page of results, 1-based")
	feed := fs.String("feed", "", "filter by feed name (substring match)")
	sortDir := fs.String("sort", "desc", "sort by publish date: asc or desc")
	noTUI := fs.Bool("no-tui", false, "print posts instead of opening the TUI")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	if *sortDir != "asc" && *sortDir != "desc" {
		return fmt.Errorf("invalid sort %q: use 'asc' or 'desc'", *sortDir)
	}
	if *limit < 1 {
		return fmt.Errorf("invalid limit %d: must be at least 1", *limit)
	}
	if *page < 1 {
		return fmt.Errorf("invalid page %d: pages are 1-based", *page)
	}

	// Interaktivni terminal dobija TUI; pipe i --no-tui dobijaju plain ispis.
	if !*noTUI && stdoutIsTerminal() {
		return tui.Run(context.Background(), s.Db, user.ID)
	}

	posts, err := s.Db.GetPostsForUserFiltered(context.Background(), database.GetPostsForUserFilteredParams{
		UserID:     user.ID,
		FeedName:   *feed,
		SortDir:    *sortDir,
		PostLimit:  int32(*limit),
		PostOffset: int32((*page - 1) * *limit),
	})
	if err != nil {
		return fmt.Errorf("error fetching posts: %w", err)
	}

	for _, p := range posts {
		printPost(p)
	}
	return nil
}

// stdoutIsTerminal razlikuje interaktivni terminal od pipe-a ili fajla.
func stdoutIsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
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

func scrapeFeeds(s *state) {
	_, err := feeds.ScrapeAll(context.Background(), s.Db, func(r feeds.Result) {
		if r.Err != nil {
			fmt.Fprintln(os.Stderr, "error:", r.Err)
			return
		}
		fmt.Printf("Fetched %d posts from %s (%d new)\n", r.Items, r.Feed.Name, r.Saved)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
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
