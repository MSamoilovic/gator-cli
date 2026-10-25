package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/invite"
)

func (s *Server) joinPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := s.q.GetInvite(r.Context(), database.GetInviteParams{
		Now:       time.Now(),
		TokenHash: auth.HashToken(token),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeText(w, http.StatusNotFound, "This invite link is not valid.\n")
	case err != nil:
		s.fail(w, r, err)
	case inv.ClaimedAt.Valid:
		writeText(w, http.StatusGone, "This invite link has already been used.\n")
	case inv.Expired:
		writeText(w, http.StatusGone, "This invite link expired on "+inv.ExpiresAt.Format(invite.TimeFormat)+".\n")
	default:
		writeText(w, http.StatusOK, joinInstructions(invite.Link(externalBase(r), token), inv.ExpiresAt))
	}
}

func joinInstructions(link string, expires time.Time) string {
	return fmt.Sprintf(`You have been invited to gator, an RSS reader that runs in your terminal.

Install it from https://github.com/MSamoilovic/gator-cli, then run:

    gator register --link=%s <your-name>

The link is good for one account and expires on %s.
`, link, expires.Format(invite.TimeFormat))
}

func externalBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ","); strings.TrimSpace(forwarded) == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
