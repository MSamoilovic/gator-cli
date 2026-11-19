package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/fetch"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/store/local"
	"github.com/MSamoilovic/gator-cli/internal/wire"
)

const (
	maxRequestBody = 1 << 20
	blockedMessage = "that address is not on the public internet"
)

type Options struct {
	TrustedProxies []netip.Prefix
	PublicURL      string
}

type Server struct {
	q   *database.Queries
	log *log.Logger

	trustedProxies []netip.Prefix
	publicURL      string

	anonymous *limiter
	login     *limiter
	costly    *limiter
	user      *limiter

	allowPrivateFetch bool
}

type tokenKey struct{}

func New(q *database.Queries, logger *log.Logger, opts Options) http.Handler {
	return newServer(&Server{q: q, log: logger}, opts)
}

func newServer(s *Server, opts Options) http.Handler {
	s.trustedProxies = opts.TrustedProxies
	s.publicURL = strings.TrimRight(opts.PublicURL, "/")
	s.anonymous = newLimiter(anonymousLimit, time.Now)
	s.login = newLimiter(loginLimit, time.Now)
	s.costly = newLimiter(costlyLimit, time.Now)
	s.user = newLimiter(userLimit, time.Now)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/healthz", s.healthz)
	mux.Handle("GET /join/{token}", s.perIP(s.joinPage))
	mux.Handle("POST /v1/users", s.perIP(s.register))
	mux.Handle("POST /v1/tokens", s.perIP(s.issueToken))
	mux.Handle("DELETE /v1/tokens/current", s.authed(s.revokeToken))
	mux.Handle("DELETE /v1/tokens", s.authed(s.revokeAllTokens))
	mux.Handle("GET /v1/me", s.authed(s.me))

	mux.Handle("GET /v1/posts", s.authed(s.posts))
	mux.Handle("GET /v1/posts/bookmarked", s.authed(s.bookmarkedPosts))
	mux.Handle("GET /v1/posts/bookmarked/ids", s.authed(s.bookmarkedIDs))
	mux.Handle("GET /v1/posts/read/ids", s.authed(s.readIDs))
	mux.Handle("POST /v1/posts/read", s.authed(s.markAllRead))
	mux.Handle("GET /v1/posts/by-url", s.authed(s.postByURL))
	mux.Handle("POST /v1/posts/{id}/read", s.authed(s.markRead))
	mux.Handle("DELETE /v1/posts/{id}/read", s.authed(s.markUnread))
	mux.Handle("POST /v1/posts/{id}/bookmark", s.authed(s.bookmark))
	mux.Handle("DELETE /v1/posts/{id}/bookmark", s.authed(s.unbookmark))
	mux.Handle("GET /v1/posts/{id}/fulltext", s.authed(s.fullText, s.costly))

	mux.Handle("GET /v1/subscriptions", s.authed(s.subscriptions))
	mux.Handle("POST /v1/subscriptions", s.authed(s.addFeed, s.costly))
	mux.Handle("POST /v1/subscriptions/follow", s.authed(s.follow, s.costly))
	mux.Handle("PATCH /v1/subscriptions", s.authed(s.categorize))
	mux.Handle("DELETE /v1/subscriptions", s.authed(s.unfollowURL))
	mux.Handle("DELETE /v1/subscriptions/{feed_id}", s.authed(s.unfollow))
	mux.Handle("GET /v1/subscriptions/stats", s.authed(s.stats))
	mux.Handle("GET /v1/unread", s.authed(s.unread))

	return s.recoverer(headers(s.publicFetchOnly(mux)))
}

func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) publicFetchOnly(next http.Handler) http.Handler {
	if s.allowPrivateFetch {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(fetch.PublicOnly(r.Context())))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, v)
				writeError(w, http.StatusInternalServerError, wire.CodeInternal, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authed(next func(http.ResponseWriter, *http.Request, store.Store), extra ...*limiter) http.Handler {
	limiters := append([]*limiter{s.user}, extra...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		token, ok := bearer(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, wire.CodeUnauthorized, "missing bearer token")
			return
		}

		tokenHash := auth.HashToken(token)
		user, err := s.q.GetUserByTokenHash(r.Context(), tokenHash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, wire.CodeUnauthorized, "invalid token")
			return
		}
		s.touchToken(r.Context(), tokenHash)

		for _, l := range limiters {
			if wait := l.take(user.ID.String()); wait > 0 {
				tooMany(w, wait)
				return
			}
		}

		ctx := context.WithValue(r.Context(), tokenKey{}, token)
		next(w, r.WithContext(ctx), local.NewFor(s.q, user))
	})
}

func bearer(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, wire.Error{Error: msg, Code: code})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrFeedNotFound):
		writeError(w, http.StatusNotFound, wire.CodeFeedNotFound, err.Error())
	case errors.Is(err, store.ErrNotFollowed):
		writeError(w, http.StatusConflict, wire.CodeNotFollowed, err.Error())
	case errors.Is(err, fetch.ErrBlocked):
		s.log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		writeError(w, http.StatusUnprocessableEntity, wire.CodeBadRequest, blockedMessage)
	case errors.Is(err, store.ErrNoMoreText):
		writeError(w, http.StatusUnprocessableEntity, wire.CodeNoMoreText, err.Error())
	case errors.Is(err, context.Canceled):
		writeError(w, 499, wire.CodeInternal, "request cancelled")
	default:
		s.log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		writeError(w, http.StatusInternalServerError, wire.CodeInternal, "internal error")
	}
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
