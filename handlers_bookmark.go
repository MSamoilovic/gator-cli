package main

import (
	"context"
	"fmt"
	"time"

	"gator-ci/internal/database"

	"github.com/google/uuid"
)

func handlerBookmark(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: bookmark <post_url>")
	}

	post, err := s.Db.GetPostByUrl(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("post not found: %v", err)
	}

	_, err = s.Db.CreateBookmark(context.Background(), database.CreateBookmarkParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UserID:    user.ID,
		PostID:    post.ID,
	})
	if err != nil {
		return fmt.Errorf("error bookmarking post: %v", err)
	}

	fmt.Printf("Bookmarked %q\n", post.Title)
	return nil
}

func handlerUnbookmark(s *state, cmd command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: unbookmark <post_url>")
	}

	post, err := s.Db.GetPostByUrl(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("post not found: %v", err)
	}

	if err := s.Db.DeleteBookmark(context.Background(), database.DeleteBookmarkParams{
		UserID: user.ID,
		PostID: post.ID,
	}); err != nil {
		return fmt.Errorf("error removing bookmark: %v", err)
	}

	fmt.Printf("Removed bookmark for %q\n", post.Title)
	return nil
}

func handlerBookmarks(s *state, _ command, user database.User) error {
	posts, err := s.Db.GetBookmarksForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error fetching bookmarks: %v", err)
	}

	for _, p := range posts {
		fmt.Printf("--- %s ---\n", p.Title)
		fmt.Printf("URL: %s\n", p.Url)
		if p.Description.Valid {
			fmt.Printf("%s\n", p.Description.String)
		}
		fmt.Println()
	}
	return nil
}
