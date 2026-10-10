package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"
)

func (s *Server) subscriptions(w http.ResponseWriter, r *http.Request, st store.Store) {
	subs, err := st.Subscriptions(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if subs == nil {
		subs = []store.Subscription{}
	}
	writeJSON(w, http.StatusOK, wire.Subscriptions{Subscriptions: subs})
}

func (s *Server) addFeed(w http.ResponseWriter, r *http.Request, st store.Store) {
	var req wire.AddFeedRequest
	if !decode(w, r, &req) {
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "url is required")
		return
	}

	feed, created, err := st.AddFeed(r.Context(), req.Name, req.URL)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			s.fail(w, r, err)
			return
		}
		writeError(w, http.StatusUnprocessableEntity, wire.CodeBadRequest, err.Error())
		return
	}

	if req.Category != "" {
		if _, err := st.Categorize(r.Context(), feed.Url, req.Category); err != nil {
			s.fail(w, r, err)
			return
		}
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, wire.AddFeedResponse{Feed: wire.FromFeed(feed), Created: created})
}

func (s *Server) follow(w http.ResponseWriter, r *http.Request, st store.Store) {
	var req wire.FollowRequest
	if !decode(w, r, &req) {
		return
	}

	follow, created, err := st.FollowURL(r.Context(), req.URL)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, wire.FollowResponse{Follow: follow, Created: created})
}

func (s *Server) categorize(w http.ResponseWriter, r *http.Request, st store.Store) {
	var req wire.CategorizeRequest
	if !decode(w, r, &req) {
		return
	}

	feed, err := st.Categorize(r.Context(), req.URL, req.Category)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.FeedResponse{Feed: wire.FromFeed(feed)})
}

func (s *Server) unfollowURL(w http.ResponseWriter, r *http.Request, st store.Store) {
	feed, err := st.UnfollowURL(r.Context(), r.URL.Query().Get("url"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.FeedResponse{Feed: wire.FromFeed(feed)})
}

func (s *Server) unfollow(w http.ResponseWriter, r *http.Request, st store.Store) {
	id, ok := pathID(w, r, "feed_id")
	if !ok {
		return
	}
	if err := st.Unfollow(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request, st store.Store) {
	since := time.Now().AddDate(0, 0, -7)
	if raw := r.URL.Query().Get("since"); raw != "" {
		t, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "since must be an RFC 3339 timestamp")
			return
		}
		since = t.In(time.Local)
	}

	stats, err := st.Stats(r.Context(), since)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if stats == nil {
		stats = []store.FeedStat{}
	}
	writeJSON(w, http.StatusOK, wire.Stats{Stats: stats})
}

func (s *Server) unread(w http.ResponseWriter, r *http.Request, st store.Store) {
	counts, err := st.UnreadCounts(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if counts == nil {
		counts = []store.UnreadCount{}
	}
	writeJSON(w, http.StatusOK, wire.Unread{Unread: counts})
}
