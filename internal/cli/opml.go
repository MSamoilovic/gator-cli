package cli

import (
	"context"
	"fmt"
	"os"

	"gator-cli/internal/database"
	"gator-cli/internal/feeds"
	"gator-cli/internal/opml"
)

// stdioName je dogovor iz Unix alata: "-" znaci stdin odnosno stdout, pa
// `curl ... | gator import -` i `gator export - > f.opml` rade bez privremenog
// fajla.
const stdioName = "-"

func handlerExport(s *state, cmd command, user database.User) error {
	if len(cmd.Args) > 1 {
		return fmt.Errorf("usage: export [file|-]")
	}

	follows, err := s.Db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error fetching follows: %w", err)
	}

	list := make([]opml.Feed, len(follows))
	for i, f := range follows {
		list[i] = opml.Feed{Title: f.FeedName, XMLURL: f.FeedUrl}
	}

	out := os.Stdout
	if len(cmd.Args) == 1 && cmd.Args[0] != stdioName {
		f, err := os.Create(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("creating %s: %w", cmd.Args[0], err)
		}
		defer f.Close()
		out = f
	}

	if err := opml.Write(out, user.Name+" subscriptions", list); err != nil {
		return err
	}

	// Poruka ide na stderr da ne zaprlja OPML kad izlaz ide u pipe.
	if out == os.Stdout {
		fmt.Fprintf(os.Stderr, "Exported %d feeds\n", len(list))
	} else {
		fmt.Printf("Exported %d feeds to %s\n", len(list), cmd.Args[0])
	}
	return nil
}

func handlerImport(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: import <file|->")
	}

	in := os.Stdin
	if cmd.Args[0] != stdioName {
		f, err := os.Open(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("opening %s: %w", cmd.Args[0], err)
		}
		defer f.Close()
		in = f
	}

	list, err := opml.Parse(in)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return fmt.Errorf("no feeds found in the OPML")
	}

	entries := make([]feeds.Entry, len(list))
	for i, f := range list {
		entries[i] = feeds.Entry{Name: f.Title, URL: f.XMLURL}
	}

	fmt.Printf("Importing %d feeds…\n", len(entries))

	results := feeds.AddMany(context.Background(), s.Db, user.ID, entries, func(r feeds.AddResult) {
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
		return fmt.Errorf("no feed from the OPML could be added")
	}
	fmt.Println("Fetch posts with: gator agg 15m")
	return nil
}
