package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) filter() postFilter {
	f := postFilter{
		feedID:     m.feedID,
		sortDir:    m.sortDir,
		unreadOnly: m.unreadOnly,
	}
	if m.since > 0 {
		f.since = time.Now().Add(-m.since)
	}
	return f
}

func (m model) reloadFeeds() tea.Cmd {
	return tea.Batch(
		loadFeeds(m.ctx, m.queries, m.userID),
		loadUnreadCounts(m.ctx, m.queries, m.userID),
	)
}

func (m *model) startLoad() tea.Cmd {
	m.offset = 0
	m.hasMore = true
	m.loadingMore = false
	if m.showBookmarks {
		return loadBookmarkedPosts(m.ctx, m.queries, m.userID)
	}
	return loadPosts(m.ctx, m.queries, m.userID, m.filter(), 0)
}

func (m *model) maybeLoadMore() tea.Cmd {
	switch {
	case !m.hasMore, m.loadingMore, m.query != "", m.showBookmarks:
		return nil
	case m.list.FilterState() == list.Filtering:
		return nil
	case m.list.Index() < len(m.list.Items())-loadMoreThreshold:
		return nil
	}

	m.loadingMore = true
	m.offset += pageSize
	return loadPosts(m.ctx, m.queries, m.userID, m.filter(), m.offset)
}

func (m *model) setPostsTitle() {
	switch {
	case m.showBookmarks:
		m.list.Title = bookmarksTitle
		return
	case m.query != "":
		m.list.Title = "Search: " + m.query
		return
	case m.feedName != "":
		m.list.Title = m.feedName
	default:
		m.list.Title = postsTitle
	}
	if label := sinceLabel(m.since); label != "" {
		m.list.Title += " · " + label
	}
}

var sinceRanges = []time.Duration{0, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}

func sinceLabel(d time.Duration) string {
	switch d {
	case 24 * time.Hour:
		return "24h"
	case 7 * 24 * time.Hour:
		return "7d"
	case 30 * 24 * time.Hour:
		return "30d"
	default:
		return ""
	}
}

func nextSince(d time.Duration) time.Duration {
	for i, r := range sinceRanges {
		if r == d {
			return sinceRanges[(i+1)%len(sinceRanges)]
		}
	}
	return 0
}
