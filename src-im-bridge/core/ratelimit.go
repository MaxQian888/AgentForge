package core

import (
	"context"
	"sync"
	"time"
)

// RateStore is the durable-persistence contract the limiter relies on when
// running with the state package. core/state.Store satisfies it.
type RateStore interface {
	Record(scopeKey, policyID string, ts time.Time) error
	Count(scopeKey, policyID string, since time.Time) (int, error)
}

// RateLimiter evaluates a list of RateLimitPolicy against each incoming
// Scope. It can run against an in-memory sliding window (good for tests
// and single-process bridges) or delegate to a durable RateStore so state
// survives restarts.
type RateLimiter struct {
	mu       sync.Mutex
	policies []RateLimitPolicy
	memory   map[string][]time.Time // composite key → timestamps (fallback)
	store    RateStore
	now      func() time.Time
}

// NewRateLimiter creates a limiter with the given policies. Passing nil or
// empty policies yields DefaultPolicies.
func NewRateLimiter(policies []RateLimitPolicy) *RateLimiter {
	if len(policies) == 0 {
		policies = DefaultPolicies()
	}
	return &RateLimiter{
		policies: policies,
		memory:   make(map[string][]time.Time),
		now:      time.Now,
	}
}

// NewLegacyRateLimiter is a backward-compat constructor that produces a
// limiter with a single policy equivalent to the former global
// (rate, window) per (chat, user) sliding window.
func NewLegacyRateLimiter(rate int, window time.Duration) *RateLimiter {
	if rate <= 0 {
		rate = 20
	}
	if window <= 0 {
		window = time.Minute
	}
	return NewRateLimiter([]RateLimitPolicy{{
		ID:         "session-default",
		Dimensions: []RateDimension{DimChat, DimUser},
		Rate:       rate,
		Window:     window,
	}})
}

// SetStore wires a durable store for rate counters. When set, the limiter
// uses the store as authoritative and the in-memory fallback is unused.
func (rl *RateLimiter) SetStore(store RateStore) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.store = store
}

// Allow checks the scope against all applicable policies. On first
// rejection it returns the offending policy id + retry-after seconds and
// does NOT record a new event for any policy. On full acceptance it
// records an event for each applicable policy.
//
// The in-memory path serializes check-and-record under the receiver mutex
// so concurrent callers cannot simultaneously observe the pre-record
// count and each claim the last slot. The store path relies on the
// store's own write serialization (core/state uses WAL + a single writer)
// for the same guarantee.
func (rl *RateLimiter) Allow(ctx context.Context, scope Scope) (RateDecision, error) {
	now := rl.now()

	rl.mu.Lock()
	store := rl.store
	policies := append([]RateLimitPolicy(nil), rl.policies...)
	// When running with a durable store we must release the mutex before
	// making the (potentially slow) store calls. When running on memory
	// we hold the mutex across check+record to keep the decision atomic.
	if store != nil {
		rl.mu.Unlock()
		return rl.allowStore(store, policies, scope, now)
	}
	defer rl.mu.Unlock()
	return rl.allowMemoryLocked(policies, scope, now), nil
}

func (rl *RateLimiter) allowMemoryLocked(policies []RateLimitPolicy, scope Scope, now time.Time) RateDecision {
	type applied struct {
		policy RateLimitPolicy
		key    string
	}
	var toRecord []applied
	for _, policy := range policies {
		key := compositeKey(policy, scope)
		if key == "" {
			continue
		}
		since := now.Add(-policy.Window)
		timestamps := rl.memory[key]
		for len(timestamps) > 0 && timestamps[0].Before(since) {
			timestamps = timestamps[1:]
		}
		rl.memory[key] = timestamps
		if len(timestamps) >= policy.Rate {
			return RateDecision{
				Allowed:       false,
				Policy:        policy.ID,
				RetryAfterSec: retryAfterSeconds(policy.Window),
			}
		}
		toRecord = append(toRecord, applied{policy: policy, key: key})
	}
	for _, r := range toRecord {
		rl.memory[r.key] = append(rl.memory[r.key], now)
	}
	return RateDecision{Allowed: true}
}

func (rl *RateLimiter) allowStore(store RateStore, policies []RateLimitPolicy, scope Scope, now time.Time) (RateDecision, error) {
	type applied struct {
		policy RateLimitPolicy
		key    string
	}
	var toRecord []applied
	for _, policy := range policies {
		key := compositeKey(policy, scope)
		if key == "" {
			continue
		}
		since := now.Add(-policy.Window)
		count, err := store.Count(key, policy.ID, since)
		if err != nil {
			return RateDecision{}, err
		}
		if count >= policy.Rate {
			return RateDecision{
				Allowed:       false,
				Policy:        policy.ID,
				RetryAfterSec: retryAfterSeconds(policy.Window),
			}, nil
		}
		toRecord = append(toRecord, applied{policy: policy, key: key})
	}
	for _, r := range toRecord {
		if err := store.Record(r.key, r.policy.ID, now); err != nil {
			return RateDecision{}, err
		}
	}
	return RateDecision{Allowed: true}, nil
}

// AllowLegacy is a compatibility shim letting pre-refactor call sites
// that still hold a single composite string key continue to work against
// the default session policy. Returns true iff all applicable policies
// admit the event.
func (rl *RateLimiter) AllowLegacy(key string) bool {
	decision, err := rl.Allow(context.Background(), Scope{User: key, Chat: key})
	if err != nil {
		return false
	}
	return decision.Allowed
}

// Cleanup prunes stale in-memory buckets. When a durable store is wired
// the store handles its own eviction, and Cleanup is a no-op.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.store != nil {
		return
	}
	var longest time.Duration
	for _, p := range rl.policies {
		if p.Window > longest {
			longest = p.Window
		}
	}
	if longest <= 0 {
		longest = time.Minute
	}
	cutoff := rl.now().Add(-longest * 2)
	for key, timestamps := range rl.memory {
		if len(timestamps) == 0 || timestamps[len(timestamps)-1].Before(cutoff) {
			delete(rl.memory, key)
		}
	}
}

// Policies returns the currently-active policy list (read-only copy).
func (rl *RateLimiter) Policies() []RateLimitPolicy {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	out := make([]RateLimitPolicy, len(rl.policies))
	copy(out, rl.policies)
	return out
}

// setNow is a test hook.
func (rl *RateLimiter) setNow(fn func() time.Time) { rl.now = fn }

func retryAfterSeconds(window time.Duration) int {
	s := int(window.Seconds())
	if s <= 0 {
		return 1
	}
	return s
}
