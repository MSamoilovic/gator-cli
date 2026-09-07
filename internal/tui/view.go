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
	case m.screen == screenCatalog:
		return m.catalogView()
	case m.screen == screenDetail:
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
		m.feedPanel(),
		verticalRule(m.panelHeight()),
		m.postsPanel(),
	)
	return panels + "\n" + m.footer()
}

func (m model) panelHeight() int {
	return max(m.height-m.footerHeight(), 1)
}

func (m model) postsGutter() int {
	if m.feedWidth == 0 {
		return 0
	}
	return panelGutter
}

func (m model) feedPanel() string {
	return panelBox(m.feedList.View(), m.feedWidth, m.panelHeight(), 0)
}

func (m model) postsPanel() string {
	body := m.list.View()
	if len(m.list.Items()) == 0 {
		body = lipgloss.JoinVertical(
			lipgloss.Left,
			m.list.Styles.TitleBar.Render(m.list.Styles.Title.Render(m.list.Title)),
			emptyStateStyle.Render(m.emptyStateText()),
		)
	}
	return panelBox(body, m.postsWidth, m.panelHeight(), m.postsGutter())
}

func panelBox(content string, width, height, padLeft int) string {
	return lipgloss.NewStyle().
		PaddingLeft(padLeft).
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height).
		Render(content)
}

func (m model) emptyStateText() string {
	switch {
	case m.showBookmarks:
		return "No bookmarks yet.\nPress b on a post to save it."
	case m.query != "":
		return "No posts match " + strconv.Quote(m.query) + ".\nPress esc to go back."
	case m.feedsLoaded && m.feedCount == 0:
		return "You are not following any feeds.\nPress tab, then c to pick from the catalog."
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
	case m.screen == screenCatalog:
		return m.keys.catalogHelp()
	case m.screen == screenDetail:
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
		line = m.userTagged(m.status)
	case m.help.ShowAll:
		line = m.help.FullHelpView(m.keys.fullHelp())
	default:
		line = m.userTagged(m.help.ShortHelpView(m.currentBindings()))
	}

	return lipgloss.NewStyle().
		MaxWidth(m.width).
		Height(m.footerHeight()).
		MaxHeight(m.footerHeight()).
		Render(line)
}

func (m model) userTagged(line string) string {
	if m.userName == "" {
		return line
	}

	tag := userTagStyle.Render("@" + m.userName)
	gap := m.width - lipgloss.Width(line) - lipgloss.Width(tag)
	if gap < 1 {
		return line
	}
	return line + strings.Repeat(" ", gap) + tag
}

func (m *model) resize(w, h int) {
	m.width, m.height = w, h
	m.help.Width = w

	m.feedWidth = feedPanelWidth
	if w < minWidthForFeeds {
		m.feedWidth = 0
		m.focus = focusPosts
	}

	m.postsWidth = w - m.feedWidth
	if m.feedWidth > 0 {
		m.postsWidth--
	}

	feedListWidth, postsListWidth := m.feedWidth, m.postsWidth
	if m.feedWidth > 0 {
		feedListWidth -= panelGutter
		postsListWidth -= panelGutter
	}

	panelHeight := m.panelHeight()
	m.feedList.SetSize(max(feedListWidth, 1), panelHeight)
	m.catalogList.SetSize(w, panelHeight)
	m.list.SetSize(max(postsListWidth, 1), panelHeight)
	m.prompt.Width = max(w-len(m.prompt.Prompt)-1, 1)

	m.viewport.Width = w
	m.viewport.Height = max(h-detailChromeHeight-m.footerHeight()+1, 1)
	if m.screen == screenDetail {
		m.viewport.SetContent(renderDetailBody(m.selected, m.viewport.Width))
	}
	m.applyFocus()
}

func (m *model) applyFocus() {
	m.list.Styles.Title = panelTitleStyle(m.focus == focusPosts)
	m.feedList.Styles.Title = panelTitleStyle(m.focus == focusFeeds)
}
