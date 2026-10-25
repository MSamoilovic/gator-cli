package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/invite"

	"github.com/google/uuid"
)

const (
	defaultInviteTTL = 72 * time.Hour
	serverURLEnv     = "GATOR_SERVER_URL"
)

func handlerInvite(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("invite", flag.ContinueOnError)
	ttl := fs.Duration("ttl", defaultInviteTTL, "how long the link stays valid")
	note := fs.String("note", "", "who the link is for")
	serverFlag := fs.String("server", "", "public address of this server (else "+serverURLEnv+")")
	list := fs.Bool("list", false, "list the links handed out so far")
	revoke := fs.String("revoke", "", "delete an unused link by id")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: invite [--ttl=72h] [--note=who] [--server=url] | --list | --revoke=<id>")
	}

	switch {
	case *list && *revoke != "":
		return errors.New("--list and --revoke do different things; pick one")
	case *list:
		return listInvites(ctx, s)
	case *revoke != "":
		return revokeInvite(ctx, s, *revoke)
	case *ttl <= 0:
		return errors.New("--ttl must be positive")
	}

	server, err := inviteServer(*serverFlag)
	if err != nil {
		return err
	}

	token, err := auth.GenerateInviteToken()
	if err != nil {
		return err
	}

	now := time.Now()
	row, err := s.Db.CreateInvite(ctx, database.CreateInviteParams{
		ID:        uuid.New(),
		TokenHash: auth.HashToken(token),
		Note:      strings.TrimSpace(*note),
		CreatedAt: now,
		ExpiresAt: now.Add(*ttl),
	})
	if err != nil {
		return fmt.Errorf("creating the invite: %w", err)
	}

	fmt.Println(invite.Link(server, token))
	fmt.Printf("\nGood for one account until %s", row.ExpiresAt.Format(invite.TimeFormat))
	if row.Note != "" {
		fmt.Printf(" — for %s", row.Note)
	}
	fmt.Println(". It is not stored, so copy it now.")
	return nil
}

func inviteServer(given string) (string, error) {
	for _, candidate := range []string{given, os.Getenv(serverURLEnv), defaultServer} {
		if strings.TrimSpace(candidate) != "" {
			return normalizeServerURL(candidate)
		}
	}
	return "", fmt.Errorf("the link needs the public address of this server: gator invite --server=https://gator.example.com (or set %s)", serverURLEnv)
}

func listInvites(ctx context.Context, s *state) error {
	rows, err := s.Db.ListInvites(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("listing invites: %w", err)
	}
	if len(rows) == 0 {
		return errors.New("no invites yet — create one with: gator invite")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tEXPIRES\tFOR\tUSED BY\tID")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			inviteStatus(row), row.ExpiresAt.Format(invite.TimeFormat),
			dash(row.Note), dash(row.UserName), row.ID)
	}
	return w.Flush()
}

func inviteStatus(row database.ListInvitesRow) string {
	switch {
	case row.ClaimedAt.Valid:
		return "used"
	case row.Expired:
		return "expired"
	default:
		return "active"
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func revokeInvite(ctx context.Context, s *state, given string) error {
	id, err := uuid.Parse(strings.TrimSpace(given))
	if err != nil {
		return fmt.Errorf("%q is not an invite id (gator invite --list)", given)
	}

	deleted, err := s.Db.DeleteInvite(ctx, id)
	if err != nil {
		return fmt.Errorf("revoking the invite: %w", err)
	}
	if deleted == 0 {
		return fmt.Errorf("no unused invite with id %s (gator invite --list)", id)
	}
	fmt.Printf("Revoked %s\n", id)
	return nil
}
