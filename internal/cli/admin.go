package cli

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"
	"github.com/MSamoilovic/gator-cli/internal/store"
	"github.com/MSamoilovic/gator-cli/internal/text"

	"github.com/google/uuid"
	"golang.org/x/term"
)

var errNoAdmin = errors.New("no admin yet — run: gator admin <you>")

var readAdminPassword = func(name string) (string, error) {
	if !isTerminal(os.Stdin) {
		return "", errors.New("admin commands ask for your password; run this from a terminal")
	}
	fmt.Fprintf(os.Stderr, "Password for %s: ", name)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return string(pw), nil
}

func guardAdmin(e entry, h handlerFunc) handlerFunc {
	return func(ctx context.Context, s *state, cmd command) error {
		caller, err := requireAdmin(ctx, s, e.name)
		switch {
		case errors.Is(err, errNoAdmin) && e.bootstraps:
			return h(ctx, s, cmd)
		case err != nil:
			return err
		}
		if err := logAdminAction(ctx, s.Db, caller, cmd); err != nil {
			return err
		}
		return h(ctx, s, cmd)
	}
}

func requireAdmin(ctx context.Context, s *state, name string) (store.User, error) {
	admins, err := s.Db.CountAdmins(ctx)
	if err != nil {
		return store.User{}, fmt.Errorf("checking for an admin: %w", err)
	}
	if admins == 0 {
		return store.User{}, errNoAdmin
	}

	user, err := s.Store.Me(ctx)
	if err != nil {
		return store.User{}, signInError(s, err)
	}
	if !user.IsAdmin {
		return store.User{}, fmt.Errorf("%s is for admins, and %s is not one", name, user.Name)
	}
	if err := confirmPassword(user); err != nil {
		return store.User{}, err
	}
	return user, nil
}

func confirmPassword(user store.User) error {
	if user.PasswordHash == "" {
		return fmt.Errorf("%s has no password, so it cannot confirm an admin command", user.Name)
	}
	pw, err := readAdminPassword(user.Name)
	if err != nil {
		return err
	}
	if !auth.VerifyPassword(user.PasswordHash, pw) {
		return errors.New("wrong password")
	}
	return nil
}

func logAdminAction(ctx context.Context, q *database.Queries, user store.User, cmd command) error {
	err := q.LogAdminAction(ctx, database.LogAdminActionParams{
		ID:       uuid.New(),
		At:       time.Now(),
		UserID:   uuid.NullUUID{UUID: user.ID, Valid: true},
		UserName: user.Name,
		Command:  cmd.Name,
		Args:     strings.Join(cmd.Args, " "),
	})
	if err != nil {
		return fmt.Errorf("recording the admin action: %w", err)
	}
	return nil
}

func handlerAdmin(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("admin", flag.ContinueOnError)
	revoke := fs.Bool("revoke", false, "take the admin role away from <name>")
	showLog := fs.Bool("log", false, "print the latest admin actions")
	limit := fs.Int("limit", 50, "how many actions --log prints")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}

	switch {
	case *showLog && fs.NArg() == 0 && !*revoke:
		return printAdminLog(ctx, s.Db, *limit)
	case *revoke && fs.NArg() == 1:
		return revokeAdmin(ctx, s.Db, fs.Arg(0))
	case !*revoke && !*showLog && fs.NArg() == 1:
		return grantAdmin(ctx, s.Db, fs.Arg(0))
	case !*revoke && !*showLog && fs.NArg() == 0:
		return printAdmins(ctx, s.Db)
	}
	return errors.New("usage: admin [<name> | --revoke <name> | --log [--limit N]]")
}

func grantAdmin(ctx context.Context, q *database.Queries, name string) error {
	target, err := q.GetUser(ctx, name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("no user named %s", name)
	case err != nil:
		return fmt.Errorf("looking up %s: %w", name, err)
	case target.IsAdmin:
		return fmt.Errorf("%s is already an admin", name)
	case target.PasswordHash == "":
		return fmt.Errorf("%s has no password, and an admin must confirm every admin command with one", name)
	}

	admins, err := q.CountAdmins(ctx)
	if err != nil {
		return fmt.Errorf("checking for an admin: %w", err)
	}
	bootstrap := admins == 0
	if bootstrap {
		if err := confirmPassword(target); err != nil {
			return err
		}
	}

	n, err := q.GrantAdmin(ctx, database.GrantAdminParams{
		UpdatedAt:  time.Now(),
		ID:         target.ID,
		OnlyIfNone: bootstrap,
	})
	if err != nil {
		return fmt.Errorf("making %s an admin: %w", name, err)
	}
	if n == 0 {
		return fmt.Errorf("%s was not made an admin: someone else became one first", name)
	}

	if bootstrap {
		if err := logAdminAction(ctx, q, target, command{Name: "admin", Args: []string{name}}); err != nil {
			return err
		}
	}
	fmt.Printf("%s is now an admin\n", name)
	return nil
}

func revokeAdmin(ctx context.Context, q *database.Queries, name string) error {
	target, err := q.GetUser(ctx, name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("no user named %s", name)
	case err != nil:
		return fmt.Errorf("looking up %s: %w", name, err)
	case !target.IsAdmin:
		return fmt.Errorf("%s is not an admin", name)
	}

	n, err := q.RevokeAdmin(ctx, database.RevokeAdminParams{UpdatedAt: time.Now(), ID: target.ID})
	if err != nil {
		return fmt.Errorf("revoking %s: %w", name, err)
	}
	if n == 0 {
		return fmt.Errorf("%s is the last admin; make someone else an admin first", name)
	}
	fmt.Printf("%s is no longer an admin\n", name)
	return nil
}

func printAdmins(ctx context.Context, q *database.Queries) error {
	admins, err := q.ListAdmins(ctx)
	if err != nil {
		return fmt.Errorf("listing admins: %w", err)
	}
	if len(admins) == 0 {
		return errNoAdmin
	}
	for _, a := range admins {
		fmt.Println(text.Printable(a.Name))
	}
	return nil
}

func printAdminLog(ctx context.Context, q *database.Queries, limit int) error {
	if limit < 1 {
		return errors.New("--limit must be at least 1")
	}
	rows, err := q.ListAdminLog(ctx, int32(min(limit, 10000)))
	if err != nil {
		return fmt.Errorf("reading the admin log: %w", err)
	}
	if len(rows) == 0 {
		fmt.Println("No admin actions recorded.")
		return nil
	}
	for _, r := range rows {
		line := r.At.Format("2006-01-02 15:04") + "  " + text.Printable(r.UserName) + "  " + r.Command
		if r.Args != "" {
			line += " " + text.Printable(r.Args)
		}
		fmt.Println(line)
	}
	return nil
}
