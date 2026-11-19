package api

import (
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/MSamoilovic/gator-cli/internal/wire"

	"golang.org/x/time/rate"
)

type limit struct {
	every time.Duration
	burst int
}

var (
	anonymousLimit = limit{every: 6 * time.Second, burst: 10}
	loginLimit     = limit{every: 3 * time.Minute, burst: 10}
	costlyLimit    = limit{every: time.Second, burst: 60}
	userLimit      = limit{every: 50 * time.Millisecond, burst: 300}
)

const (
	minIdle    = 10 * time.Minute
	sweepEvery = time.Minute
)

type limiter struct {
	limit limit
	idle  time.Duration
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	swept   time.Time
}

type bucket struct {
	tokens *rate.Limiter
	seen   time.Time
}

func newLimiter(l limit, now func() time.Time) *limiter {
	return &limiter{
		limit:   l,
		idle:    max(minIdle, l.every*time.Duration(l.burst)),
		now:     now,
		buckets: make(map[string]*bucket),
	}
}

func (l *limiter) bucket(key string, now time.Time) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.swept) >= sweepEvery {
		for k, b := range l.buckets {
			if now.Sub(b.seen) >= l.idle {
				delete(l.buckets, k)
			}
		}
		l.swept = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: rate.NewLimiter(rate.Every(l.limit.every), l.limit.burst)}
		l.buckets[key] = b
	}
	b.seen = now
	return b.tokens
}

func (l *limiter) take(key string) time.Duration {
	now := l.now()
	r := l.bucket(key, now).ReserveN(now, 1)
	if !r.OK() {
		return l.limit.every
	}
	if wait := r.DelayFrom(now); wait > 0 {
		r.CancelAt(now)
		return wait
	}
	return 0
}

func (l *limiter) wait(key string) time.Duration {
	now := l.now()
	tokens := l.bucket(key, now).TokensAt(now)
	if tokens >= 1 {
		return 0
	}
	return time.Duration((1 - tokens) * float64(l.limit.every))
}

func (l *limiter) spend(key string) {
	now := l.now()
	l.bucket(key, now).AllowN(now, 1)
}

func tooMany(w http.ResponseWriter, wait time.Duration) {
	seconds := max(1, int(math.Ceil(wait.Seconds())))
	w.Header().Set("Retry-After", fmt.Sprint(seconds))
	writeError(w, http.StatusTooManyRequests, wire.CodeRateLimited, fmt.Sprintf("too many requests; try again in %ds", seconds))
}

func (s *Server) perIP(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wait := s.anonymous.take(s.clientIP(r)); wait > 0 {
			tooMany(w, wait)
			return
		}
		next(w, r)
	})
}

func (s *Server) clientIP(r *http.Request) string {
	remote, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	addr := remote.Addr().Unmap()
	if !s.trusted(addr) {
		return addr.String()
	}

	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		hop = hop.Unmap()
		if !s.trusted(hop) {
			return hop.String()
		}
	}
	return addr.String()
}

func (s *Server) trusted(addr netip.Addr) bool {
	for _, p := range s.trustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
