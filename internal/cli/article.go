package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"gator-cli/internal/article"
	"gator-cli/internal/database"
	"gator-cli/internal/text"
)

func handlerArticle(s *state, cmd command, user database.User) error {
	fs := flag.NewFlagSet("article", flag.ContinueOnError)
	refetch := fs.Bool("refetch", false, "fetch again even if the text is already stored")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: article <post_url> [--refetch]")
	}

	ctx := context.Background()
	post, err := s.Db.GetPostByUrl(ctx, fs.Args()[0])
	if err != nil {
		return fmt.Errorf("post not found: %w", err)
	}

	if post.FullText != "" && !*refetch {
		fmt.Println(post.FullText)
		return nil
	}

	got, err := article.Fetch(ctx, post.Url)
	if err != nil {
		return err
	}

	feedGave := text.StripHTML(post.Description.String)
	if !article.Improves(got.Text, feedGave) {
		return fmt.Errorf("%s has no more text than the feed already gave", post.Url)
	}

	if err := s.Db.SetPostFullText(ctx, database.SetPostFullTextParams{
		ID:       post.ID,
		FullText: got.Text,
	}); err != nil {
		return fmt.Errorf("saving article text: %w", err)
	}

	fmt.Println(got.Text)
	fmt.Fprintf(os.Stderr, "\n%d characters, where the feed gave %d\n",
		len([]rune(got.Text)), len([]rune(feedGave)))
	return nil
}
