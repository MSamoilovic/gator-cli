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
		return name + " (" + strconv.Itoa(n) + ")"
	}
	return name
}
func (i feedItem) Description() string { return "" }
func (i feedItem) FilterValue() string { return i.name }

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
