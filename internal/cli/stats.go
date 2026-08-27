package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"gator-cli/internal/database"
)

const statsWindow = 7 * 24 * time.Hour

const neverLabel = "never"

func handlerStats(s *state, cmd command, user database.User) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	sortBy := fs.String("sort", "posts", "order rows by: posts, week, read, unread, stale or name")
	limit := fs.Int("limit", 0, "show only the first N feeds (0 = all)")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	less, err := statsOrder(*sortBy)
	if err != nil {
		return err
	}
	if *limit < 0 {
		return fmt.Errorf("invalid limit %d: must not be negative", *limit)
	}

	now := time.Now()
	rows, err := s.Db.GetFeedStatsForUser(context.Background(), database.GetFeedStatsForUserParams{
		UserID: user.ID,
		Since:  now.Add(-statsWindow),
	})
	if err != nil {
		return fmt.Errorf("error fetching stats: %w", err)
	}
	if len(rows) == 0 {
		fmt.Println("You are not following any feeds yet — pick some with: gator discover")
		return nil
	}

	sort.SliceStable(rows, func(i, j int) bool { return less(rows[i], rows[j]) })

	shown := rows
	if *limit > 0 && *limit < len(shown) {
		shown = shown[:*limit]
	}

	printStatsSummary(rows)
	printStatsTable(shown, now)
	printStatsAdvice(rows)
	return nil
}

func printStatsSummary(rows []database.GetFeedStatsForUserRow) {
	var posts, recent, read, saved, broken int64
	for _, r := range rows {
		posts += r.PostCount
		recent += r.RecentCount
		read += r.ReadCount
		saved += r.BookmarkCount
		if r.FailureCount > 0 {
			broken++
		}
	}

	summary := fmt.Sprintf("%s · %s · %d read (%s) · %d saved · %d in the last 7 days",
		plural(int64(len(rows)), "feed"), plural(posts, "post"), read, percent(read, posts), saved, recent)
	if broken > 0 {
		summary += fmt.Sprintf(" · %s %d failing", brokenMark, broken)
	}
	fmt.Println(summary)
	fmt.Println()
}

func printStatsTable(rows []database.GetFeedStatsForUserRow, now time.Time) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FEED\tPOSTS\t/WEEK\tREAD\tUNREAD\tLAST POST\tLAST READ")

	for _, r := range rows {
		name := r.FeedName
		if r.FailureCount > 0 {
			name = brokenMark + " " + name
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%s\t%s\n",
			name,
			r.PostCount,
			r.RecentCount,
			r.ReadCount,
			r.PostCount-r.ReadCount,
			ago(r.LastPublished, now),
			ago(r.LastRead, now),
		)
	}
	w.Flush()
}

func printStatsAdvice(rows []database.GetFeedStatsForUserRow) {
	var noisy []database.GetFeedStatsForUserRow
	for _, r := range rows {
		if r.ReadCount == 0 && r.BookmarkCount == 0 && r.RecentCount > 0 {
			noisy = append(noisy, r)
		}
	}
	if len(noisy) == 0 {
		return
	}

	sort.SliceStable(noisy, func(i, j int) bool { return noisy[i].RecentCount > noisy[j].RecentCount })
	if len(noisy) > 5 {
		noisy = noisy[:5]
	}

	fmt.Printf("\nArriving but never opened:\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range noisy {
		fmt.Fprintf(w, "  %s\t%d/week\t%s\n", r.FeedName, r.RecentCount, r.FeedUrl)
	}
	w.Flush()
	fmt.Println("\nDrop one with: gator unfollow <url>")
}

func statsOrder(name string) (func(a, b database.GetFeedStatsForUserRow) bool, error) {
	switch name {
	case "posts":
		return func(a, b database.GetFeedStatsForUserRow) bool { return a.PostCount > b.PostCount }, nil
	case "week":
		return func(a, b database.GetFeedStatsForUserRow) bool { return a.RecentCount > b.RecentCount }, nil
	case "read":
		return func(a, b database.GetFeedStatsForUserRow) bool { return a.ReadCount > b.ReadCount }, nil
	case "unread":
		return func(a, b database.GetFeedStatsForUserRow) bool {
			return a.PostCount-a.ReadCount > b.PostCount-b.ReadCount
		}, nil
	case "stale":
		return func(a, b database.GetFeedStatsForUserRow) bool {
			return a.LastPublished.Before(b.LastPublished)
		}, nil
	case "name":
		return func(a, b database.GetFeedStatsForUserRow) bool {
			return strings.ToLower(a.FeedName) < strings.ToLower(b.FeedName)
		}, nil
	}
	return nil, fmt.Errorf("invalid sort %q: use posts, week, read, unread, stale or name", name)
}

func plural(n int64, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.FormatInt(n, 10) + " " + word + "s"
}

func percent(part, whole int64) string {
	if whole == 0 {
		return "0%"
	}
	p := float64(part) * 100 / float64(whole)
	if p > 0 && p < 1 {
		return "<1%"
	}
	return strconv.FormatFloat(p, 'f', 0, 64) + "%"
}

func ago(t, now time.Time) string {
	if t.IsZero() {
		return neverLabel
	}

	d := now.Sub(t)
	switch {
	case d < 0:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "d"
	case d < 365*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/(24*30))) + "mo"
	default:
		return strconv.Itoa(int(d.Hours()/(24*365))) + "y"
	}
}
