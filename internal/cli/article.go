package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/text"
)

func handlerArticle(ctx context.Context, s *state, cmd command, _ store.User) error {
	fs := flag.NewFlagSet("article", flag.ContinueOnError)
	refetch := fs.Bool("refetch", false, "fetch again even if the text is already stored")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: article <post_url> [--refetch]")
	}

	post, err := s.Store.PostByURL(ctx, fs.Args()[0])
	if err != nil {
		return fmt.Errorf("post not found: %w", err)
	}

	if post.FullText != "" && !*refetch {
		fmt.Println(post.FullText)
		return nil
	}

	body, err := s.Store.FullText(ctx, post)
	if err != nil {
		return err
	}

	fmt.Println(body)
	feedGave := text.StripHTML(post.Description.String)
	fmt.Fprintf(os.Stderr, "\n%d characters, where the feed gave %d\n",
		len([]rune(body)), len([]rune(feedGave)))
	return nil
}
