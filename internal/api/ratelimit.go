package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	lim  *rate.Limiter
	seen time.Time
}

// limiter is a per-IP token bucket.
type limiter struct {
	mu    sync.Mutex
	ips   map[string]*entry
	rate  rate.Limit
	burst int
}

func newLimiter(r rate.Limit, burst int) *limiter {
	l := &limiter{ips: map[string]*entry{}, rate: r, burst: burst}
	go func() { // forget idle IPs
		for range time.Tick(5 * time.Minute) {
			l.mu.Lock()
			for ip, e := range l.ips {
				if time.Since(e.seen) > 10*time.Minute {
					delete(l.ips, ip)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.ips[ip]
	if !ok {
		e = &entry{lim: rate.NewLimiter(l.rate, l.burst)}
		l.ips[ip] = e
	}
	e.seen = time.Now()
	return e.lim.Allow()
}

func (l *limiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "slow down, rate limit hit", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP trusts proxy headers. That's right behind Fly/Cloudflare/nginx,
// but spoofable if the node is exposed directly. Revisit at deploy time.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
		return ip
	}
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}