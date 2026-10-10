package api

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

const (
	minPasswordLength = 8
	maxNameLength     = 20
)

var decoyHash = func() string {
	hash, err := auth.HashPassword("decoy-password-never-matches")
	if err != nil {
		panic(err)
	}
	return hash
}()

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req wire.RegisterRequest
	if !decode(w, r, &req) {
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	switch {
	case name == "" || utf8.RuneCountInString(name) > maxNameLength:
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "name must be 1-20 characters")
		return
	case !validEmail(email):
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "a valid email is required")
		return
	case utf8.RuneCountInString(req.Password) < minPasswordLength:
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, err.Error())
		return
	}

	now := time.Now()
	user, err := s.q.CreateUserWithCredentials(r.Context(), database.CreateUserWithCredentialsParams{
		ID:           uuid.New(),
		CreatedAt:    now,
		UpdatedAt:    now,
		Name:         name,
		Email:        sql.NullString{String: email, Valid: true},
		PasswordHash: hash,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			writeError(w, http.StatusConflict, wire.CodeConflict, "that name or email is already taken")
			return
		}
		s.fail(w, r, err)
		return
	}

	s.respondWithToken(w, r, user, http.StatusCreated)
}

func (s *Server) issueToken(w http.ResponseWriter, r *http.Request) {
	var req wire.TokenRequest
	if !decode(w, r, &req) {
		return
	}

	login := strings.TrimSpace(req.Login)
	var (
		user database.User
		err  error
	)
	if strings.Contains(login, "@") {
		user, err = s.q.GetUserByEmail(r.Context(), login)
	} else {
		user, err = s.q.GetUser(r.Context(), login)
	}

	hash := user.PasswordHash
	if err != nil || hash == "" {
		hash = decoyHash
	}
	verified := auth.VerifyPassword(hash, req.Password)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.fail(w, r, err)
		return
	}
	if err != nil || user.PasswordHash == "" || !verified {
		writeError(w, http.StatusUnauthorized, wire.CodeUnauthorized, "wrong name, email or password")
		return
	}

	s.respondWithToken(w, r, user, http.StatusOK)
}

func (s *Server) respondWithToken(w http.ResponseWriter, r *http.Request, user database.User, status int) {
	token, err := auth.GenerateToken()
	if err != nil {
		s.fail(w, r, err)
		return
	}

	if _, err := s.q.CreateAPIToken(r.Context(), database.CreateAPITokenParams{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: auth.HashToken(token),
		CreatedAt: time.Now(),
	}); err != nil {
		s.fail(w, r, err)
		return
	}

	writeJSON(w, status, wire.TokenResponse{Token: token, User: wire.FromUser(user)})
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request, _ store.Store) {
	token, _ := r.Context().Value(tokenKey{}).(string)
	if _, err := s.q.DeleteAPIToken(r.Context(), auth.HashToken(token)); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, st store.Store) {
	user, err := st.Me(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.FromUser(user))
}

func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s
}
