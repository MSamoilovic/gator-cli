package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
)

func handlerPrune(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	olderThan := fs.Duration("older-than", feeds.DefaultRetention, "delete posts older than this")
	keep := fs.Int("keep-per-feed", feeds.DefaultKeepPerFeed, "always keep this many newest posts per feed, whatever their age")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: prune [--older-than %s] [--keep-per-feed %d]", feeds.DefaultRetention, feeds.DefaultKeepPerFeed)
	}

	n, err := feeds.Prune(ctx, s.Db, *olderThan, int32(*keep))
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %d posts older than %s, keeping the newest %d per feed\n", n, *olderThan, *keep)
	return nil
}
