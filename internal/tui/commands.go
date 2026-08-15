package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"gator-cli/internal/database"
	"gator-cli/internal/feeds"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type (
	postsLoadedMsg struct {
		posts  []database.Post
		offset int32
		// paged je false za rezultate pretrage, koja nema OFFSET.
		paged bool
	}
	bookmarksLoadedMsg struct{ postIDs []uuid.UUID }
	bookmarkToggledMsg struct {
		postID     uuid.UUID
		bookmarked bool
	}
	feedsLoadedMsg struct {
		feeds []database.GetFeedFollowsForUserRow
	}
	readsLoadedMsg  struct{ postIDs []uuid.UUID }
	unreadCountsMsg struct {
		counts []database.GetUnreadCountsForUserRow
	}
	readToggledMsg struct {
		postID uuid.UUID
		feedID uuid.UUID
		read   bool
	}
	allReadMsg   struct{ count int }
	feedAddedMsg struct {
		name    string
		created bool
	}
	feedUnfollowMsg struct{ name string }
	catalogAddedMsg struct {
		added  int
		known  int
		failed int
	}
	scrapedMsg struct {
		feeds     int
		saved     int
		failed    int
		unchanged int
	}
	openedMsg        struct{ url string }
	copiedMsg        struct{ url string }
	statusExpiredMsg struct{ token int }
	errMsg           struct{ err error }
)

func (e errMsg) Error() string { return e.err.Error() }

const (
	pageSize      = 50
	statusTimeout = 3 * time.Second
)

// postFilter skuplja sve sto odredjuje koji se postovi ucitavaju, da lista
// parametara loadPosts ne postane niz anonimnih argumenata.
type postFilter struct {
	feedID     uuid.UUID
	sortDir    string
	unreadOnly bool
	since      time.Time
}

func loadPosts(ctx context.Context, q *database.Queries, userID uuid.UUID, f postFilter, offset int32) tea.Cmd {
	return func() tea.Msg {
		posts, err := q.GetPostsForUserFiltered(ctx, database.GetPostsForUserFilteredParams{
			UserID:     userID,
			FeedID:     f.feedID,
			SortDir:    f.sortDir,
			UnreadOnly: f.unreadOnly,
			Since:      f.since,
			PostLimit:  pageSize,
			PostOffset: offset,
		})
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts, offset: offset, paged: true}
	}
}

func loadBookmarkedPosts(ctx context.Context, q *database.Queries, userID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		posts, err := q.GetBookmarksForUser(ctx, userID)
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
			PostLimit: pageSize,
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

func loadReads(ctx context.Context, q *database.Queries, userID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		ids, err := q.GetReadPostIDs(ctx, userID)
		if err != nil {
			return errMsg{err}
		}
		return readsLoadedMsg{postIDs: ids}
	}
}

func loadUnreadCounts(ctx context.Context, q *database.Queries, userID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		counts, err := q.GetUnreadCountsForUser(ctx, userID)
		if err != nil {
			return errMsg{err}
		}
		return unreadCountsMsg{counts: counts}
	}
}

func setPostRead(ctx context.Context, q *database.Queries, userID uuid.UUID, post database.Post, read bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if read {
			err = q.MarkPostRead(ctx, database.MarkPostReadParams{
				UserID: userID,
				PostID: post.ID,
				ReadAt: time.Now(),
			})
		} else {
			err = q.MarkPostUnread(ctx, database.MarkPostUnreadParams{
				UserID: userID,
				PostID: post.ID,
			})
		}
		if err != nil {
			return errMsg{err}
		}
		return readToggledMsg{postID: post.ID, feedID: post.FeedID, read: read}
	}
}

func markAllRead(ctx context.Context, q *database.Queries, userID uuid.UUID, postIDs []uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		if len(postIDs) == 0 {
			return allReadMsg{}
		}
		err := q.MarkPostsRead(ctx, database.MarkPostsReadParams{
			UserID:  userID,
			PostIds: postIDs,
			ReadAt:  time.Now(),
		})
		if err != nil {
			return errMsg{err}
		}
		return allReadMsg{count: len(postIDs)}
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

func addFeed(ctx context.Context, q *database.Queries, userID uuid.UUID, url string) tea.Cmd {
	return func() tea.Msg {
		feed, created, err := feeds.Add(ctx, q, userID, "", url)
		if err != nil {
			return errMsg{err}
		}
		return feedAddedMsg{name: feed.Name, created: created}
	}
}

// addCatalogFeeds povlaci sve izabrane feedove u jednoj komandi. Napredak po
// feedu se ne prikazuje: onResult se zove iz goroutine, a za slanje poruke u
// petlju treba tea.Program.Send, sto tea.Cmd ne moze sam.
func addCatalogFeeds(ctx context.Context, q *database.Queries, userID uuid.UUID, entries []feeds.Entry) tea.Cmd {
	return func() tea.Msg {
		var msg catalogAddedMsg
		for _, r := range feeds.AddMany(ctx, q, userID, entries, nil) {
			switch {
			case r.Err != nil:
				msg.failed++
			case r.Created:
				msg.added++
			default:
				msg.known++
			}
		}
		return msg
	}
}

func unfollowFeed(ctx context.Context, q *database.Queries, userID, feedID uuid.UUID, name string) tea.Cmd {
	return func() tea.Msg {
		err := q.DeleteFeedFollow(ctx, database.DeleteFeedFollowParams{
			UserID: userID,
			FeedID: feedID,
		})
		if err != nil {
			return errMsg{err}
		}
		return feedUnfollowMsg{name: name}
	}
}

func scrapeFeeds(ctx context.Context, q *database.Queries) tea.Cmd {
	return func() tea.Msg {
		results, err := feeds.ScrapeAll(ctx, q, nil)
		if err != nil {
			return errMsg{err}
		}

		msg := scrapedMsg{feeds: len(results)}
		for _, r := range results {
			switch {
			case r.Err != nil:
				msg.failed++
			case r.NotModified:
				msg.unchanged++
			default:
				msg.saved += r.Saved
			}
		}
		return msg
	}
}

func copyToClipboard(url string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(url); err != nil {
			return errMsg{fmt.Errorf("copying to clipboard: %w", err)}
		}
		return copiedMsg{url: url}
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
