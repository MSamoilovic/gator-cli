package tui

import (
	"github.com/MSamoilovic/gator-cli/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func (m model) statusAndReload(text string) (tea.Model, tea.Cmd) {
	next, cmd := m.withStatus(text)
	return next, tea.Batch(cmd, next.startLoad())
}

func (m model) refuseDerived(what string) (tea.Model, tea.Cmd) {
	return m.withStatus(what + " applies to feed posts only")
}

func (m model) focusFeedPanel() (tea.Model, tea.Cmd) {
	if m.feedWidth > 0 {
		m.focus = focusFeeds
		m.applyFocus()
	}
	return m, nil
}

func (m model) leaveDerivedView() (tea.Model, tea.Cmd) {
	if !m.inDerivedView() && m.filter.read == store.ReadAny {
		return m, nil
	}
	m.source = sourceFeeds
	m.filter.query = ""
	m.filter.read = store.ReadAny
	m.setPostsTitle()
	return m, m.startLoad()
}

func (m model) reload() (tea.Model, tea.Cmd) {
	return m.statusAndReload("Reloading…")
}

func (m model) fetchAll() (tea.Model, tea.Cmd) {
	if m.remote() {
		return m.withStatus(m.serverName() + " refreshes feeds on its own — press r to reload")
	}
	if m.fetching {
		return m, nil
	}
	m.fetching = true
	next, cmd := m.withStatus("Fetching feeds…")
	return next, tea.Batch(cmd, scrapeFeeds(next.ctx, next.store))
}

func (m model) toggleBookmarksView() (tea.Model, tea.Cmd) {
	return m.switchSource(sourceBookmarks)
}

func (m model) toggleReadView() (tea.Model, tea.Cmd) {
	return m.setReadState(store.ReadOnly, "Showing read posts")
}

func (m model) switchSource(want listSource) (tea.Model, tea.Cmd) {
	if m.source == want {
		m.source = sourceFeeds
	} else {
		m.source = want
	}
	m.filter.query = ""
	m.setPostsTitle()
	return m, m.startLoad()
}

func (m model) toggleSort() (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Sorting")
	}

	label := "oldest first"
	if m.filter.sortDir == sortAsc {
		m.filter.sortDir, label = sortDesc, "newest first"
	} else {
		m.filter.sortDir = sortAsc
	}
	return m.statusAndReload("Sorted " + label)
}

func (m model) cycleTimeRange() (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Time range")
	}

	m.filter.since = nextSince(m.filter.since)
	m.setPostsTitle()

	label := "Showing all time"
	if l := sinceLabel(m.filter.since); l != "" {
		label = "Showing the last " + l
	}
	return m.statusAndReload(label)
}

func (m model) toggleUnreadOnly() (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Unread filter")
	}

	return m.setReadState(store.ReadUnread, "Showing unread only")
}

func (m model) setReadState(want store.ReadState, label string) (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Read filter")
	}

	if m.filter.read == want {
		m.filter.read = store.ReadAny
		label = "Showing all posts"
	} else {
		m.filter.read = want
	}
	m.setPostsTitle()
	return m.statusAndReload(label)
}

func (m model) openSelected() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(postItem)
	if !ok {
		return m, nil
	}
	m.screen = screenDetail
	return m, m.openPost(item.post)
}

func (m model) copySelectedURL() (tea.Model, tea.Cmd) {
	post, ok := m.currentPost()
	if !ok {
		return m, nil
	}
	return m, copyToClipboard(post.Url)
}

func (m model) toggleSelectedRead() (tea.Model, tea.Cmd) {
	post, ok := m.currentPost()
	if !ok {
		return m, nil
	}

	read := !m.reads[post.ID]
	m.applyRead(post.ID, post.FeedID, read)

	label := "Marked unread"
	if read {
		label = "Marked read"
	}
	next, cmd := m.withStatus(label)
	return next, tea.Batch(cmd, setPostRead(next.ctx, next.store, post, read))
}

func (m model) markListRead() (tea.Model, tea.Cmd) {
	ids := make([]uuid.UUID, 0, len(m.list.Items()))
	for _, it := range m.list.Items() {
		pi, ok := it.(postItem)
		if !ok || m.reads[pi.post.ID] {
			continue
		}
		ids = append(ids, pi.post.ID)
		m.applyRead(pi.post.ID, pi.post.FeedID, true)
	}
	return m, markAllRead(m.ctx, m.store, ids)
}

func (m model) toggleSelectedBookmark() (tea.Model, tea.Cmd) {
	post, ok := m.currentPost()
	if !ok {
		return m, nil
	}
	if m.bookmarks[post.ID] {
		return m, removeBookmark(m.ctx, m.store, post.ID)
	}
	return m, addBookmark(m.ctx, m.store, post.ID)
}

func (m model) openSelectedInBrowser() (tea.Model, tea.Cmd) {
	post, ok := m.currentPost()
	if !ok {
		return m, nil
	}
	return m, openInBrowser(post.Url)
}

func (m model) selectedFeed() (feedItem, bool) {
	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok || item.id == uuid.Nil {
		return feedItem{}, false
	}
	return item, true
}

func (m model) askUnfollow() (tea.Model, tea.Cmd) {
	if _, ok := m.selectedFeed(); !ok {
		return m.withStatus("Pick a feed in the feed pane first")
	}
	m.confirming = true
	m.focus = focusFeeds
	m.applyFocus()
	return m, nil
}

func (m model) promptCategory() (tea.Model, tea.Cmd) {
	item, ok := m.selectedFeed()
	if !ok {
		return m.withStatus("Pick a feed in the feed pane first")
	}
	return m, m.startInput(inputCategorize, "move "+item.name+" to folder: ")
}

func (m model) applyCategory(folder string) (tea.Model, tea.Cmd) {
	item, ok := m.selectedFeed()
	if !ok {
		return m.withStatus("Pick a feed in the feed pane first")
	}
	next, cmd := m.withStatus("Moving " + item.name + " to " + folder + "…")
	return next, tea.Batch(cmd, categorizeFeed(next.ctx, next.store, item.url, folder))
}
