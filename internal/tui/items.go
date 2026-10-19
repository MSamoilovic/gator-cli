package tui

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/catalog"
	"github.com/MSamoilovic/gator-cli/internal/database"

	"github.com/google/uuid"
)

const (
	unreadMark = "●"
	savedMark  = "★"
	undatedAge = "-"
	ageCols    = 3
)

type postItem struct {
	post      database.Post
	bookmarks map[uuid.UUID]bool
	reads     map[uuid.UUID]bool
	feedNames map[uuid.UUID]string
}

func (i postItem) marks() string {
	read, saved := " ", " "
	if !i.reads[i.post.ID] {
		read = unreadMark
	}
	if i.bookmarks[i.post.ID] {
		saved = savedMark
	}
	return read + saved
}

func (i postItem) Title() string {
	return i.marks() + " " + relativeAge(i.post.PublishedAt, time.Now()) + " " + i.post.Title
}

func (i postItem) Description() string { return i.feedNames[i.post.FeedID] }

func (i postItem) FilterValue() string { return i.Title() }

func relativeAge(published sql.NullTime, now time.Time) string {
	return fmt.Sprintf("%*s", ageCols, ageLabel(published, now))
}

func ageLabel(published sql.NullTime, now time.Time) string {
	if !published.Valid {
		return undatedAge
	}

	const (
		day  = 24 * time.Hour
		week = 7 * day
		year = 52 * week
	)

	switch age := now.Sub(published.Time); {
	case age < time.Hour:
		return strconv.Itoa(max(int(age/time.Minute), 0)) + "m"
	case age < day:
		return strconv.Itoa(int(age/time.Hour)) + "h"
	case age < week:
		return strconv.Itoa(int(age/day)) + "d"
	case age < year:
		return strconv.Itoa(int(age/week)) + "w"
	default:
		return strconv.Itoa(int(age/year)) + "y"
	}
}

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
	name     string
	number   int
	feedIDs  []uuid.UUID
	broken   int
	expanded map[string]bool
	unread   map[uuid.UUID]int
}

func (i folderItem) Title() string {
	chevron := folderClosed
	if i.expanded[i.name] {
		chevron = folderOpen
	}

	label := chevron + " " + i.name
	if i.number >= 1 && i.number <= maxFolderKeys {
		label = strconv.Itoa(i.number) + " " + label
	}

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
