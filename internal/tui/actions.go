package tui

import (
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
	if !m.inDerivedView() {
		return m, nil
	}
	m.query = ""
	m.showBookmarks = false
	m.setPostsTitle()
	return m, m.startLoad()
}

func (m model) reload() (tea.Model, tea.Cmd) {
	return m.statusAndReload("Reloading…")
}

func (m model) fetchAll() (tea.Model, tea.Cmd) {
	if m.fetching {
		return m, nil
	}
	m.fetching = true
	next, cmd := m.withStatus("Fetching feeds…")
	return next, tea.Batch(cmd, scrapeFeeds(next.ctx, next.queries))
}

func (m model) toggleBookmarksView() (tea.Model, tea.Cmd) {
	m.showBookmarks = !m.showBookmarks
	m.query = ""
	m.setPostsTitle()
	return m, m.startLoad()
}

func (m model) toggleSort() (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Sorting")
	}

	label := "oldest first"
	if m.sortDir == sortAsc {
		m.sortDir, label = sortDesc, "newest first"
	} else {
		m.sortDir = sortAsc
	}
	return m.statusAndReload("Sorted " + label)
}

func (m model) cycleTimeRange() (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Time range")
	}

	m.since = nextSince(m.since)
	m.setPostsTitle()

	label := "Showing all time"
	if l := sinceLabel(m.since); l != "" {
		label = "Showing the last " + l
	}
	return m.statusAndReload(label)
}

func (m model) toggleUnreadOnly() (tea.Model, tea.Cmd) {
	if m.inDerivedView() {
		return m.refuseDerived("Unread filter")
	}

	m.unreadOnly = !m.unreadOnly
	label := "Showing all posts"
	if m.unreadOnly {
		label = "Showing unread only"
	}
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
	return next, tea.Batch(cmd, setPostRead(next.ctx, next.queries, next.userID, post, read))
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
	return m, markAllRead(m.ctx, m.queries, m.userID, ids)
}
