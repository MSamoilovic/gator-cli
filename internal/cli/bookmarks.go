package cli

import (
	"context"
	"fmt"

	"github.com/MSamoilovic/gator-cli/internal/store"
)

func handlerBookmark(ctx context.Context, s *state, cmd command, _ store.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: bookmark <post_url>")
	}

	post, err := s.Store.PostByURL(ctx, cmd.Args[0])
	if err != nil {
		return fmt.Errorf("post not found: %w", err)
	}

	created, err := s.Store.Bookmark(ctx, post.ID)
	if err != nil {
		return fmt.Errorf("error bookmarking post: %w", err)
	}
	if !created {
		fmt.Printf("Already bookmarked %q\n", post.Title)
		return nil
	}

	fmt.Printf("Bookmarked %q\n", post.Title)
	return nil
}

func handlerUnbookmark(ctx context.Context, s *state, cmd command, _ store.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: unbookmark <post_url>")
	}

	post, err := s.Store.PostByURL(ctx, cmd.Args[0])
	if err != nil {
		return fmt.Errorf("post not found: %w", err)
	}

	if err := s.Store.Unbookmark(ctx, post.ID); err != nil {
		return fmt.Errorf("error removing bookmark: %w", err)
	}

	fmt.Printf("Removed bookmark for %q\n", post.Title)
	return nil
}

func handlerBookmarks(ctx context.Context, s *state, _ command, _ store.User) error {
	posts, err := s.Store.BookmarkedPosts(ctx)
	if err != nil {
		return fmt.Errorf("error fetching bookmarks: %w", err)
	}

	for _, p := range posts {
		printPost(p)
	}
	return nil
}
