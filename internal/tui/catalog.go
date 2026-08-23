package tui

import (
	"strconv"

	"gator-cli/internal/catalog"
	"gator-cli/internal/feeds"

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
		items[i] = catalogItem{cat: c, picked: m.picked, followed: countFollowed(c, followed)}
	}

	m.screen = screenCatalog
	m.catalogList.ResetSelected()
	return m, m.catalogList.SetItems(items)
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
		
		if m.picked[item.cat.ID] {
			delete(m.picked, item.cat.ID)
		} else {
			m.picked[item.cat.ID] = true
		}
		return m, nil

	case key.Matches(msg, m.keys.Select):
		return m.confirmCatalog()
	}

	var cmd tea.Cmd
	m.catalogList, cmd = m.catalogList.Update(msg)
	return m, cmd
}

func (m model) confirmCatalog() (tea.Model, tea.Cmd) {
	var entries []feeds.Entry
	seen := make(map[string]bool)

	for _, it := range m.catalogList.Items() {
		item, ok := it.(catalogItem)
		if !ok || !m.picked[item.cat.ID] {
			continue
		}
		for _, f := range item.cat.Feeds {
			if seen[f.URL] {
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
		return m.withStatus("Nothing picked — press space to pick a category")
	}

	m.screen = screenList
	next, cmd := m.withStatus("Adding " + strconv.Itoa(len(entries)) + " feeds…")
	return next, tea.Batch(cmd, addCatalogFeeds(next.ctx, next.queries, next.userID, entries))
}

func (m model) catalogView() string {
	return m.catalogList.View() + "\n" + m.footer()
}

// followedURLs se cita iz m.feeds, a ne iz feed liste: u listi nema feedova iz
// sklopljenih foldera, pa bi katalog za njih tvrdio da se ne prate.
func (m model) followedURLs() map[string]bool {
	followed := make(map[string]bool, len(m.feeds))
	for _, f := range m.feeds {
		followed[f.FeedUrl] = true
	}
	return followed
}

func countFollowed(c catalog.Category, followed map[string]bool) int {
	n := 0
	for _, f := range c.Feeds {
		if followed[f.URL] {
			n++
		}
	}
	return n
}
