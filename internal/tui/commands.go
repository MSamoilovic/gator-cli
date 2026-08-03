package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type (
	postsLoadedMsg     struct{ posts []database.Post }
	bookmarksLoadedMsg struct{ postIDs []uuid.UUID }
	bookmarkToggledMsg struct {
		postID     uuid.UUID
		bookmarked bool
	}
	feedsLoadedMsg struct {
		feeds []database.GetFeedFollowsForUserRow
	}
	openedMsg        struct{ url string }
	statusExpiredMsg struct{ token int }
	errMsg           struct{ err error }
)

func (e errMsg) Error() string { return e.err.Error() }

const (
	postsLimit    = 100
	statusTimeout = 3 * time.Second
)

func loadPosts(ctx context.Context, q *database.Queries, userID, feedID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		posts, err := q.GetPostsForUserFiltered(ctx, database.GetPostsForUserFilteredParams{
			UserID:    userID,
			FeedID:    feedID,
			SortDir:   "desc",
			PostLimit: postsLimit,
		})
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts}
	}
}

func searchPosts(ctx context.Context, q *database.Queries, userID uuid.UUID, query string) tea.Cmd {
	return func() tea.Msg {
		posts, err := q.SearchPostsForUser(ctx, database.SearchPostsForUserParams{
			UserID:    userID,
			Query:     query,
			PostLimit: postsLimit,
		})
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts}
	}
}

func loadFeeds(ctx context.Context, q *database.Queries, userID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		feeds, err := q.GetFeedFollowsForUser(ctx, userID)
		if err != nil {
			return errMsg{err}
		}
		return feedsLoadedMsg{feeds: feeds}
	}
}

func loadBookmarks(ctx context.Context, q *database.Queries, userID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		ids, err := q.GetBookmarkedPostIDs(ctx, userID)
		if err != nil {
			return errMsg{err}
		}
		return bookmarksLoadedMsg{postIDs: ids}
	}
}

func addBookmark(ctx context.Context, q *database.Queries, userID, postID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		_, err := q.CreateBookmark(ctx, database.CreateBookmarkParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UserID:    userID,
			PostID:    postID,
		})
		if err != nil {
			return errMsg{err}
		}
		return bookmarkToggledMsg{postID: postID, bookmarked: true}
	}
}

func removeBookmark(ctx context.Context, q *database.Queries, userID, postID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		err := q.DeleteBookmark(ctx, database.DeleteBookmarkParams{
			UserID: userID,
			PostID: postID,
		})
		if err != nil {
			return errMsg{err}
		}
		return bookmarkToggledMsg{postID: postID, bookmarked: false}
	}
}

func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			return errMsg{fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)}
		}

		if err := cmd.Start(); err != nil {
			return errMsg{fmt.Errorf("opening %s: %w", url, err)}
		}
		go cmd.Wait()

		return openedMsg{url: url}
	}
}

func expireStatus(token int) tea.Cmd {
	return tea.Tick(statusTimeout, func(time.Time) tea.Msg {
		return statusExpiredMsg{token: token}
	})
}
