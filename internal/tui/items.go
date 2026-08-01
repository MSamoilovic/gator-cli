package tui

import (
	"gator-cli/internal/database"

	"github.com/google/uuid"
)

type postItem struct {
	post      database.Post
	bookmarks map[uuid.UUID]bool
}

func (i postItem) Title() string {
	if i.bookmarks[i.post.ID] {
		return "★ " + i.post.Title
	}
	return i.post.Title
}

func (i postItem) Description() string {
	if i.post.PublishedAt.Valid {
		return i.post.PublishedAt.Time.Format("2006-01-02") + " · " + i.post.Url
	}
	return i.post.Url
}

func (i postItem) FilterValue() string { return i.post.Title }
