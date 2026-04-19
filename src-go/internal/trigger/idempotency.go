// Package trigger implements workflow trigger primitives including
// idempotent event deduplication.
package trigger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrIdempotencyStoreUnavailable is returned when the underlying Redis client
// is nil and no deduplication check can be performed.
var ErrIdempotencyStoreUnavailable = errors.New("idempotency store unavailable")

const idempotencyKeyPrefix = "af:trigger:idem:"

// IdempotencyStore remembers which trigger-firing keys were seen recently,
// so duplicate events within a dedupe window can be skipped.
type IdempotencyStore interface {
	// SeenWithin reports whether key was seen in the last `window`.
	// On a first-sight, records the key for future checks.
	// A zero/negative window disables deduplication: the key is never
	// recorded and the method always reports false.
	SeenWithin(ctx context.Context, key string, window time.Duration) (bool, error)
}

// RedisIdempotencyStore is the production implementation backed by Redis SETNX.
type RedisIdempotencyStore struct {
	rdb *redis.Client
}

// NewRedisIdempotencyStore creates a new RedisIdempotencyStore backed by rdb.
func NewRedisIdempotencyStore(rdb *redis.Client) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{rdb: rdb}
}

// SeenWithin reports whether key was seen within the last window duration.
// When window is zero or negative, deduplication is disabled: the method
// always returns false without recording anything.
// When the Redis client is nil, ErrIdempotencyStoreUnavailable is returned.
func (s *RedisIdempotencyStore) SeenWithin(ctx context.Context, key string, window time.Duration) (bool, error) {
	if window <= 0 {
		return false, nil
	}
	if s.rdb == nil {
		return false, ErrIdempotencyStoreUnavailable
	}

	redisKey := fmt.Sprintf("%s%s", idempotencyKeyPrefix, key)
	// SetNX returns true if the key was newly set (fresh insert),
	// false if the key already existed.
	inserted, err := s.rdb.SetNX(ctx, redisKey, "1", window).Result()
	if err != nil {
		return false, fmt.Errorf("idempotency check: %w", err)
	}
	// inserted=true  → key was brand new → not seen before → return false
	// inserted=false → key already existed → seen within window → return true
	return !inserted, nil
}
