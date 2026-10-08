package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/feeds"
	"github.com/MSamoilovic/gator-cli/internal/store"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type (
	postsLoadedMsg struct {
		posts  []database.Post
		offset int32
		paged  bool
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
	fullTextMsg struct {
		postID uuid.UUID
		body   string
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

type postFilter struct {
	feedID     uuid.UUID
	sortDir    string
	unreadOnly bool
	since      time.Time
}

func loadPosts(ctx context.Context, st store.Store, f postFilter, offset int32) tea.Cmd {
	return func() tea.Msg {
		posts, err := st.Posts(ctx, store.PostQuery{
			FeedID:     f.feedID,
			SortDir:    f.sortDir,
			UnreadOnly: f.unreadOnly,
			Since:      f.since,
			Limit:      pageSize,
			Offset:     offset,
		})
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts, offset: offset, paged: true}
	}
}

func loadBookmarkedPosts(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		posts, err := st.BookmarkedPosts(ctx)
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts}
	}
}

func searchPosts(ctx context.Context, st store.Store, query string) tea.Cmd {
	return func() tea.Msg {
		posts, err := st.Posts(ctx, store.PostQuery{Query: query, Limit: pageSize})
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts}
	}
}

func loadFeeds(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		subs, err := st.Subscriptions(ctx)
		if err != nil {
			return errMsg{err}
		}
		return feedsLoadedMsg{feeds: subs}
	}
}

func loadBookmarks(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		ids, err := st.BookmarkedIDs(ctx)
		if err != nil {
			return errMsg{err}
		}
		return bookmarksLoadedMsg{postIDs: ids}
	}
}

func loadReadPosts(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		posts, err := st.ReadPosts(ctx)
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts}
	}
}

func loadReads(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		ids, err := st.ReadIDs(ctx)
		if err != nil {
			return errMsg{err}
		}
		return readsLoadedMsg{postIDs: ids}
	}
}

func loadUnreadCounts(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		counts, err := st.UnreadCounts(ctx)
		if err != nil {
			return errMsg{err}
		}
		return unreadCountsMsg{counts: counts}
	}
}

func setPostRead(ctx context.Context, st store.Store, post database.Post, read bool) tea.Cmd {
	return func() tea.Msg {
		if err := st.SetRead(ctx, post.ID, read); err != nil {
			return errMsg{err}
		}
		return readToggledMsg{postID: post.ID, feedID: post.FeedID, read: read}
	}
}

func markAllRead(ctx context.Context, st store.Store, postIDs []uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		if len(postIDs) == 0 {
			return allReadMsg{}
		}
		if err := st.SetAllRead(ctx, postIDs); err != nil {
			return errMsg{err}
		}
		return allReadMsg{count: len(postIDs)}
	}
}

func addBookmark(ctx context.Context, st store.Store, postID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		if _, err := st.Bookmark(ctx, postID); err != nil {
			return errMsg{err}
		}
		return bookmarkToggledMsg{postID: postID, bookmarked: true}
	}
}

func removeBookmark(ctx context.Context, st store.Store, postID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		if err := st.Unbookmark(ctx, postID); err != nil {
			return errMsg{err}
		}
		return bookmarkToggledMsg{postID: postID, bookmarked: false}
	}
}

func addFeed(ctx context.Context, st store.Store, url string) tea.Cmd {
	return func() tea.Msg {
		feed, created, err := st.AddFeed(ctx, "", url)
		if err != nil {
			return errMsg{err}
		}
		return feedAddedMsg{name: feed.Name, created: created}
	}
}

func addCatalogFeeds(ctx context.Context, st store.Store, entries []feeds.Entry) tea.Cmd {
	return func() tea.Msg {
		var msg catalogAddedMsg
		for _, r := range st.AddFeeds(ctx, entries, nil) {
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

func unfollowFeed(ctx context.Context, st store.Store, feedID uuid.UUID, name string) tea.Cmd {
	return func() tea.Msg {
		if err := st.Unfollow(ctx, feedID); err != nil {
			return errMsg{err}
		}
		return feedUnfollowMsg{name: name}
	}
}

func scrapeFeeds(ctx context.Context, st store.Store) tea.Cmd {
	return func() tea.Msg {
		got, err := st.Refresh(ctx)
		if err != nil {
			return errMsg{err}
		}
		return scrapedMsg{
			feeds:     got.Feeds,
			saved:     got.Saved,
			failed:    got.Failed,
			unchanged: got.Unchanged,
		}
	}
}

func fetchFullText(ctx context.Context, st store.Store, post database.Post) tea.Cmd {
	return func() tea.Msg {
		body, err := st.FullText(ctx, post)
		if err != nil {
			return errMsg{err}
		}
		return fullTextMsg{postID: post.ID, body: body}
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
