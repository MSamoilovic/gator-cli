package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestOnlyPublicAddressesAreAllowed(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"127.0.0.1", false},
		{"127.0.0.53", false},
		{"::1", false},
		{"169.254.169.254", false},
		{"fe80::1", false},
		{"10.0.0.5", false},
		{"172.16.0.1", false},
		{"172.31.255.255", false},
		{"192.168.1.5", false},
		{"fc00::1", false},
		{"fd12:3456::1", false},
		{"0.0.0.0", false},
		{"::", false},
		{"224.0.0.1", false},
		{"ff02::1", false},
		{"255.255.255.255", false},
		{"100.64.0.1", false},
		{"198.18.0.1", false},
		{"192.0.0.1", false},
		{"240.0.0.1", false},
		{"64:ff9b::a00:5", false},
		{"2002:0a00:0005::", false},
		{"2001:db8::1", false},
	}

	for _, tc := range cases {
		a, err := netip.ParseAddr(tc.addr)
		if err != nil {
			t.Fatalf("parsing %q: %v", tc.addr, err)
		}
		if got := public(a); got != tc.want {
			t.Errorf("public(%s) = %v, want %v", tc.addr, got, tc.want)
		}
	}

	for _, mapped := range []string{"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:100.64.0.1", "::ffff:169.254.169.254"} {
		if public(netip.MustParseAddr(mapped)) {
			t.Errorf("public(%s) = true — an IPv4-mapped address reaches the same socket, and "+
				"netip.Prefix.Contains compares address families, so the reserved list alone misses it", mapped)
		}
	}
	if public(netip.Addr{}) {
		t.Error("public(invalid) = true")
	}
}

func TestAnUnguardedClientStillReachesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "hi")
	}))
	defer srv.Close()

	body, err := get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("plain fetch of %s: %v — the CLI fetches feeds the user named themselves", srv.URL, err)
	}
	if body != "hi" {
		t.Errorf("body = %q", body)
	}
}

func TestAGuardedClientRefusesLoopback(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		io.WriteString(w, "secret")
	}))
	defer srv.Close()

	_, err := get(PublicOnly(context.Background()), srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("guarded fetch of %s = %v, want ErrBlocked", srv.URL, err)
	}
	if hits != 0 {
		t.Errorf("the server was reached %d times — the address must be refused before the connection", hits)
	}
}

func TestTheGuardRunsOnEveryRedirectHop(t *testing.T) {
	var hits int
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		io.WriteString(w, "credentials")
	}))
	defer internal.Close()

	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL+"/latest/meta-data", http.StatusFound)
	}))
	defer outside.Close()

	ctx := withPolicy(context.Background(), blockPort(t, internal.URL))
	if _, err := get(ctx, outside.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("following the redirect = %v, want ErrBlocked — a public URL that redirects "+
			"inward is the whole attack", err)
	}
	if hits != 0 {
		t.Errorf("the redirect target was reached %d times", hits)
	}
}

func TestAGuardedRequestDoesNotReuseAnUnguardedConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "hi")
	}))
	defer srv.Close()

	if _, err := get(context.Background(), srv.URL); err != nil {
		t.Fatalf("warming the connection pool: %v", err)
	}
	if _, err := get(PublicOnly(context.Background()), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("guarded fetch after an unguarded one = %v, want ErrBlocked — a pooled "+
			"connection skips the dial, and the dial is where the guard lives", err)
	}
}

func TestAGuardedRequestIgnoresTheProxy(t *testing.T) {
	if got, want := funcOf(shared.guarded.Proxy), funcOf(noProxy); got != want {
		t.Error("the guarded transport takes its proxy from the environment — a proxy resolves the " +
			"address itself, so the guard would only ever see the proxy's own address")
	}

	proxy, err := shared.guarded.Proxy(&http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}})
	if err != nil || proxy != nil {
		t.Errorf("proxy for a guarded request = %v, %v; want none", proxy, err)
	}
}

func TestOneGuardedConnectionIsNotReusedUnderAnotherPolicy(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		io.WriteString(w, "hi")
	}))
	defer srv.Close()

	allowAll := withPolicy(context.Background(), func(netip.AddrPort) error { return nil })
	if _, err := get(allowAll, srv.URL); err != nil {
		t.Fatalf("fetch under a permissive policy: %v", err)
	}
	if hits != 1 {
		t.Fatalf("the server was reached %d times, want 1", hits)
	}

	if _, err := get(PublicOnly(context.Background()), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("fetch under the real policy = %v, want ErrBlocked — a pooled connection skips "+
			"the dial, and the dial is the only place a policy is applied", err)
	}
	if hits != 1 {
		t.Errorf("the server was reached %d times", hits)
	}
}

func funcOf(f func(*http.Request) (*url.URL, error)) uintptr {
	return reflect.ValueOf(f).Pointer()
}

func get(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	res, err := Client(5 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	return string(body), err
}

func blockPort(t *testing.T, rawURL string) policy {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return func(ap netip.AddrPort) error {
		if fmt.Sprint(ap.Port()) == u.Port() {
			return fmt.Errorf("%w: %s", ErrBlocked, ap)
		}
		return nil
	}
}

func TestTheControlHookChecksTheAddressItIsGiven(t *testing.T) {
	check := control(publicAddress)

	for _, address := range []string{"127.0.0.1:80", "[::ffff:127.0.0.1]:80", "[::1]:443", "169.254.169.254:80", "garbage"} {
		if err := check("tcp", address, nil); !errors.Is(err, ErrBlocked) {
			t.Errorf("control(%q) = %v, want ErrBlocked", address, err)
		}
	}
	if err := check("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("control(8.8.8.8:443) = %v, want it allowed", err)
	}
}

func TestTheGuardedDialerFailsClosedWithoutAPolicy(t *testing.T) {
	if _, err := dialGuarded(context.Background(), "tcp", "8.8.8.8:443"); !errors.Is(err, ErrBlocked) {
		t.Errorf("dialing with no policy in the context = %v, want ErrBlocked", err)
	}
}

func TestThePolicyIsGivenAnUnmappedAddress(t *testing.T) {
	var got netip.AddrPort
	check := control(func(ap netip.AddrPort) error {
		got = ap
		return nil
	})
	if err := check("tcp", "[::ffff:100.64.0.1]:80", nil); err != nil {
		t.Fatal(err)
	}

	if want := netip.MustParseAddrPort("100.64.0.1:80"); got != want {
		t.Errorf("the policy saw %s, want %s — netip.Prefix.Contains compares address families, "+
			"so a policy matching prefixes needs the unmapped form", got, want)
	}
}
