package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

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
	switch m.source {
	case sourceBookmarks:
		return loadBookmarkedPosts(m.ctx, m.queries, m.userID)
	case sourceRead:
		return loadReadPosts(m.ctx, m.queries, m.userID)
	case sourceSearch:
		return searchPosts(m.ctx, m.queries, m.userID, m.filter.query)
	}
	return loadPosts(m.ctx, m.queries, m.userID, m.filter.params(), 0)
}

func (m *model) maybeLoadMore() tea.Cmd {
	switch {
	case !m.hasMore, m.loadingMore, m.source.derived():
		return nil
	case m.list.FilterState() == list.Filtering:
		return nil
	case m.list.Index() < len(m.list.Items())-loadMoreThreshold:
		return nil
	}

	m.loadingMore = true
	m.offset += pageSize
	return loadPosts(m.ctx, m.queries, m.userID, m.filter.params(), m.offset)
}

func (m *model) setPostsTitle() {
	switch m.source {
	case sourceBookmarks:
		m.list.Title = bookmarksTitle
		return
	case sourceRead:
		m.list.Title = readTitle
		return
	case sourceSearch:
		m.list.Title = "Search: " + m.filter.query
		return
	}

	if m.filter.feedName != "" {
		m.list.Title = m.filter.feedName
	} else {
		m.list.Title = postsTitle
	}
	if label := sinceLabel(m.filter.since); label != "" {
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
