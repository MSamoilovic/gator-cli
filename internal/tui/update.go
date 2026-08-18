package tui

import (
	"fmt"

	"gator-cli/internal/database"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case postsLoadedMsg:
		return m.postsLoaded(msg)

	case feedsLoadedMsg:
		m.feedsLoaded = true
		m.feedCount = len(msg.feeds)
		cmd := m.feedList.SetItems(m.feedItems(msg.feeds))

		// Katalog se otvara tek ovde, ne u Init: openCatalog cita feed listu
		// da bi znao sta se vec prati, a ona do sada nije bila ucitana.
		if m.openOnLoad {
			m.openOnLoad = false
			opened, openCmd := m.openCatalog()
			m = opened
			cmd = tea.Batch(cmd, openCmd)
		}
		return m.selectStoredFeed(cmd)

	case readsLoadedMsg:
		for _, id := range msg.postIDs {
			m.reads[id] = true
		}
		return m, nil

	case unreadCountsMsg:
		clear(m.unread)
		for _, c := range msg.counts {
			m.unread[c.FeedID] = int(c.Unread)
		}
		return m, nil

	case readToggledMsg:
		m.applyRead(msg.postID, msg.feedID, msg.read)
		return m, nil

	case allReadMsg:
		if msg.count == 0 {
			return m.withStatus("Nothing to mark")
		}
		next, cmd := m.withStatus(fmt.Sprintf("Marked %d posts read", msg.count))
		if next.unreadOnly {
			return next, tea.Batch(cmd, next.startLoad())
		}
		return next, cmd

	case bookmarksLoadedMsg:
		for _, id := range msg.postIDs {
			m.bookmarks[id] = true
		}
		return m, nil

	case bookmarkToggledMsg:
		if msg.bookmarked {
			m.bookmarks[msg.postID] = true
			return m.withStatus("Bookmarked ★")
		}
		delete(m.bookmarks, msg.postID)
		next, cmd := m.withStatus("Bookmark removed")

		if next.showBookmarks {
			return next, tea.Batch(cmd, loadBookmarkedPosts(next.ctx, next.queries, next.userID))
		}
		return next, cmd

	case feedAddedMsg:
		label := "Now following " + msg.name
		if msg.created {
			label = "Added " + msg.name
		}
		next, cmd := m.withStatus(label)
		return next, tea.Batch(cmd, next.reloadFeeds(), next.startLoad())

	case feedUnfollowMsg:
		if m.feedName == msg.name {
			m.feedID = uuid.Nil
			m.feedName = ""
			m.setPostsTitle()
			m.feedList.ResetSelected()
		}
		next, cmd := m.withStatus("Unfollowed " + msg.name)
		return next, tea.Batch(cmd, next.reloadFeeds(), next.startLoad())

	case catalogAddedMsg:
		next, cmd := m.withStatus(catalogSummary(msg))
		if msg.added+msg.known == 0 {
			return next, cmd
		}
		return next, tea.Batch(cmd, next.reloadFeeds(), next.startLoad())

	case scrapedMsg:
		m.fetching = false
		next, cmd := m.withStatus(scrapeSummary(msg))
		return next, tea.Batch(cmd, next.startLoad())

	case openedMsg:
		return m.withStatus("Opened " + msg.url)

	case copiedMsg:
		return m.withStatus("Copied " + msg.url)

	case statusExpiredMsg:
		if msg.token == m.statusToken {
			m.status = ""
		}
		return m, nil

	case errMsg:
		m.loadingMore = false
		m.fetching = false
		if m.loading {
			m.loading = false
			m.err = msg.err
			return m, nil
		}
		return m.withStatus("Error: " + msg.err.Error())

	case tea.KeyMsg:
		switch {
		case m.confirming:
			return m.updateConfirm(msg)
		case m.input != inputNone:
			return m.updateInput(msg)
		case m.screen == screenCatalog:
			return m.updateCatalog(msg)
		case m.screen == screenDetail:
			return m.updateDetail(msg)
		case m.focus == focusFeeds:
			return m.updateFeeds(msg)
		default:
			return m.updateList(msg)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) postsLoaded(msg postsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.paged && msg.offset != m.offset {
		return m, nil
	}

	m.loading = false
	m.loadingMore = false
	m.hasMore = msg.paged && len(msg.posts) == pageSize

	items := make([]list.Item, len(msg.posts))
	for i, p := range msg.posts {
		items[i] = postItem{post: p, bookmarks: m.bookmarks, reads: m.reads}
	}

	if msg.paged && msg.offset > 0 {
		return m, m.list.SetItems(append(m.list.Items(), items...))
	}

	m.offset = 0
	m.list.ResetSelected()
	return m, m.list.SetItems(items)
}

func (m model) selectStoredFeed(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	if m.feedID == uuid.Nil {
		return m, cmd
	}

	for i, it := range m.feedList.Items() {
		if fi, ok := it.(feedItem); ok && fi.id == m.feedID {
			m.feedList.Select(i)
			m.feedName = fi.name
			m.setPostsTitle()
			return m, cmd
		}
	}

	m.feedID = uuid.Nil
	m.feedName = ""
	m.setPostsTitle()
	return m, tea.Batch(cmd, m.startLoad())
}

func (m model) feedItems(feeds []database.GetFeedFollowsForUserRow) []list.Item {
	items := make([]list.Item, 0, len(feeds)+1)
	items = append(items, feedItem{id: uuid.Nil, name: allFeedsLabel, unread: m.unread})
	for _, f := range feeds {
		items = append(items, feedItem{
			id:       f.FeedID,
			name:     f.FeedName,
			url:      f.FeedUrl,
			failures: f.FeedFailures,
			unread:   m.unread,
		})
	}
	return items
}

func (m *model) applyRead(postID, feedID uuid.UUID, read bool) {
	if m.reads[postID] == read {
		return
	}
	if read {
		m.reads[postID] = true
		if m.unread[feedID] > 0 {
			m.unread[feedID]--
		}
		return
	}
	delete(m.reads, postID)
	m.unread[feedID]++
}

func catalogSummary(msg catalogAddedMsg) string {
	switch {
	case msg.added+msg.known == 0:
		return fmt.Sprintf("No feed could be added (%d failed)", msg.failed)
	case msg.failed > 0:
		return fmt.Sprintf("Following %d feeds · %d unreachable", msg.added+msg.known, msg.failed)
	default:
		return fmt.Sprintf("Following %d feeds — press R to fetch", msg.added+msg.known)
	}
}

func scrapeSummary(msg scrapedMsg) string {
	switch {
	case msg.feeds == 0:
		return "No feeds were due for a fetch"
	case msg.failed > 0:
		return fmt.Sprintf("Fetched %d new posts · %d feed(s) failed", msg.saved, msg.failed)
	case msg.unchanged > 0:
		return fmt.Sprintf("Fetched %d new posts · %d unchanged", msg.saved, msg.unchanged)
	default:
		return fmt.Sprintf("Fetched %d new posts from %d feeds", msg.saved, msg.feeds)
	}
}
