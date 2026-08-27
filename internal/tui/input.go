package tui

import (
	"strings"

	"gator-cli/internal/database"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

func (m model) postAction(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Open):
		post, ok := m.currentPost()
		if !ok {
			return m, nil, true
		}
		return m, openInBrowser(post.Url), true

	case key.Matches(msg, m.keys.Bookmark):
		post, ok := m.currentPost()
		if !ok {
			return m, nil, true
		}
		if m.bookmarks[post.ID] {
			return m, removeBookmark(m.ctx, m.queries, m.userID, post.ID), true
		}
		return m, addBookmark(m.ctx, m.queries, m.userID, post.ID), true
	}
	return m, nil, false
}

func (m model) toggleHelp() (tea.Model, tea.Cmd) {
	m.help.ShowAll = !m.help.ShowAll
	m.resize(m.width, m.height)
	return m, nil
}

func (m *model) startInput(mode inputMode, prompt string) tea.Cmd {
	m.input = mode
	m.prompt.Prompt = prompt
	m.prompt.SetValue("")
	m.prompt.Width = max(m.width-len(prompt)-1, 1)
	return m.prompt.Focus()
}

func (m *model) cancelInput() {
	m.input = inputNone
	m.prompt.Blur()
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.cancelInput()
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEnter:
		mode := m.input
		value := strings.TrimSpace(m.prompt.Value())
		m.cancelInput()
		if value == "" {
			return m, nil
		}

		switch mode {
		case inputSearch:
			m.query = value
			m.showBookmarks = false
			m.setPostsTitle()
			return m, searchPosts(m.ctx, m.queries, m.userID, value)

		case inputAddFeed:
			next, cmd := m.withStatus("Adding " + value + "…")
			return next, tea.Batch(cmd, addFeed(next.ctx, next.queries, next.userID, value))
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m model) confirmText() string {
	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok {
		return ""
	}
	return "Unfollow " + item.name + "? (y/n)"
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirming = false

	if msg.String() != "y" {
		return m.withStatus("Cancelled")
	}

	item, ok := m.feedList.SelectedItem().(feedItem)
	if !ok || item.id == uuid.Nil {
		return m, nil
	}
	return m, unfollowFeed(m.ctx, m.queries, m.userID, item.id, item.name)
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()
	case key.Matches(msg, m.keys.Back), msg.String() == "q":
		m.screen = screenList
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Copy):
		return m, copyToClipboard(m.selected.Url)

	case key.Matches(msg, m.keys.Next):
		return m, m.stepPost(1)

	case key.Matches(msg, m.keys.Prev):
		return m, m.stepPost(-1)
	}

	if next, cmd, handled := m.postAction(msg); handled {
		return next, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateFeeds(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()

	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Back):
		m.focus = focusPosts
		m.applyFocus()
		return m, nil

	case key.Matches(msg, m.keys.AddFeed):
		return m, m.startInput(inputAddFeed, "feed url: ")

	case key.Matches(msg, m.keys.Catalog):
		next, cmd := m.openCatalog()
		return next, cmd

	case key.Matches(msg, m.keys.Unfollow):
		item, ok := m.feedList.SelectedItem().(feedItem)
		if !ok {
			return m.withStatus("Pick a feed to unfollow")
		}
		if item.id == uuid.Nil {
			return m.withStatus("Pick a feed to unfollow")
		}
		m.confirming = true
		return m, nil

	case key.Matches(msg, m.keys.Select):
		if fol, ok := m.feedList.SelectedItem().(folderItem); ok {
			return m.toggleFolder(fol.name)
		}

		item, ok := m.feedList.SelectedItem().(feedItem)
		if !ok {
			return m, nil
		}
		m.feedID = item.id
		m.feedName = ""
		if item.id != uuid.Nil {
			m.feedName = item.name
		}
		m.query = ""
		m.showBookmarks = false
		m.setPostsTitle()
		m.focus = focusPosts
		m.applyFocus()
		return m, m.startLoad()
	}

	var cmd tea.Cmd
	m.feedList, cmd = m.feedList.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			return m.toggleHelp()
		case key.Matches(msg, m.keys.Tab):
			return m.focusFeedPanel()
		case key.Matches(msg, m.keys.Search):
			return m, m.startInput(inputSearch, "search: ")
		case key.Matches(msg, m.keys.Back):
			return m.leaveDerivedView()
		case key.Matches(msg, m.keys.Reload):
			return m.reload()
		case key.Matches(msg, m.keys.Fetch):
			return m.fetchAll()
		case key.Matches(msg, m.keys.Saved):
			return m.toggleBookmarksView()
		case key.Matches(msg, m.keys.Sort):
			return m.toggleSort()
		case key.Matches(msg, m.keys.Since):
			return m.cycleTimeRange()
		case key.Matches(msg, m.keys.OnlyNew):
			return m.toggleUnreadOnly()
		case key.Matches(msg, m.keys.Read):
			return m.openSelected()
		case key.Matches(msg, m.keys.Copy):
			return m.copySelectedURL()
		case key.Matches(msg, m.keys.Unread):
			return m.toggleSelectedRead()
		case key.Matches(msg, m.keys.AllRead):
			return m.markListRead()
		}

		if next, cmd, handled := m.postAction(msg); handled {
			return next, cmd
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, tea.Batch(cmd, m.maybeLoadMore())
}

func (m *model) openPost(post database.Post) tea.Cmd {
	m.selected = post
	m.viewport.SetContent(renderDetailBody(post, m.viewport.Width))
	m.viewport.GotoTop()

	if m.reads[post.ID] {
		return nil
	}
	m.applyRead(post.ID, post.FeedID, true)
	return setPostRead(m.ctx, m.queries, m.userID, post, true)
}

func (m *model) stepPost(delta int) tea.Cmd {
	next := m.list.Index() + delta
	if next < 0 || next >= len(m.list.Items()) {
		return nil
	}
	m.list.Select(next)

	item, ok := m.list.SelectedItem().(postItem)
	if !ok {
		return nil
	}
	return m.openPost(item.post)
}
