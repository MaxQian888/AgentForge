// src-go/internal/middleware/ratelimit_ingest.go
package middleware

import (
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// IngestRateLimit returns a per-remote-IP token bucket limiter.
// rps is sustained rate, burst is the initial bucket depth.
// Exceeded requests get HTTP 429 with an empty body.
func IngestRateLimit(rps float64, burst int) echo.MiddlewareFunc {
	var mu sync.Mutex
	buckets := map[string]*rate.Limiter{}

	limiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := buckets[ip]
		if !ok {
			l = rate.NewLimiter(rate.Limit(rps), burst)
			buckets[ip] = l
		}
		return l
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !limiter(c.RealIP()).Allow() {
				return c.NoContent(http.StatusTooManyRequests)
			}
			return next(c)
		}
	}
}
