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
	"time"

	"github.com/MSamoilovic/gator-cli/internal/api"
)

const shutdownGrace = 10 * time.Second

func handlerServe(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: serve [--addr=host:port]")
	}

	ctx, stop := signal.NotifyContext(ctx, shutdownSignals()...)
	defer stop()

	logger := log.New(os.Stderr, "serve: ", log.LstdFlags)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(s.Db, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          logger,
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	fmt.Printf("Serving the gator API on %s (Ctrl+C to stop)\n", *addr)

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
