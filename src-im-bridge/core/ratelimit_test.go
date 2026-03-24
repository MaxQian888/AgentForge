package core

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.Allow("user:1") {
			t.Fatalf("request %d denied, expected allowed", i+1)
		}
	}
}

func TestRateLimiter_DeniesOverLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("user:1")
	}
	if rl.Allow("user:1") {
		t.Fatal("4th request allowed, expected denied")
	}
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	fakeNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(2, time.Minute)
	rl.now = func() time.Time { return fakeNow }

	rl.Allow("user:1")
	rl.Allow("user:1")
	if rl.Allow("user:1") {
		t.Fatal("3rd request allowed, expected denied")
	}

	// Advance past the window.
	fakeNow = fakeNow.Add(61 * time.Second)
	if !rl.Allow("user:1") {
		t.Fatal("request after window denied, expected allowed")
	}
}

func TestRateLimiter_IsolatesByKey(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	if !rl.Allow("user:1") {
		t.Fatal("user:1 denied")
	}
	if rl.Allow("user:1") {
		t.Fatal("user:1 second request allowed")
	}
	// Different key should still be allowed.
	if !rl.Allow("user:2") {
		t.Fatal("user:2 denied")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	fakeNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(5, time.Minute)
	rl.now = func() time.Time { return fakeNow }

	rl.Allow("user:1")

	// Advance well past the cleanup threshold.
	fakeNow = fakeNow.Add(5 * time.Minute)
	rl.Cleanup()

	rl.mu.Lock()
	count := len(rl.buckets)
	rl.mu.Unlock()
	if count != 0 {
		t.Fatalf("buckets = %d, want 0 after cleanup", count)
	}
}
