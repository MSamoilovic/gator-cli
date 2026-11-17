package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

type clock struct{ now time.Time }

func (c *clock) read() time.Time { return c.now }

func TestABucketRefusesPastItsBurstAndSaysHowLongToWait(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	l := newLimiter(limit{every: 6 * time.Second, burst: 10}, c.read)

	for i := range 10 {
		if wait := l.take("1.2.3.4"); wait != 0 {
			t.Fatalf("request %d refused with wait %v, want the burst of 10 to pass", i+1, wait)
		}
	}
	if wait := l.take("1.2.3.4"); wait != 6*time.Second {
		t.Errorf("11th request wait = %v, want 6s, the time to earn one token", wait)
	}
	if wait := l.take("5.6.7.8"); wait != 0 {
		t.Errorf("another key waited %v; keys must not share a bucket", wait)
	}

	c.now = c.now.Add(6 * time.Second)
	if wait := l.take("1.2.3.4"); wait != 0 {
		t.Errorf("after 6s the request still waited %v", wait)
	}
}

func TestARefusedRequestDoesNotDrainTheBucketFurther(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	l := newLimiter(limit{every: time.Second, burst: 1}, c.read)

	l.take("k")
	for range 50 {
		l.take("k")
	}
	c.now = c.now.Add(time.Second)
	if wait := l.take("k"); wait != 0 {
		t.Errorf("50 refused requests pushed the next allowed one back by %v", wait)
	}
}

func TestAnIdleBucketIsForgottenOnlyOnceItWouldHaveRefilled(t *testing.T) {
	c := &clock{now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	l := newLimiter(loginLimit, c.read)

	for range loginLimit.burst {
		l.spend("login:marko")
	}
	c.now = c.now.Add(minIdle + time.Second)
	l.take("someone-else")
	if _, kept := l.buckets["login:marko"]; !kept {
		t.Fatal("ten minutes of silence deleted a bucket that refills over thirty; a guesser would get a fresh burst every ten")
	}
	if tokens := l.buckets["login:marko"].tokens.TokensAt(c.now); tokens >= float64(loginLimit.burst) {
		t.Errorf("bucket holds %.1f tokens after ten minutes, want fewer than the full %d", tokens, loginLimit.burst)
	}

	c.now = c.now.Add(time.Duration(loginLimit.burst) * loginLimit.every)
	l.take("someone-else")
	if _, kept := l.buckets["login:marko"]; kept {
		t.Error("a bucket idle longer than its refill time was never swept")
	}
}

func TestTheClientAddressIgnoresForwardedForUnlessTheProxyIsTrusted(t *testing.T) {
	tests := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		xff     []string
		want    string
	}{
		{"no proxy trusted", nil, "203.0.113.9:5000", []string{"1.1.1.1"}, "203.0.113.9"},
		{"trusted proxy names the client", []netip.Prefix{netip.MustParsePrefix("172.30.0.0/24")}, "172.30.0.2:5000", []string{"1.1.1.1"}, "1.1.1.1"},
		{"a forged hop on the left is skipped", []netip.Prefix{netip.MustParsePrefix("172.30.0.0/24")}, "172.30.0.2:5000", []string{"9.9.9.9, 1.1.1.1"}, "1.1.1.1"},
		{"the proxy's own hops are skipped", []netip.Prefix{netip.MustParsePrefix("172.30.0.0/24")}, "172.30.0.2:5000", []string{"1.1.1.1, 172.30.0.3"}, "1.1.1.1"},
		{"split across headers", []netip.Prefix{netip.MustParsePrefix("172.30.0.0/24")}, "172.30.0.2:5000", []string{"9.9.9.9", "1.1.1.1"}, "1.1.1.1"},
		{"garbage stops the walk", []netip.Prefix{netip.MustParsePrefix("172.30.0.0/24")}, "172.30.0.2:5000", []string{"1.1.1.1, nonsense"}, "172.30.0.2"},
		{"untrusted sender with a trusted list", []netip.Prefix{netip.MustParsePrefix("172.30.0.0/24")}, "203.0.113.9:5000", []string{"1.1.1.1"}, "203.0.113.9"},
		{"mapped v4 is the same address", nil, "[::ffff:203.0.113.9]:5000", nil, "203.0.113.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{trustedProxies: tt.trusted}
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := s.clientIP(r); got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
