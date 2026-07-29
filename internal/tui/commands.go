package tui

import (
	"context"

	"gator-cli/internal/database"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type (
	postsLoadedMsg struct{ posts []database.Post }
	errMsg         struct{ err error }
)

func (e errMsg) Error() string { return e.err.Error() }

const postsLimit = 100

func loadPosts(ctx context.Context, q *database.Queries, userID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		posts, err := q.GetPostsForUserFiltered(ctx, database.GetPostsForUserFilteredParams{
			UserID:    userID,
			SortDir:   "desc",
			PostLimit: postsLimit,
		})
		if err != nil {
			return errMsg{err}
		}
		return postsLoadedMsg{posts: posts}
	}
}
