// Package repository implements data access for PostgreSQL and Redis.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheRepository struct {
	client *redis.Client
}

func NewCacheRepository(client *redis.Client) *CacheRepository {
	return &CacheRepository{client: client}
}

// Refresh token operations

func (r *CacheRepository) SetRefreshToken(ctx context.Context, userID, token string, ttl time.Duration) error {
	if r.client == nil {
		return ErrCacheUnavailable
	}
	key := fmt.Sprintf("refresh:%s", userID)
	return r.client.Set(ctx, key, token, ttl).Err()
}

func (r *CacheRepository) GetRefreshToken(ctx context.Context, userID string) (string, error) {
	if r.client == nil {
		return "", ErrCacheUnavailable
	}
	key := fmt.Sprintf("refresh:%s", userID)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("get refresh token: %w", err)
	}
	return val, nil
}

func (r *CacheRepository) DeleteRefreshToken(ctx context.Context, userID string) error {
	if r.client == nil {
		return ErrCacheUnavailable
	}
	key := fmt.Sprintf("refresh:%s", userID)
	return r.client.Del(ctx, key).Err()
}

// Token blacklist (for invalidated access tokens)

func (r *CacheRepository) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	if r.client == nil {
		return ErrCacheUnavailable
	}
	key := fmt.Sprintf("blacklist:%s", jti)
	return r.client.Set(ctx, key, "1", ttl).Err()
}

// IsBlacklisted checks if a token JTI has been revoked.
// Returns false when cache is unavailable (fail-open in degraded mode).
func (r *CacheRepository) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	if r.client == nil {
		return false, nil
	}
	key := fmt.Sprintf("blacklist:%s", jti)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return exists > 0, nil
}
