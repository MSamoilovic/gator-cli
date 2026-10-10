package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
)

const maxPostLimit = 1000

func (s *Server) posts(w http.ResponseWriter, r *http.Request, st store.Store) {
	q, err := postQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, err.Error())
		return
	}

	posts, err := st.Posts(r.Context(), q)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.Posts{Posts: wire.FromPosts(posts)})
}

func postQuery(r *http.Request) (store.PostQuery, error) {
	v := r.URL.Query()
	q := store.PostQuery{
		FeedName: v.Get("feed_name"),
		Query:    v.Get("q"),
		SortDir:  v.Get("sort"),
	}

	if raw := v.Get("feed_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return q, errors.New("feed_id is not a uuid")
		}
		q.FeedID = id
	}
	if raw := v.Get("unread"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return q, errors.New("unread must be true or false")
		}
		q.UnreadOnly = b
	}
	if raw := v.Get("since"); raw != "" {
		t, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return q, errors.New("since must be an RFC 3339 timestamp")
		}
		q.Since = t.In(time.Local)
	}
	if q.SortDir != "" && q.SortDir != "asc" && q.SortDir != "desc" {
		return q, errors.New("sort must be asc or desc")
	}

	limit, err := intParam(v.Get("limit"), "limit")
	if err != nil {
		return q, err
	}
	offset, err := intParam(v.Get("offset"), "offset")
	if err != nil {
		return q, err
	}
	q.Limit, q.Offset = min(limit, maxPostLimit), offset
	return q, nil
}

func intParam(raw, name string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < 0 {
		return 0, errors.New(name + " must be a non-negative integer")
	}
	return int32(n), nil
}

func (s *Server) bookmarkedPosts(w http.ResponseWriter, r *http.Request, st store.Store) {
	posts, err := st.BookmarkedPosts(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.Posts{Posts: wire.FromPosts(posts)})
}

func (s *Server) readPosts(w http.ResponseWriter, r *http.Request, st store.Store) {
	posts, err := st.ReadPosts(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.Posts{Posts: wire.FromPosts(posts)})
}

func (s *Server) bookmarkedIDs(w http.ResponseWriter, r *http.Request, st store.Store) {
	ids, err := st.BookmarkedIDs(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.IDs{IDs: nonNil(ids)})
}

func (s *Server) readIDs(w http.ResponseWriter, r *http.Request, st store.Store) {
	ids, err := st.ReadIDs(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.IDs{IDs: nonNil(ids)})
}

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

func (s *Server) postByURL(w http.ResponseWriter, r *http.Request, st store.Store) {
	post, err := st.PostByURL(r.Context(), r.URL.Query().Get("url"))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, wire.CodeNotFound, "post not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.FromPost(post))
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, name+" is not a uuid")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) markRead(w http.ResponseWriter, r *http.Request, st store.Store) {
	s.setRead(w, r, st, true)
}

func (s *Server) markUnread(w http.ResponseWriter, r *http.Request, st store.Store) {
	s.setRead(w, r, st, false)
}

func (s *Server) setRead(w http.ResponseWriter, r *http.Request, st store.Store, read bool) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := st.SetRead(r.Context(), id, read); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) markAllRead(w http.ResponseWriter, r *http.Request, st store.Store) {
	var req wire.MarkReadRequest
	if !decode(w, r, &req) {
		return
	}
	if err := st.SetAllRead(r.Context(), req.PostIDs); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) bookmark(w http.ResponseWriter, r *http.Request, st store.Store) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	created, err := st.Bookmark(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, wire.Created{Created: created})
}

func (s *Server) unbookmark(w http.ResponseWriter, r *http.Request, st store.Store) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := st.Unbookmark(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) fullText(w http.ResponseWriter, r *http.Request, st store.Store) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	post, err := s.q.GetPostByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, wire.CodeNotFound, "post not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}

	text, err := st.FullText(r.Context(), post)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.Text{Text: text})
}
