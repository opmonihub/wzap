package authsession

import (
	"net"
	"net/http"
	"sync"
	"time"

	"wzap/internal/httpapi/core"
)

// DefaultLoginLimit caps password guesses per client IP per minute on the
// session login route. It bounds brute-force without throttling the manager
// UI: humans retry far below this, while a guess loop burns a bcrypt
// comparison per attempt.
const DefaultLoginLimit = 10

// MaxLoginBuckets caps the number of active IP budgets, so
// a distributed scan cannot grow memory without bound (single replica by
// design, same as the Chatwoot webhook limiter).
const MaxLoginBuckets = 1024

// LoginRateLimiter is a per-IP fixed window for POST /auth/login: it answers
// whether a password guess from ip may proceed, responding 429 rate_limited
// on exhaustion. A nil limiter or limit<=0 disables limiting (always allow),
// mirroring ChatwootRateLimiter.
type LoginRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string]*rateWindow
}

// NewLoginRateLimiter builds the limiter. limit<=0 disables (always allows);
// window<=0 uses 1 minute.
func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	if window <= 0 {
		window = time.Minute
	}
	return &LoginRateLimiter{limit: limit, window: window, now: time.Now, hits: make(map[string]*rateWindow)}
}

// Allow consumes 1 of the IP budget and reports whether the attempt may
// proceed.
func (l *LoginRateLimiter) Allow(ip string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.hits) >= MaxLoginBuckets {
		for key, w := range l.hits {
			if !now.Before(w.reset) {
				delete(l.hits, key)
			}
		}
	}
	w, ok := l.hits[ip]
	if !ok && len(l.hits) >= MaxLoginBuckets {
		return false
	}
	if !ok || !now.Before(w.reset) {
		l.hits[ip] = &rateWindow{count: 1, reset: now.Add(l.window)}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

// LoginLimit rejects password guesses over the IP budget with the shared
// 429 envelope before the handler burns a bcrypt comparison.
func LoginLimit(limiter *LoginRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(ClientIP(r)) {
			core.Error(w, r, http.StatusTooManyRequests, "rate_limited", "rate limited")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP reports the TCP peer IP of r without the port. Only RemoteAddr is
// trusted: X-Forwarded-For is client-controlled without a trusted-proxy list
// and would let an attacker rotate identities past the login budget.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type rateWindow struct {
	count int
	reset time.Time
}
