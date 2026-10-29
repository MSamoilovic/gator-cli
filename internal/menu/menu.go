package menu

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Item struct {
	Name    string
	Args    string
	Summary string
	Group   string
}

type Config struct {
	Title    string
	Greeting string
	Hint     string
	Items    []Item
}

type Choice struct {
	Name string
	Args []string
}

const nameCols = 12

func (i Item) needsArgs() bool { return strings.Contains(i.Args, "<") }

func (i Item) Title() string {
	if i.Summary == "" {
		return i.Name
	}
	return fmt.Sprintf("%-*s %s", nameCols, i.Name, i.Summary)
}

func (i Item) Description() string { return "" }

func (i Item) FilterValue() string { return i.Title() }

type header string

func (h header) Title() string       { return string(h) }
func (h header) Description() string { return "" }
func (h header) FilterValue() string { return "" }

func rows(items []Item) []list.Item {
	out := make([]list.Item, 0, len(items))
	group := ""
	for _, it := range items {
		if it.Group != group {
			group = it.Group
			out = append(out, header(group))
		}
		out = append(out, it)
	}
	return out
}

const (
	listHint     = "↑↓ move · ⏎ run · / filter · q quit"
	askHint      = "⏎ run · esc back"
	footerHeight = 4
)

var (
	muted         = lipgloss.AdaptiveColor{Light: "#5C5C5C", Dark: "#9B9B9B"}
	accent        = lipgloss.AdaptiveColor{Light: "#5A3FD6", Dark: "#7D56F4"}
	hintStyle     = lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
	greetingStyle = lipgloss.NewStyle().Foreground(accent).Bold(true).Padding(0, 2)
	headerStyle   = lipgloss.NewStyle().Foreground(muted).Bold(true).Padding(0, 0, 0, 2)
)

type delegate struct {
	list.DefaultDelegate
}

func newDelegate() delegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	return delegate{DefaultDelegate: d}
}

func (d delegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if h, ok := item.(header); ok {
		fmt.Fprint(w, headerStyle.Render(string(h)))
		return
	}
	d.DefaultDelegate.Render(w, m, index, item)
}

type Model struct {
	list     list.Model
	prompt   textinput.Model
	keys     keyMap
	greeting string
	hint     string

	width   int
	asking  bool
	pending Item
	status  string

	choice    Choice
	chosen    bool
	cancelled bool
}

type keyMap struct {
	Select key.Binding
	Quit   key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Select: key.NewBinding(key.WithKeys("enter")),
		Quit:   key.NewBinding(key.WithKeys("q", "esc")),
	}
}

func New(cfg Config) Model {
	l := list.New(rows(cfg.Items), newDelegate(), 0, 0)
	l.Title = cfg.Title
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	hint := cfg.Hint
	if hint == "" {
		hint = listHint
	}

	m := Model{
		list:     l,
		prompt:   textinput.New(),
		keys:     newKeyMap(),
		greeting: cfg.Greeting,
		hint:     hint,
		status:   hint,
	}
	return m.onItem(false)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if m.asking {
			return m.updatePrompt(msg)
		}
		return m.updateList(msg)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.list.SetSize(width, max(height-footerHeight-m.greetingHeight(), 1))
	m.prompt.Width = max(width-len(m.prompt.Prompt)-1, 1)
}

func (m Model) Chosen() (Choice, bool) { return m.choice, m.chosen }

func (m Model) Cancelled() bool { return m.cancelled }

func (m Model) Done() bool { return m.chosen || m.cancelled }

func (m Model) greetingHeight() int {
	if m.greeting == "" {
		return 0
	}
	return 2
}

func (m Model) onItem(up bool) Model {
	for range m.list.VisibleItems() {
		if _, ok := m.list.SelectedItem().(header); !ok {
			return m
		}
		before := m.list.Index()
		if up {
			m.list.CursorUp()
		} else {
			m.list.CursorDown()
		}
		if m.list.Index() == before {
			up = !up
		}
	}
	return m
}

func (m Model) updateList(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.list.FilterState() != list.Filtering {
		switch {
		case msg.Type == tea.KeyCtrlC, key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, nil

		case key.Matches(msg, m.keys.Select):
			item, ok := m.list.SelectedItem().(Item)
			if !ok {
				return m, nil
			}
			if item.Args == "" {
				return m.choose(item, nil)
			}
			return m.ask(item)
		}
	}

	up := key.Matches(msg, m.list.KeyMap.CursorUp)

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m.onItem(up), cmd
}

func (m Model) updatePrompt(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.cancelled = true
		return m, nil

	case tea.KeyEsc:
		return m.cancel(), nil

	case tea.KeyEnter:
		value := strings.TrimSpace(m.prompt.Value())
		if value == "" && m.pending.needsArgs() {
			m.status = m.pending.Name + " needs " + m.pending.Args + " — esc to go back"
			return m, nil
		}
		return m.choose(m.pending, SplitArgs(value))
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m Model) ask(item Item) (Model, tea.Cmd) {
	m.asking = true
	m.pending = item
	m.prompt.Prompt = "gator " + item.Name + " "
	m.prompt.Placeholder = item.Args
	m.prompt.SetValue("")
	m.prompt.Width = max(m.list.Width()-len(m.prompt.Prompt)-1, 1)
	m.status = askHint
	return m, m.prompt.Focus()
}

func (m Model) cancel() Model {
	m.asking = false
	m.prompt.Blur()
	m.status = m.hint
	return m
}

func (m Model) choose(item Item, args []string) (Model, tea.Cmd) {
	m.choice = Choice{Name: item.Name, Args: args}
	m.chosen = true
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	if m.greeting != "" {
		b.WriteString(m.bounded(greetingStyle.Render(m.greeting)) + "\n\n")
	}
	b.WriteString(m.list.View() + "\n")
	if m.asking {
		b.WriteString(m.bounded(m.prompt.View()) + "\n")
	}
	b.WriteString(m.bounded(hintStyle.Render(m.status)))
	return b.String()
}

func (m Model) bounded(line string) string {
	if m.width <= 0 {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
}

type program struct {
	Model
}

func (p program) Init() tea.Cmd { return nil }

func (p program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := p.Model.Update(msg)
	p.Model = next
	if next.Done() {
		return p, tea.Quit
	}
	return p, cmd
}

func Select(cfg Config) (Choice, bool, error) {
	final, err := tea.NewProgram(program{Model: New(cfg)}, tea.WithAltScreen()).Run()
	if err != nil {
		return Choice{}, false, err
	}

	p, ok := final.(program)
	if !ok {
		return Choice{}, false, nil
	}
	choice, chosen := p.Chosen()
	return choice, chosen, nil
}
