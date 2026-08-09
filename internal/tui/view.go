package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	switch {
	case m.err != nil:
		return "Error: " + m.err.Error() + "\n\nPress q to quit.\n"
	case m.loading:
		return "\n  " + m.spinner.View() + " Loading posts...\n"
	case m.showDetail:
		return m.detailView()
	default:
		return m.panelsView()
	}
}

func (m model) panelsView() string {
	if m.feedWidth == 0 {
		return m.postsPanel() + "\n" + m.footer()
	}

	panels := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.feedList.View(),
		verticalRule(max(m.height-m.footerHeight(), 1)),
		m.postsPanel(),
	)
	return panels + "\n" + m.footer()
}

func (m model) postsPanel() string {
	if len(m.list.Items()) > 0 {
		return m.list.View()
	}

	body := emptyStateStyle.Render(m.emptyStateText())
	panel := m.list.Styles.Title.Render(m.list.Title) + "\n\n" + body

	return lipgloss.NewStyle().
		Width(m.list.Width()).
		Height(max(m.height-m.footerHeight(), 1)).
		MaxWidth(m.list.Width()).
		Render(panel)
}

func (m model) emptyStateText() string {
	switch {
	case m.showBookmarks:
		return "No bookmarks yet.\nPress b on a post to save it."
	case m.query != "":
		return "No posts match " + strconv.Quote(m.query) + ".\nPress esc to go back."
	case m.feedsLoaded && m.feedCount == 0:
		return "You are not following any feeds.\nAdd one with: gator addfeed <name> <url>"
	case m.feedName != "":
		return "No posts stored for " + m.feedName + " yet.\nFetch some with: gator agg 15m"
	default:
		return "No posts yet.\nFetch some with: gator agg 15m"
	}
}

func verticalRule(height int) string {
	return strings.TrimSuffix(strings.Repeat("│\n", height), "\n")
}

func (m model) detailView() string {
	return renderDetailHeader(m.selected, m.viewport.Width, m.viewport.ScrollPercent()) +
		m.viewport.View() +
		"\n\n" + m.footer()
}

func (m model) currentBindings() []key.Binding {
	switch {
	case m.showDetail:
		return m.keys.detailHelp()
	case m.focus == focusFeeds:
		return m.keys.feedsHelp()
	default:
		return m.keys.listHelp(m.feedWidth > 0, m.inDerivedView())
	}
}

func (m model) footerHeight() int {
	if !m.help.ShowAll {
		return 1
	}
	return lipgloss.Height(m.help.FullHelpView(m.keys.fullHelp()))
}

func (m model) footer() string {
	var line string
	switch {
	case m.confirming:
		line = m.confirmText()
	case m.input != inputNone:
		line = m.prompt.View()
	case m.status != "":
		line = m.status
	case m.help.ShowAll:
		line = m.help.FullHelpView(m.keys.fullHelp())
	default:
		line = m.help.ShortHelpView(m.currentBindings())
	}

	return lipgloss.NewStyle().
		MaxWidth(m.width).
		Height(m.footerHeight()).
		MaxHeight(m.footerHeight()).
		Render(line)
}

func (m *model) resize(w, h int) {
	m.width, m.height = w, h
	m.help.Width = w

	m.feedWidth = feedPanelWidth
	if w < minWidthForFeeds {
		m.feedWidth = 0
		m.focus = focusPosts
	}

	postsWidth := w - m.feedWidth
	if m.feedWidth > 0 {
		postsWidth--
	}

	panelHeight := max(h-m.footerHeight(), 1)
	m.feedList.SetSize(m.feedWidth, panelHeight)
	m.list.SetSize(max(postsWidth, 1), panelHeight)
	m.prompt.Width = max(w-len(m.prompt.Prompt)-1, 1)

	m.viewport.Width = w
	m.viewport.Height = max(h-detailChromeHeight-m.footerHeight()+1, 1)
	if m.showDetail {
		m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
	}
	m.applyFocus()
}

func (m *model) applyFocus() {
	m.list.Styles.Title = panelTitleStyle(m.focus == focusPosts)
	m.feedList.Styles.Title = panelTitleStyle(m.focus == focusFeeds)
}
