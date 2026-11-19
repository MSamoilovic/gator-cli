package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/MSamoilovic/gator-cli/internal/config"
	"github.com/MSamoilovic/gator-cli/internal/credentials"
	"github.com/MSamoilovic/gator-cli/internal/invite"
	"github.com/MSamoilovic/gator-cli/internal/store/offline"
	"github.com/MSamoilovic/gator-cli/internal/store/remote"
	"github.com/MSamoilovic/gator-cli/internal/text"
	"github.com/MSamoilovic/gator-cli/internal/wire"

	"golang.org/x/term"
)

var defaultServer string

func serverTarget(given string) string {
	if given != "" {
		return given
	}

	cfg, err := config.Read()
	switch {
	case err != nil:
		return defaultServer
	case cfg.ServerURL != "":
		return cfg.ServerURL
	case cfg.DBURL == "":
		return defaultServer
	}
	return ""
}

func normalizeServerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%q is not a server address (try https://gator.example.com)", raw)
	}

	switch u.Scheme {
	case "https":
	case "http":
		if !loopbackHost(u.Hostname()) {
			return "", fmt.Errorf("refusing to send a password or token over plain http to %s; use https", u.Host)
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

func registrationTarget(serverFlag, link string) (server, token string, err error) {
	if strings.TrimSpace(link) == "" {
		return serverTarget(serverFlag), "", nil
	}

	server, token, err = invite.Parse(link)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(serverFlag) == "" {
		return server, token, nil
	}

	given, err := normalizeServerURL(serverFlag)
	if err != nil {
		return "", "", err
	}
	if given != server {
		return "", "", fmt.Errorf("the invite link points at %s, but --server says %s", server, given)
	}
	return server, token, nil
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
	if err := saveSession(server, res); err != nil {
		return err
	}
	fmt.Println("Open the reader with: gator tui")
	return nil
}

func registerRemote(ctx context.Context, server, name, email, password, invite string) error {
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

	res, err := remote.Register(ctx, server, wire.RegisterRequest{
		Name:     name,
		Email:    email,
		Password: password,
		Invite:   invite,
	})
	if err != nil {
		return fmt.Errorf("can't register: %w", err)
	}
	if err := saveSession(server, res); err != nil {
		return err
	}
	if interactive() {
		return runNext{cmd: command{Name: "discover"}, reopen: true}
	}
	fmt.Println("Pick some feeds with: gator discover")
	return nil
}

func saveSession(server string, res wire.TokenResponse) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding the home directory: %w", err)
	}
	lost, err := offline.ClearUnlessOwnedBy(home, server, res.User.Name)
	if err != nil {
		return fmt.Errorf("clearing the previous account's offline data: %w", err)
	}
	if lost > 0 {
		fmt.Fprintf(os.Stderr, "Discarded %s made offline by the previous account\n", plural(int64(lost), "change"))
	}

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

	fmt.Printf("Logged in to %s as %s\n", server, text.Printable(res.User.Name))
	return nil
}

func handlerLogout(ctx context.Context, _ *state, cmd command) error {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	everywhere := fs.Bool("all", false, "also sign out every other device on the server")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("usage: logout [--all]")
	}

	cfg, err := config.Read()
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if cfg.ServerURL == "" {
		if *everywhere {
			return errors.New("--all signs out the devices connected to a server, and this install is local")
		}
		if cfg.CurrentUserName == "" {
			return errors.New("not logged in")
		}
		name := cfg.CurrentUserName
		cfg.CurrentUserName = ""
		if err := cfg.Write(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Logged out %s\n", name)
		return nil
	}

	revoked := int64(-1)
	if *everywhere {
		if revoked, err = logoutEverywhere(ctx, cfg.ServerURL); err != nil {
			return err
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding the home directory: %w", err)
	}
	lost, err := offline.Clear(home)
	if err != nil {
		return fmt.Errorf("clearing offline data: %w", err)
	}
	if lost > 0 {
		fmt.Fprintf(os.Stderr, "Discarded %s that never reached %s\n", plural(int64(lost), "change"), cfg.ServerURL)
	}

	if !*everywhere {
		revokeThisDevice(ctx, cfg.ServerURL)
	}
	if err := credentials.Delete(); err != nil {
		return fmt.Errorf("removing saved token: %w", err)
	}

	server := cfg.ServerURL
	cfg.ServerURL = ""
	cfg.CurrentUserName = ""
	if err := cfg.Write(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	if revoked >= 0 {
		fmt.Printf("Signed out %s on %s\n", plural(revoked, "session"), server)
	}
	fmt.Printf("Disconnected from %s\n", server)
	return nil
}

func revokeThisDevice(ctx context.Context, server string) {
	if _, err := normalizeServerURL(server); err != nil {
		fmt.Fprintf(os.Stderr, "warning: not revoking the token on the server: %v\n", err)
		return
	}
	token, err := credentials.Load(server)
	if err != nil {
		return
	}
	if err := remote.New(server, token).Logout(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: the server did not confirm the logout: %v\n", err)
	}
}

func logoutEverywhere(ctx context.Context, server string) (int64, error) {
	if _, err := normalizeServerURL(server); err != nil {
		return 0, fmt.Errorf("not signing out everywhere: %w", err)
	}
	token, err := credentials.Load(server)
	switch {
	case errors.Is(err, credentials.ErrNoToken):
		return 0, fmt.Errorf("no saved sign-in for %s, so there is nothing to sign out with — run: gator login <name>, then gator logout --all", server)
	case err != nil:
		return 0, err
	}
	n, err := remote.New(server, token).LogoutAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s did not sign out the other devices, so nothing was changed here: %w", server, err)
	}
	return n, nil
}
