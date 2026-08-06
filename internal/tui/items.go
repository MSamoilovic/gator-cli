package tui

import (
	"strconv"

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

type feedItem struct {
	id     uuid.UUID
	name   string
	unread map[uuid.UUID]int
}

func (i feedItem) Title() string {
	if n := i.unread[i.id]; n > 0 {
		return i.name + " (" + strconv.Itoa(n) + ")"
	}
	return i.name
}
func (i feedItem) Description() string { return "" }
func (i feedItem) FilterValue() string { return i.name }
