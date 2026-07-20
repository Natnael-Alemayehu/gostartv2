package middleware

import (
	"gostartv2/internal/httpx"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// rateLimiter holds a per-IP rate limiter. Each client IP gets its own
// *rate.Limiter, created lazily on first request. Limiters for IPs that
// have not been seen in a while are not purged — for a starter template this
// is acceptable; production services with high cardinality should replace
// this with a Redis-backed limiter or an LRU cache.
type rateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

func (rl *rateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if l, ok := rl.limiters[ip]; ok {
		return l
	}

	l := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[ip] = l

	return l
}

// RateLimit returns an HTTP middleware that limits each client IP to rps
// requests per second with the given burst size. When the limit is exceeded
// the middleware responds with 429 Too Many Requests.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	rl := newRateLimiter(rps, burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if ip == "" {
				ip = "unknown"
			}

			if !rl.get(ip).Allow() {
				httpx.RespondError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
