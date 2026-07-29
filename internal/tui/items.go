package tui

import (
	"gator-cli/internal/database"
)

type postItem struct {
	post database.Post
}

func (i postItem) Title() string { return i.post.Title }

func (i postItem) Description() string {
	if i.post.PublishedAt.Valid {
		return i.post.PublishedAt.Time.Format("2006-01-02") + " · " + i.post.Url
	}
	return i.post.Url
}

func (i postItem) FilterValue() string { return i.post.Title }
