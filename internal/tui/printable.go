package tui

import (
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/text"
)

func printablePosts(posts []database.Post) []database.Post {
	out := make([]database.Post, len(posts))
	for i, p := range posts {
		p.Title = text.Printable(p.Title)
		p.Url = text.Printable(p.Url)
		p.Description.String = text.Printable(p.Description.String)
		p.FullText = text.Printable(p.FullText)
		out[i] = p
	}
	return out
}

func printableSubscriptions(subs []database.GetFeedFollowsForUserRow) []database.GetFeedFollowsForUserRow {
	out := make([]database.GetFeedFollowsForUserRow, len(subs))
	for i, s := range subs {
		s.FeedName = text.Printable(s.FeedName)
		s.FeedUrl = text.Printable(s.FeedUrl)
		s.Category = text.Printable(s.Category)
		s.UserName = text.Printable(s.UserName)
		out[i] = s
	}
	return out
}
