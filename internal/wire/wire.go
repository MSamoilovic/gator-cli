package wire

import (
	"database/sql"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/database"

	"github.com/google/uuid"
)

const (
	CodeFeedNotFound = "feed_not_found"
	CodeNotFollowed  = "not_followed"
	CodeNoMoreText   = "no_more_text"
	CodeNotFound     = "not_found"
	CodeUnauthorized = "unauthorized"
	CodeBadRequest   = "bad_request"
	CodeConflict     = "conflict"
	CodeInternal     = "internal"
)

type Error struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Post struct {
	ID          uuid.UUID  `json:"id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Description *string    `json:"description"`
	PublishedAt *time.Time `json:"published_at"`
	FeedID      uuid.UUID  `json:"feed_id"`
	FullText    string     `json:"full_text"`
}

type Feed struct {
	ID            uuid.UUID  `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	UserID        *uuid.UUID `json:"user_id"`
	LastFetchedAt *time.Time `json:"last_fetched_at"`
	LastError     string     `json:"last_error"`
	FailureCount  int32      `json:"failure_count"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type TokenResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Posts struct {
	Posts []Post `json:"posts"`
}

type IDs struct {
	IDs []uuid.UUID `json:"ids"`
}

type Created struct {
	Created bool `json:"created"`
}

type Text struct {
	Text string `json:"text"`
}

type AddFeedRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

type AddFeedResponse struct {
	Feed    Feed `json:"feed"`
	Created bool `json:"created"`
}

type FollowRequest struct {
	URL string `json:"url"`
}

type FollowResponse struct {
	Follow  database.CreateFeedFollowRow `json:"follow"`
	Created bool                         `json:"created"`
}

type CategorizeRequest struct {
	URL      string `json:"url"`
	Category string `json:"category"`
}

type FeedResponse struct {
	Feed Feed `json:"feed"`
}

type MarkReadRequest struct {
	PostIDs []uuid.UUID `json:"post_ids"`
}

type Subscriptions struct {
	Subscriptions []database.GetFeedFollowsForUserRow `json:"subscriptions"`
}

type Stats struct {
	Stats []database.GetFeedStatsForUserRow `json:"stats"`
}

type Unread struct {
	Unread []database.GetUnreadCountsForUserRow `json:"unread"`
}

func FromUser(u database.User) User {
	return User{ID: u.ID, Name: u.Name, Email: u.Email.String, CreatedAt: u.CreatedAt}
}

func (u User) ToDatabase() database.User {
	return database.User{ID: u.ID, Name: u.Name, Email: nullString(u.Email), CreatedAt: u.CreatedAt}
}

func FromPost(p database.Post) Post {
	out := Post{
		ID: p.ID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		Title: p.Title, URL: p.Url, FeedID: p.FeedID, FullText: p.FullText,
	}
	if p.Description.Valid {
		out.Description = &p.Description.String
	}
	if p.PublishedAt.Valid {
		out.PublishedAt = &p.PublishedAt.Time
	}
	return out
}

func (p Post) ToDatabase() database.Post {
	out := database.Post{
		ID: p.ID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		Title: p.Title, Url: p.URL, FeedID: p.FeedID, FullText: p.FullText,
	}
	if p.Description != nil {
		out.Description.String, out.Description.Valid = *p.Description, true
	}
	if p.PublishedAt != nil {
		out.PublishedAt.Time, out.PublishedAt.Valid = *p.PublishedAt, true
	}
	return out
}

func FromPosts(ps []database.Post) []Post {
	out := make([]Post, len(ps))
	for i, p := range ps {
		out[i] = FromPost(p)
	}
	return out
}

func ToPosts(ps []Post) []database.Post {
	if len(ps) == 0 {
		return nil
	}
	out := make([]database.Post, len(ps))
	for i, p := range ps {
		out[i] = p.ToDatabase()
	}
	return out
}

func FromFeed(f database.Feed) Feed {
	out := Feed{
		ID: f.ID, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
		Name: f.Name, URL: f.Url, LastError: f.LastError, FailureCount: f.FailureCount,
	}
	if f.UserID.Valid {
		out.UserID = &f.UserID.UUID
	}
	if f.LastFetchedAt.Valid {
		out.LastFetchedAt = &f.LastFetchedAt.Time
	}
	return out
}

func (f Feed) ToDatabase() database.Feed {
	out := database.Feed{
		ID: f.ID, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
		Name: f.Name, Url: f.URL, LastError: f.LastError, FailureCount: f.FailureCount,
	}
	if f.UserID != nil {
		out.UserID.UUID, out.UserID.Valid = *f.UserID, true
	}
	if f.LastFetchedAt != nil {
		out.LastFetchedAt.Time, out.LastFetchedAt.Valid = *f.LastFetchedAt, true
	}
	return out
}

func nullString(s string) (out sql.NullString) {
	return sql.NullString{String: s, Valid: s != ""}
}
