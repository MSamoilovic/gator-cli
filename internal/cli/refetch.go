package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/feeds"
)

func handlerRefetch(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("refetch", flag.ContinueOnError)
	all := fs.Bool("all", false, "forget the validators of every followed feed, not only the thin ones")
	below := fs.Int("below", feeds.DefaultKeepPerFeed, "treat a feed as thin when it holds fewer posts than this")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: refetch [--all] [--below %d]", feeds.DefaultKeepPerFeed)
	}

	n, err := feeds.Forget(ctx, s.Db, int32(*below), *all)
	if err != nil {
		return err
	}

	scope := fmt.Sprintf("%d feeds holding fewer than %d posts", n, *below)
	if *all {
		scope = fmt.Sprintf("%d feeds", n)
	}
	if n == 0 {
		fmt.Println("Nothing to forget — no followed feed is remembering a validator worth clearing")
		return nil
	}

	fmt.Printf("Forgot the stored ETag and Last-Modified of %s\n", scope)
	fmt.Println("The next fetch reads them in full. Run: gator agg 15m")
	return nil
}
