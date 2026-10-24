package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/api"
)

const (
	shutdownGrace = 10 * time.Second
	inviteEnv     = "GATOR_INVITE_CODE"
)

func inviteCode(given string) string {
	if given = strings.TrimSpace(given); given != "" {
		return given
	}
	return strings.TrimSpace(os.Getenv(inviteEnv))
}

func handlerServe(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
	invite := fs.String("invite-code", "", "code new accounts must present; without it registration is closed")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: serve [--addr=host:port] [--invite-code=code]")
	}
	cfg := api.Config{InviteCode: inviteCode(*invite)}

	ctx, stop := signal.NotifyContext(ctx, shutdownSignals()...)
	defer stop()

	logger := log.New(os.Stderr, "serve: ", log.LstdFlags)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(s.Db, logger, cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          logger,
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	fmt.Printf("Serving the gator API on %s (Ctrl+C to stop)\n", *addr)
	if cfg.InviteCode == "" {
		fmt.Println("Registration is closed: no invite code is set (--invite-code, or GATOR_INVITE_CODE)")
	} else {
		fmt.Println("Registration needs an invite code")
	}

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down: %w", err)
	}
	fmt.Println("Stopped")
	return nil
}
