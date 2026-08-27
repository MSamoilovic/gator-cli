package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"gator-cli/internal/database"

	"github.com/google/uuid"
)

const rootLabel = "(uncategorized)"

func handlerCategorize(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 2 {
		return fmt.Errorf(`usage: categorize <feed_url> <category>   (empty category moves it back to the root)`)
	}
	url, category := cmd.Args[0], strings.TrimSpace(cmd.Args[1])

	ctx := context.Background()
	feed, err := s.Db.GetFeedByUrl(ctx, url)
	if err != nil {
		return fmt.Errorf("feed not found: %w", err)
	}

	follows, err := s.Db.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("error fetching follows: %w", err)
	}
	if !followsFeed(follows, feed.ID) {
		return fmt.Errorf("you do not follow %s — run: gator follow %s", feed.Name, url)
	}

	if err := s.Db.SetFeedFollowCategory(ctx, database.SetFeedFollowCategoryParams{
		UserID:   user.ID,
		FeedID:   feed.ID,
		Category: category,
	}); err != nil {
		return fmt.Errorf("error setting category: %w", err)
	}

	if category == "" {
		fmt.Printf("Moved %s to the root\n", feed.Name)
		return nil
	}
	fmt.Printf("Moved %s to %s\n", feed.Name, category)
	return nil
}

func followsFeed(follows []database.GetFeedFollowsForUserRow, feedID uuid.UUID) bool {
	for _, f := range follows {
		if f.FeedID == feedID {
			return true
		}
	}
	return false
}

func handlerFollowing(s *state, _ command, user database.User) error {
	follows, err := s.Db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error fetching follows: %w", err)
	}

	grouped := make(map[string][]database.GetFeedFollowsForUserRow)
	for _, f := range follows {
		grouped[f.Category] = append(grouped[f.Category], f)
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if _, ok := grouped[""]; ok {
		names = append(names, "")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, name := range names {
		label := name
		if label == "" {
			label = rootLabel
		}
		fmt.Fprintf(w, "%s\t(%d)\n", label, len(grouped[name]))

		rows := grouped[name]
		sort.Slice(rows, func(i, j int) bool { return rows[i].FeedName < rows[j].FeedName })
		for _, f := range rows {
			mark := " "
			if f.FeedFailures > 0 {
				mark = brokenMark
			}
			fmt.Fprintf(w, "  %s %s\t\n", mark, f.FeedName)
		}
	}
	return w.Flush()
}
