package fetch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

var ErrBlocked = errors.New("address is not on the public internet")

const (
	dialTimeout = 10 * time.Second
	keepAlive   = 30 * time.Second
)

type policy func(netip.AddrPort) error

type policyKey struct{}

func PublicOnly(ctx context.Context) context.Context {
	return withPolicy(ctx, publicAddress)
}

func withPolicy(ctx context.Context, p policy) context.Context {
	return context.WithValue(ctx, policyKey{}, p)
}

func policyFor(ctx context.Context) policy {
	p, _ := ctx.Value(policyKey{}).(policy)
	return p
}

func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: shared}
}

var shared = newRouter()

type router struct {
	plain   *http.Transport
	guarded *http.Transport
}

func newRouter() *router {
	plain := http.DefaultTransport.(*http.Transport).Clone()
	plain.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}).DialContext

	guarded := http.DefaultTransport.(*http.Transport).Clone()
	guarded.DialContext = dialGuarded
	guarded.DisableKeepAlives = true
	guarded.Proxy = noProxy

	return &router{plain: plain, guarded: guarded}
}

func (r *router) RoundTrip(req *http.Request) (*http.Response, error) {
	if policyFor(req.Context()) != nil {
		return r.guarded.RoundTrip(req)
	}
	return r.plain.RoundTrip(req)
}

func noProxy(*http.Request) (*url.URL, error) { return nil, nil }

func dialGuarded(ctx context.Context, network, address string) (net.Conn, error) {
	p := policyFor(ctx)
	if p == nil {
		return nil, fmt.Errorf("%w: %s", ErrBlocked, address)
	}
	d := net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive, Control: control(p)}
	return d.DialContext(ctx, network, address)
}

func control(p policy) func(string, string, syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrBlocked, address)
		}
		return p(netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port()))
	}
}

func publicAddress(ap netip.AddrPort) error {
	if !public(ap.Addr()) {
		return fmt.Errorf("%w: %s", ErrBlocked, ap.Addr())
	}
	return nil
}

var reserved = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("100::/64"),
}

func public(a netip.Addr) bool {
	if a.Is4In6() || !a.IsGlobalUnicast() || a.IsPrivate() {
		return false
	}
	for _, p := range reserved {
		if p.Contains(a) {
			return false
		}
	}
	return true
}
