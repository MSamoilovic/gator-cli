package tui

import (
	"strconv"

	"github.com/MSamoilovic/gator-cli/internal/catalog"
	"github.com/MSamoilovic/gator-cli/internal/feeds"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

const catalogTitle = "Pick your interests"

func (m model) openCatalog() (model, tea.Cmd) {
	cats, err := catalog.Categories()
	if err != nil {
		return m.withStatus("Error: " + err.Error())
	}

	followed := m.followedURLs()
	clear(m.picked)

	items := make([]list.Item, len(cats))
	for i, c := range cats {
		items[i] = catalogItem{cat: c, picked: m.picked, followed: followed}
	}

	m.screen = screenCatalog
	m.catalogList.ResetSelected()
	cmd := m.catalogList.SetItems(items)
	m.catalogList.SetSize(m.width, m.panelHeight())
	return m, cmd
}

func (m model) openCatalogCategory(item catalogItem) (model, tea.Cmd) {
	items := make([]list.Item, len(item.cat.Feeds))
	for i, f := range item.cat.Feeds {
		items[i] = catalogFeedItem{feed: f, picked: m.picked, followed: item.followed}
	}

	m.catalogOpen = item.cat
	m.screen = screenCatalogFeeds
	m.catalogFeeds.Title = item.cat.Label + " — pick feeds"
	m.catalogFeeds.ResetSelected()
	cmd := m.catalogFeeds.SetItems(items)
	m.catalogFeeds.SetSize(m.width, m.panelHeight())
	return m, cmd
}

func (m model) updateCatalog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC, key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()

	case key.Matches(msg, m.keys.Back):
		m.screen = screenList
		return m, nil

	case key.Matches(msg, m.keys.Toggle):
		item, ok := m.catalogList.SelectedItem().(catalogItem)
		if !ok {
			return m, nil
		}
		m.toggleCategory(item)
		return m, nil

	case key.Matches(msg, m.keys.Select), msg.Type == tea.KeyRight, msg.String() == "l":
		item, ok := m.catalogList.SelectedItem().(catalogItem)
		if !ok {
			return m, nil
		}
		return m.openCatalogCategory(item)

	case key.Matches(msg, m.keys.Confirm):
		return m.confirmCatalog()
	}

	var cmd tea.Cmd
	m.catalogList, cmd = m.catalogList.Update(msg)
	return m, cmd
}

func (m model) updateCatalogFeeds(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC, key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		return m.toggleHelp()

	case key.Matches(msg, m.keys.Back), msg.Type == tea.KeyLeft, msg.String() == "h":
		m.screen = screenCatalog
		return m, nil

	case key.Matches(msg, m.keys.Toggle), key.Matches(msg, m.keys.Select):
		item, ok := m.catalogFeeds.SelectedItem().(catalogFeedItem)
		if !ok {
			return m, nil
		}
		return m.toggleFeed(item)

	case key.Matches(msg, m.keys.Confirm):
		return m.confirmCatalog()
	}

	var cmd tea.Cmd
	m.catalogFeeds, cmd = m.catalogFeeds.Update(msg)
	return m, cmd
}

func (m model) toggleCategory(item catalogItem) {
	var open []catalog.Feed
	allPicked := true
	for _, f := range item.cat.Feeds {
		if item.followed[f.URL] {
			continue
		}
		open = append(open, f)
		allPicked = allPicked && m.picked[f.URL]
	}

	for _, f := range open {
		if allPicked {
			delete(m.picked, f.URL)
		} else {
			m.picked[f.URL] = true
		}
	}
}

func (m model) toggleFeed(item catalogFeedItem) (tea.Model, tea.Cmd) {
	switch {
	case item.followed[item.feed.URL]:
		return m.withStatus("Already following " + item.feed.Name)
	case m.picked[item.feed.URL]:
		delete(m.picked, item.feed.URL)
	default:
		m.picked[item.feed.URL] = true
	}
	return m, nil
}

func (m model) confirmCatalog() (tea.Model, tea.Cmd) {
	var entries []feeds.Entry
	seen := make(map[string]bool)

	for _, it := range m.catalogList.Items() {
		item, ok := it.(catalogItem)
		if !ok {
			continue
		}
		for _, f := range item.cat.Feeds {
			if !m.picked[f.URL] || seen[f.URL] {
				continue
			}
			seen[f.URL] = true
			entries = append(entries, feeds.Entry{
				Name:     f.Name,
				URL:      f.URL,
				Category: item.cat.Label,
			})
		}
	}

	if len(entries) == 0 {
		return m.withStatus("Nothing picked — press space to pick a feed or a whole category")
	}

	m.screen = screenList
	next, cmd := m.withStatus("Adding " + strconv.Itoa(len(entries)) + " feeds…")
	return next, tea.Batch(cmd, addCatalogFeeds(next.ctx, next.store, entries))
}

func (m model) catalogView() string {
	return m.catalogList.View() + "\n" + m.footer()
}

func (m model) catalogFeedsView() string {
	return m.catalogFeeds.View() + "\n" + m.footer()
}

func (m model) followedURLs() map[string]bool {
	followed := make(map[string]bool, len(m.feeds))
	for _, f := range m.feeds {
		followed[f.FeedUrl] = true
	}
	return followed
}
