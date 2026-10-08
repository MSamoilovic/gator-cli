package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/MSamoilovic/gator-cli/internal/store"
)

const rootLabel = "(uncategorized)"

func handlerCategorize(ctx context.Context, s *state, cmd command, _ store.User) error {
	if len(cmd.Args) != 2 {
		return fmt.Errorf(`usage: categorize <feed_url> <category>   (empty category moves it back to the root)`)
	}
	url, category := cmd.Args[0], strings.TrimSpace(cmd.Args[1])

	feed, err := s.Store.Categorize(ctx, url, category)
	if errors.Is(err, store.ErrNotFollowed) {
		return fmt.Errorf("you do not follow %s — run: gator follow %s", feed.Name, url)
	}
	if err != nil {
		return err
	}

	if category == "" {
		fmt.Printf("Moved %s to the root\n", feed.Name)
		return nil
	}
	fmt.Printf("Moved %s to %s\n", feed.Name, category)
	return nil
}

func handlerFollowing(ctx context.Context, s *state, _ command, _ store.User) error {
	follows, err := s.Store.Subscriptions(ctx)
	if err != nil {
		return fmt.Errorf("error fetching follows: %w", err)
	}

	grouped := make(map[string][]store.Subscription)
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
