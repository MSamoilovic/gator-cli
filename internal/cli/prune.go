package cli

import (
	"context"
	"flag"
	"fmt"

	"gator-cli/internal/feeds"
)

func handlerPrune(s *state, cmd command) error {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	olderThan := fs.Duration("older-than", feeds.DefaultRetention, "delete posts older than this")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: prune [--older-than %s]", feeds.DefaultRetention)
	}

	n, err := feeds.Prune(context.Background(), s.Db, *olderThan)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted %d posts older than %s\n", n, *olderThan)
	return nil
}
