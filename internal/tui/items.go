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

const brokenMark = "⚠"

type feedItem struct {
	id       uuid.UUID
	name     string
	url      string
	failures int32
	indent   bool
	unread   map[uuid.UUID]int
}

func (i feedItem) Title() string {
	name := i.name
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

type folderItem struct {
	name      string
	feedIDs   []uuid.UUID
	broken    int
	collapsed map[string]bool
	unread    map[uuid.UUID]int
}

func (i folderItem) Title() string {
	label := folderOpen
	if i.collapsed[i.name] {
		label = folderClosed
	}
	label += " " + i.name

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
