package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"gator-cli/internal/catalog"
	"gator-cli/internal/database"
	"gator-cli/internal/feeds"

	"github.com/google/uuid"
)

func handlerDiscover(s *state, cmd command, user database.User) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	add := fs.String("add", "", "comma-separated categories to add and follow")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	ctx := context.Background()

	if *add != "" {
		return addCategories(ctx, s, user, splitCategories(*add))
	}

	switch args := fs.Args(); len(args) {
	case 0:
		return listCategories(ctx, s, user)
	case 1:
		return listCategoryFeeds(ctx, s, user, args[0])
	default:
		return fmt.Errorf("usage: discover [category] | discover --add <category,...>")
	}
}

func listCategories(ctx context.Context, s *state, user database.User) error {
	cats, err := catalog.Categories()
	if err != nil {
		return err
	}
	followed, err := followedURLs(ctx, s, user.ID)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, c := range cats {
		line := fmt.Sprintf("%s\t%s\t%d feeds", c.ID, c.Label, len(c.Feeds))
		if n := countFollowed(c.Feeds, followed); n > 0 {
			line += fmt.Sprintf("\t(%d followed)", n)
		}
		fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println("\nAdd a whole category with: gator discover --add <category>")
	fmt.Println("See what is in one with:   gator discover <category>")
	return nil
}

func listCategoryFeeds(ctx context.Context, s *state, user database.User, id string) error {
	c, err := catalog.Find(id)
	if err != nil {
		return err
	}
	followed, err := followedURLs(ctx, s, user.ID)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n\n", c.Label)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, f := range c.Feeds {
		mark := " "
		if followed[f.URL] {
			mark = "✓"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", mark, f.Name, f.URL)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	if countFollowed(c.Feeds, followed) < len(c.Feeds) {
		fmt.Printf("\nFollow them all with: gator discover --add %s\n", c.ID)
	}
	return nil
}

func addCategories(ctx context.Context, s *state, user database.User, ids []string) error {
	catFeeds, err := catalog.Resolve(ids)
	if err != nil {
		return err
	}
	if len(catFeeds) == 0 {
		return fmt.Errorf("no feeds in %s", strings.Join(ids, ", "))
	}

	entries := make([]feeds.Entry, len(catFeeds))
	for i, f := range catFeeds {
		entries[i] = feeds.Entry{Name: f.Name, URL: f.URL}
	}

	results := feeds.AddMany(ctx, s.Db, user.ID, entries, func(r feeds.AddResult) {
		switch {
		case r.Err != nil:
			fmt.Fprintf(os.Stderr, "  x %s: %v\n", r.Entry.Name, r.Err)
		case r.Created:
			fmt.Printf("  + %s\n", r.Entry.Name)
		default:
			fmt.Printf("  = %s (already known)\n", r.Entry.Name)
		}
	})

	var added, known, failed int
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
		case r.Created:
			added++
		default:
			known++
		}
	}

	fmt.Printf("\n%d added, %d already known, %d failed\n", added, known, failed)
	if failed == len(results) {
		return fmt.Errorf("no feed could be added")
	}
	fmt.Println("Fetch posts with: gator agg 15m")
	return nil
}

func followedURLs(ctx context.Context, s *state, userID uuid.UUID) (map[string]bool, error) {
	rows, err := s.Db.GetFeedFollowsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching follows: %w", err)
	}

	followed := make(map[string]bool, len(rows))
	for _, r := range rows {
		followed[r.FeedUrl] = true
	}
	return followed, nil
}

func countFollowed(catFeeds []catalog.Feed, followed map[string]bool) int {
	n := 0
	for _, f := range catFeeds {
		if followed[f.URL] {
			n++
		}
	}
	return n
}

func splitCategories(s string) []string {
	var ids []string
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
