package cli

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
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

func (s *state) withDB() (*state, func() error, error) {
	if s.Db != nil {
		return s, func() error { return nil }, nil
	}

	opened, closeDB, err := open(s.Schema)
	if err != nil {
		return nil, nil, err
	}
	if opened.Db == nil {
		return nil, nil, fmt.Errorf("connected to %s, which has no local database (gator logout first)", opened.Cfg.ServerURL)
	}
	return opened, closeDB, nil
}

func handlerLogin(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	serverFlag := fs.String("server", "", "log in to a gator server instead of the local database")
	passwordFlag := fs.String("password", "", "password for the server (prompted when omitted; "+visiblePassword+")")
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("username required to log in")
	}

	if server := serverTarget(*serverFlag); server != "" {
		password, err := passwordInput(*passwordFlag, *fromStdin, os.Stdin)
		if err != nil {
			return err
		}
		return loginRemote(ctx, server, fs.Arg(0), password)
	}

	s, closeDB, err := s.withDB()
	if err != nil {
		return err
	}
	defer closeDB()

	dbUser, err := s.Db.GetUser(ctx, fs.Arg(0))
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

func handlerRegister(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	emailFlag := fs.String("email", "", "email for a future server login (optional)")
	passwordFlag := fs.String("password", "", "password (required; asked for when omitted; "+visiblePassword+")")
	fromStdin := fs.Bool("password-stdin", false, "read the password from standard input")
	serverFlag := fs.String("server", "", "register on a gator server instead of the local database (email and password required)")
	linkFlag := fs.String("link", "", "invite link the server was handed out with")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: register [--email=...] [--password=... | --password-stdin] [--link=...] <username>")
	}
	username := fs.Arg(0)

	email := *emailFlag
	password, err := passwordInput(*passwordFlag, *fromStdin, os.Stdin)
	if err != nil {
		return err
	}
	if !*fromStdin && isTerminal(os.Stdin) {
		email = resolveEmail(email, os.Stdin)
		password = resolvePassword(password)
	}
	if password == "" {
		return errors.New("a password is required (pass --password-stdin, or run from a terminal)")
	}

	server, token, err := registrationTarget(*serverFlag, *linkFlag)
	if err != nil {
		return err
	}
	if server != "" {
		return registerRemote(ctx, server, username, email, password, token)
	}

	s, closeDB, err := s.withDB()
	if err != nil {
		return err
	}
	defer closeDB()

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("can't register: %w", err)
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
	if interactive() {
		return runNext{cmd: command{Name: "discover"}, reopen: true}
	}
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

const (
	visiblePassword  = "visible in ps and shell history, prefer --password-stdin"
	maxPasswordInput = 1024
)

func passwordInput(given string, fromStdin bool, in io.Reader) (string, error) {
	if !fromStdin {
		return given, nil
	}
	if given != "" {
		return "", errors.New("pass --password or --password-stdin, not both")
	}

	data, err := io.ReadAll(io.LimitReader(in, maxPasswordInput+1))
	if err != nil {
		return "", fmt.Errorf("reading the password from standard input: %w", err)
	}
	if len(data) > maxPasswordInput {
		return "", fmt.Errorf("the password on standard input is over %d bytes", maxPasswordInput)
	}

	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" {
		return "", errors.New("no password on standard input")
	}
	return password, nil
}

func resolvePassword(given string) string {
	if given != "" {
		return given
	}

	fmt.Print("Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return string(pw)
}
