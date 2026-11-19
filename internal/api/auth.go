package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/invite"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLength = 8
	maxNameLength     = 20
	tokenLifetimeDays = 90
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

	token := strings.TrimSpace(req.Invite)
	if token == "" {
		writeError(w, http.StatusForbidden, wire.CodeUnauthorized, "this server needs an invite link: gator register --link=<link> <name>")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	switch {
	case !validName(name):
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "name must be 1-20 letters, digits, dots, dashes or underscores")
		return
	case !validEmail(email):
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "a valid email is required")
		return
	case utf8.RuneCountInString(req.Password) < minPasswordLength:
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	switch {
	case errors.Is(err, bcrypt.ErrPasswordTooLong):
		writeError(w, http.StatusBadRequest, wire.CodeBadRequest, "password must be at most 72 bytes")
		return
	case err != nil:
		s.fail(w, r, err)
		return
	}

	now := time.Now()
	tokenHash := auth.HashToken(token)
	claimed, err := s.q.ClaimInvite(r.Context(), database.ClaimInviteParams{Now: now, TokenHash: tokenHash})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusForbidden, wire.CodeUnauthorized, s.inviteRefusal(r.Context(), tokenHash, now))
		return
	case err != nil:
		s.fail(w, r, err)
		return
	}

	user, err := s.q.CreateUserWithCredentials(r.Context(), database.CreateUserWithCredentialsParams{
		ID:           uuid.New(),
		CreatedAt:    now,
		UpdatedAt:    now,
		Name:         name,
		Email:        sql.NullString{String: email, Valid: true},
		PasswordHash: hash,
	})
	if err != nil {
		s.releaseInvite(r.Context(), claimed.ID)

		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			writeError(w, http.StatusConflict, wire.CodeConflict, "that name or email is already taken")
			return
		}
		s.fail(w, r, err)
		return
	}

	if _, err := s.q.SetInviteUser(context.WithoutCancel(r.Context()), database.SetInviteUserParams{
		ID:     claimed.ID,
		UserID: uuid.NullUUID{UUID: user.ID, Valid: true},
	}); err != nil {
		s.log.Printf("recording who used invite %s: %v", claimed.ID, err)
	}

	s.respondWithToken(w, r, user, http.StatusCreated)
}

func (s *Server) releaseInvite(ctx context.Context, id uuid.UUID) {
	if _, err := s.q.ReleaseInvite(context.WithoutCancel(ctx), id); err != nil {
		s.log.Printf("releasing invite %s: %v", id, err)
	}
}

func (s *Server) inviteRefusal(ctx context.Context, tokenHash string, now time.Time) string {
	inv, err := s.q.GetInvite(ctx, database.GetInviteParams{Now: now, TokenHash: tokenHash})
	switch {
	case err != nil:
		return "that invite link is not valid"
	case inv.ClaimedAt.Valid:
		return "that invite link has already been used"
	default:
		return "that invite link expired on " + inv.ExpiresAt.Format(invite.TimeFormat)
	}
}

func (s *Server) issueToken(w http.ResponseWriter, r *http.Request) {
	var req wire.TokenRequest
	if !decode(w, r, &req) {
		return
	}

	login := strings.TrimSpace(req.Login)
	attempts := "login:" + strings.ToLower(login)
	if wait := s.login.wait(attempts); wait > 0 {
		tooMany(w, wait)
		return
	}

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
		s.login.spend(attempts)
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
		ID:           uuid.New(),
		UserID:       user.ID,
		TokenHash:    auth.HashToken(token),
		CreatedAt:    time.Now(),
		LifetimeDays: tokenLifetimeDays,
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

func (s *Server) revokeAllTokens(w http.ResponseWriter, r *http.Request, st store.Store) {
	user, err := st.Me(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	n, err := s.q.DeleteAPITokensForUser(r.Context(), user.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.RevokeResponse{Revoked: n})
}

func (s *Server) touchToken(ctx context.Context, tokenHash string) {
	if _, err := s.q.TouchAPIToken(ctx, database.TouchAPITokenParams{
		LifetimeDays: tokenLifetimeDays,
		TokenHash:    tokenHash,
	}); err != nil {
		s.log.Printf("extending a token: %v", err)
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, st store.Store) {
	user, err := st.Me(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, wire.FromUser(user))
}

func validName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("._-", r) {
			return false
		}
	}
	return true
}

func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s
}
