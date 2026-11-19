package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/api"
)

const (
	shutdownGrace  = 10 * time.Second
	maxHeaderBytes = 64 << 10
)

func handlerServe(ctx context.Context, s *state, cmd command) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
	proxies := fs.String("trusted-proxy", "", "comma-separated CIDRs of reverse proxies whose X-Forwarded-For to believe")
	public := fs.String("public-url", "", "public address of this server, used in invite pages (else "+serverURLEnv+")")
	if err := fs.Parse(cmd.Args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: serve [--addr=host:port] [--trusted-proxy=cidr,...] [--public-url=url]")
	}

	trusted, err := parsePrefixes(*proxies)
	if err != nil {
		return err
	}
	publicURL, err := servePublicURL(*public)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, shutdownSignals()...)
	defer stop()

	logger := log.New(os.Stderr, "serve: ", log.LstdFlags)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(s.Db, logger, api.Options{TrustedProxies: trusted, PublicURL: publicURL}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          logger,
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	fmt.Printf("Serving the gator API on %s (Ctrl+C to stop)\n", *addr)
	fmt.Println("Registration needs a link from: gator invite")

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

func parsePrefixes(raw string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			addr, addrErr := netip.ParseAddr(part)
			if addrErr != nil {
				return nil, fmt.Errorf("--trusted-proxy %q is not a CIDR such as 172.30.0.0/24", part)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func servePublicURL(given string) (string, error) {
	for _, candidate := range []string{given, os.Getenv(serverURLEnv), defaultServer} {
		if strings.TrimSpace(candidate) != "" {
			return normalizeServerURL(candidate)
		}
	}
	return "", nil
}
