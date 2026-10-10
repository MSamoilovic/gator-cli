package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/credentials"
	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"golang.org/x/term"
)

func normalizeServerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%q is not a server address (try https://gator.example.com)", raw)
	}

	switch u.Scheme {
	case "https":
	case "http":
		if !loopbackHost(u.Hostname()) {
			return "", fmt.Errorf("refusing to send a password over plain http to %s; use https", u.Host)
		}
	default:
		return "", fmt.Errorf("server address must start with https:// (got %q)", raw)
	}

	return strings.TrimRight(u.String(), "/"), nil
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serverPassword(given string) (string, error) {
	if given != "" {
		return given, nil
	}
	if !isTerminal(os.Stdin) {
		return "", errors.New("a password is required (pass --password, or run from a terminal)")
	}

	fmt.Print("Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	if len(pw) == 0 {
		return "", errors.New("a password is required")
	}
	return string(pw), nil
}

func loginRemote(ctx context.Context, server, login, password string) error {
	server, err := normalizeServerURL(server)
	if err != nil {
		return err
	}
	password, err = serverPassword(password)
	if err != nil {
		return err
	}

	res, err := remote.Login(ctx, server, login, password)
	if err != nil {
		return fmt.Errorf("can't log in: %w", err)
	}
	return saveSession(server, res)
}

func registerRemote(ctx context.Context, server, name, email, password string) error {
	server, err := normalizeServerURL(server)
	if err != nil {
		return err
	}
	if email == "" {
		return errors.New("an email is required to register on a server (--email)")
	}
	password, err = serverPassword(password)
	if err != nil {
		return err
	}

	res, err := remote.Register(ctx, server, name, email, password)
	if err != nil {
		return fmt.Errorf("can't register: %w", err)
	}
	return saveSession(server, res)
}

func saveSession(server string, res wire.TokenResponse) error {
	if err := credentials.Save(credentials.Credentials{ServerURL: server, Token: res.Token}); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	cfg, err := config.Read()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading config: %w", err)
	}
	cfg.ServerURL = server
	cfg.CurrentUserName = res.User.Name
	if err := cfg.Write(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Logged in to %s as %s\n", server, res.User.Name)
	fmt.Println("Open the reader with: gator tui")
	return nil
}

func handlerLogout(ctx context.Context, _ *state, _ command) error {
	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if cfg.ServerURL == "" {
		return errors.New("not connected to a server")
	}

	if token, err := credentials.Load(cfg.ServerURL); err == nil {
		if err := remote.New(cfg.ServerURL, token).Logout(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: the server did not confirm the logout: %v\n", err)
		}
	}
	if err := credentials.Delete(); err != nil {
		return fmt.Errorf("removing saved token: %w", err)
	}

	server := cfg.ServerURL
	cfg.ServerURL = ""
	if err := cfg.Write(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Disconnected from %s\n", server)
	return nil
}
