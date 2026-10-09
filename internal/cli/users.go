package cli

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/auth"
	"github.com/MSamoilovic/gator-cli/internal/database"

	"github.com/google/uuid"
	"golang.org/x/term"
)

func handlerLogin(ctx context.Context, s *state, cmd command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("username required to log in")
	}

	dbUser, err := s.Db.GetUser(ctx, cmd.Args[0])
	if err != nil {
		return fmt.Errorf("user doesn't exist: %v", err)
	}

	err = s.Cfg.SetUsername(dbUser.Name)
	if err != nil {
		return fmt.Errorf("can't log in: %v", err)
	}

	fmt.Printf("User %s logged on\n", s.Cfg.CurrentUserName)
	printNextStep(ctx, s, dbUser)

	return nil
}

func printNextStep(ctx context.Context, s *state, user database.User) {
	follows, err := s.Db.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return
	}

	if len(follows) == 0 {
		fmt.Println("No feeds yet — pick some with: gator discover")
		return
	}
	fmt.Printf("Following %d feeds — open the reader with: gator tui\n", len(follows))
}

func handlerUsers(ctx context.Context, s *state, _ command) error {
	users, err := s.Db.GetUsers(ctx)
	if err != nil {
		return fmt.Errorf("error fetching users: %v", err)
	}

	for _, u := range users {
		if u.Name == s.Cfg.CurrentUserName {
			fmt.Printf("* %s (current)\n", u.Name)
		} else {
			fmt.Printf("* %s\n", u.Name)
		}
	}
	return nil
}

func handlerRegister(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	emailFlag := fs.String("email", "", "email for a future server login (optional)")
	passwordFlag := fs.String("password", "", "password for a future server login (optional)")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: register [--email=...] [--password=...] <username>")
	}
	username := fs.Arg(0)

	email, password := *emailFlag, *passwordFlag
	if isTerminal(os.Stdin) {
		email = resolveEmail(email, os.Stdin)
		password = resolvePassword(password)
	}

	passwordHash := ""
	if password != "" {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("can't register: %w", err)
		}
		passwordHash = hash
	}

	params := database.CreateUserWithCredentialsParams{
		ID:           uuid.New(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Name:         username,
		Email:        sql.NullString{String: email, Valid: email != ""},
		PasswordHash: passwordHash,
	}
	dbUser, err := s.Db.CreateUserWithCredentials(ctx, params)
	if err != nil {
		return fmt.Errorf("error creating user: %v", err)
	}

	if err := s.Cfg.SetUsername(dbUser.Name); err != nil {
		return fmt.Errorf("error setting username: %v", err)
	}
	fmt.Printf("User %s created\n", s.Cfg.CurrentUserName)
	printNextStep(ctx, s, dbUser)

	return nil
}

func resolveEmail(given string, in io.Reader) string {
	if given = strings.TrimSpace(given); given != "" {
		return given
	}

	fmt.Print("Email (optional, enter to skip): ")
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(line)
}

func resolvePassword(given string) string {
	if given != "" {
		return given
	}

	fmt.Print("Password (optional, enter to skip): ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return string(pw)
}
