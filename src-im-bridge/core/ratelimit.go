package core

import (
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*slidingWindow
	rate    int
	window  time.Duration
	now     func() time.Time
}

type slidingWindow struct {
	timestamps []time.Time
}

// NewRateLimiter creates a rate limiter allowing `rate` messages per `window`.
// Default: 20 per minute.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	if rate <= 0 {
		rate = 20
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		buckets: make(map[string]*slidingWindow),
		rate:    rate,
		window:  window,
		now:     time.Now,
	}
}

// Allow returns true if the key has not exceeded the rate limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	w, exists := rl.buckets[key]
	if !exists {
		w = &slidingWindow{}
		rl.buckets[key] = w
	}

	// Evict timestamps outside the window.
	cutoff := now.Add(-rl.window)
	start := 0
	for start < len(w.timestamps) && w.timestamps[start].Before(cutoff) {
		start++
	}
	w.timestamps = w.timestamps[start:]

	if len(w.timestamps) >= rl.rate {
		return false
	}

	w.timestamps = append(w.timestamps, now)
	return true
}

// Cleanup removes stale buckets. Call periodically to prevent memory growth.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := rl.now().Add(-rl.window * 2)
	for key, w := range rl.buckets {
		if len(w.timestamps) == 0 || w.timestamps[len(w.timestamps)-1].Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}
