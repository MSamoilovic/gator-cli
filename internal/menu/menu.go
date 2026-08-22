package menu

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Item struct {
	Name    string // "addfeed"
	Args    string // "[name] <url>"; prazno kad komanda ne uzima argumente
	Summary string
	Group   string
}

// Config je sve sto birac treba da nacrta: pozdrav i ponuda.
type Config struct {
	Title    string // traka na vrhu, npr. "gator"
	Greeting string // red ispod nje, npr. "Hello, Tsunami"
	Items    []Item
}

// Choice je izabrana komanda, spremna za dispatch.
type Choice struct {
	Name string
	Args []string
}

// needsArgs je tacno kad bar jedan argument nije opcion. Dogovor je isti kao u
// usage porukama: obavezno ide u <>, opciono u [].
func (i Item) needsArgs() bool { return strings.Contains(i.Args, "<") }

func (i Item) Title() string {
	if i.Args == "" {
		return i.Name
	}
	return i.Name + " " + i.Args
}

func (i Item) Description() string { return i.Summary }

// FilterValue hvata i grupu i opis, pa "feed" nadje i `import` i `following`.
func (i Item) FilterValue() string { return i.Name + " " + i.Group + " " + i.Summary }

const (
	listHint = "↑↓ move · ⏎ run · / filter · q quit"
	askHint  = "⏎ run · esc back"
	// footerHeight je koliko redova ispod liste drzimo za prompt i poruku.
	footerHeight = 4
)

var (
	muted         = lipgloss.AdaptiveColor{Light: "#5C5C5C", Dark: "#9B9B9B"}
	accent        = lipgloss.AdaptiveColor{Light: "#5A3FD6", Dark: "#7D56F4"}
	hintStyle     = lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
	greetingStyle = lipgloss.NewStyle().Foreground(accent).Bold(true).Padding(0, 2)
)

type model struct {
	list     list.Model
	prompt   textinput.Model
	keys     keyMap
	greeting string

	asking  bool // ceka se unos argumenata
	pending Item
	status  string

	choice Choice
	chosen bool
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

func newModel(cfg Config) model {
	items := make([]list.Item, len(cfg.Items))
	for i, it := range cfg.Items {
		items[i] = it
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = cfg.Title
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	return model{
		list:     l,
		prompt:   textinput.New(),
		keys:     newKeyMap(),
		greeting: cfg.Greeting,
		status:   listHint,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-footerHeight-m.greetingHeight())
		m.prompt.Width = max(msg.Width-len(m.prompt.Prompt)-1, 1)
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

func (m model) greetingHeight() int {
	if m.greeting == "" {
		return 0
	}
	return 2 // pozdrav i prazan red ispod njega
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dok se filtrira, slova pripadaju filteru — inace bi "q" u "queue"
	// zatvorilo program.
	if m.list.FilterState() != list.Filtering {
		switch {
		case msg.Type == tea.KeyCtrlC, key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

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

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

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

// ask prebacuje birac na unos argumenata za item. Fokus mora da se postavi nad
// kopijom koja se vraca — textinput ignorise tastere dok nije fokusiran.
func (m model) ask(item Item) (model, tea.Cmd) {
	m.asking = true
	m.pending = item
	m.prompt.Prompt = "gator " + item.Name + " "
	m.prompt.Placeholder = item.Args
	m.prompt.SetValue("")
	m.prompt.Width = max(m.list.Width()-len(m.prompt.Prompt)-1, 1)
	m.status = askHint
	return m, m.prompt.Focus()
}

func (m model) cancel() model {
	m.asking = false
	m.prompt.Blur()
	m.status = listHint
	return m
}

func (m model) choose(item Item, args []string) (tea.Model, tea.Cmd) {
	m.choice = Choice{Name: item.Name, Args: args}
	m.chosen = true
	return m, tea.Quit
}

func (m model) View() string {
	var b strings.Builder
	if m.greeting != "" {
		b.WriteString(greetingStyle.Render(m.greeting) + "\n\n")
	}
	b.WriteString(m.list.View() + "\n")
	if m.asking {
		b.WriteString(m.prompt.View() + "\n")
	}
	b.WriteString(hintStyle.Render(m.status))
	return b.String()
}

// Select otvori birac i vrati izabranu komandu. ok je netacno kad je korisnik
// izasao bez izbora — to nije greska.
func Select(cfg Config) (Choice, bool, error) {
	final, err := tea.NewProgram(newModel(cfg), tea.WithAltScreen()).Run()
	if err != nil {
		return Choice{}, false, err
	}

	m, ok := final.(model)
	if !ok || !m.chosen {
		return Choice{}, false, nil
	}
	return m.choice, true, nil
}
