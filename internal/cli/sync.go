package cli

import (
	"context"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/store"
)

func handlerSync(ctx context.Context, s *state, cmd command) error {
	if len(cmd.Args) != 0 {
		return fmt.Errorf("usage: sync")
	}

	syncer, ok := s.Store.(store.Syncer)
	if !ok {
		return fmt.Errorf("this store keeps no offline cache, so there is nothing to sync")
	}

	res, err := syncer.Sync(ctx)
	printSync(s.Cfg.ServerURL, res, err == nil)
	return err
}

func printSync(server string, res store.SyncResult, ok bool) {
	switch {
	case res.Sent > 0:
		fmt.Printf("Sent %d queued change(s) to %s\n", res.Sent, server)
	case ok:
		fmt.Println("Nothing was waiting to be sent")
	}
	if res.Dropped > 0 {
		fmt.Printf("Lost %d change(s) that overflowed the queue while offline\n", res.Dropped)
	}
	if res.Pending > 0 {
		fmt.Printf("%d change(s) still waiting\n", res.Pending)
	}
	if res.Feeds > 0 {
		fmt.Printf("Cached %d posts from %d feeds for offline use\n", res.Posts, res.Feeds)
	}
}
