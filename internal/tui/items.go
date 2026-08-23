package tui

import (
	"fmt"
	"strconv"

	"gator-cli/internal/catalog"
	"gator-cli/internal/database"

	"github.com/google/uuid"
)

type postItem struct {
	post      database.Post
	bookmarks map[uuid.UUID]bool
	reads     map[uuid.UUID]bool
}

// Markeri se dele preko mapa koje model mutira, pa se stavke ne moraju
// ponovo praviti kad se stanje promeni.
func (i postItem) Title() string {
	prefix := ""
	if !i.reads[i.post.ID] {
		prefix += "●"
	}
	if i.bookmarks[i.post.ID] {
		prefix += "★"
	}
	if prefix == "" {
		return i.post.Title
	}
	return prefix + " " + i.post.Title
}

func (i postItem) Description() string {
	if i.post.PublishedAt.Valid {
		return i.post.PublishedAt.Time.Format("2006-01-02") + " · " + i.post.Url
	}
	return i.post.Url
}

func (i postItem) FilterValue() string { return i.post.Title }

// brokenMark stoji uz feed koji ne uspeva da se povuce, kao sto ● stoji uz
// nepracitan post.
const brokenMark = "⚠"

type feedItem struct {
	id       uuid.UUID
	name     string
	url      string
	failures int32
	indent   bool // stoji u folderu, pa je uvucen ispod zaglavlja
	unread   map[uuid.UUID]int
}

func (i feedItem) Title() string {
	name := i.name
	// Feed koji ne uspeva da se povuce inace tiho prestane da radi i to niko
	// ne primeti mesecima.
	if i.failures > 0 {
		name = brokenMark + " " + name
	}
	if n := i.unread[i.id]; n > 0 {
		name += " (" + strconv.Itoa(n) + ")"
	}
	if i.indent {
		name = feedIndent + name
	}
	return name
}
func (i feedItem) Description() string { return "" }
func (i feedItem) FilterValue() string { return i.name }

const (
	feedIndent   = "  "
	folderOpen   = "▾"
	folderClosed = "▸"
)

// folderItem je zaglavlje jedne grupe. bubbles/list nema zaglavlja i sve
// stavke su mu iste visine, pa je jedini nacin da grupa postoji taj da i ona
// bude obican red u listi. Posto je red kao i svaki drugi, moze i da se
// selektuje — i time je ⏎ nad njom prirodno mesto za sklapanje.
type folderItem struct {
	name    string
	feedIDs []uuid.UUID
	broken  int
	// collapsed se, kao i unread, deli po referenci sa modelom: menja se u
	// mestu, nikad se ne dodeljuje nova mapa.
	collapsed map[string]bool
	unread    map[uuid.UUID]int
}

func (i folderItem) Title() string {
	label := folderOpen
	if i.collapsed[i.name] {
		label = folderClosed
	}
	label += " " + i.name

	// Pokvaren feed u sklopljenom folderu bi inace bio nevidljiv — bas ono
	// protiv cega brokenMark i postoji.
	if i.broken > 0 {
		label += " " + brokenMark
	}
	if n := i.unreadTotal(); n > 0 {
		label += " (" + strconv.Itoa(n) + ")"
	}
	return label
}

func (i folderItem) unreadTotal() int {
	n := 0
	for _, id := range i.feedIDs {
		n += i.unread[id]
	}
	return n
}

func (i folderItem) Description() string { return "" }
func (i folderItem) FilterValue() string { return i.name }

// catalogItem je jedna kategorija u biracu. picked se, kao i markeri kod
// postova, deli po referenci sa modelom — space menja mapu, ne stavku.
type catalogItem struct {
	cat      catalog.Category
	picked   map[string]bool
	followed int
}

func (i catalogItem) Title() string {
	box := "[ ]"
	if i.picked[i.cat.ID] {
		box = "[x]"
	}

	counts := strconv.Itoa(len(i.cat.Feeds)) + " feeds"
	if i.followed > 0 {
		counts += ", " + strconv.Itoa(i.followed) + " followed"
	}
	return fmt.Sprintf("%s %-22s %s", box, i.cat.Label, counts)
}

func (i catalogItem) Description() string { return "" }
func (i catalogItem) FilterValue() string { return i.cat.Label }
