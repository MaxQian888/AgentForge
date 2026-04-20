package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupMiniRedis(t *testing.T) (*repository.CacheRepository, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return repository.NewCacheRepository(client), s
}

// --- Nil client tests ---

func TestNewCacheRepository(t *testing.T) {
	repo := repository.NewCacheRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil CacheRepository")
	}
}

func TestCacheRepository_SetRefreshToken_NilClient(t *testing.T) {
	repo := repository.NewCacheRepository(nil)
	err := repo.SetRefreshToken(context.Background(), "user1", "token", time.Hour)
	if err != repository.ErrCacheUnavailable {
		t.Errorf("expected ErrCacheUnavailable, got %v", err)
	}
}

func TestCacheRepository_GetRefreshToken_NilClient(t *testing.T) {
	repo := repository.NewCacheRepository(nil)
	_, err := repo.GetRefreshToken(context.Background(), "user1")
	if err != repository.ErrCacheUnavailable {
		t.Errorf("expected ErrCacheUnavailable, got %v", err)
	}
}

func TestCacheRepository_DeleteRefreshToken_NilClient(t *testing.T) {
	repo := repository.NewCacheRepository(nil)
	err := repo.DeleteRefreshToken(context.Background(), "user1")
	if err != repository.ErrCacheUnavailable {
		t.Errorf("expected ErrCacheUnavailable, got %v", err)
	}
}

func TestCacheRepository_BlacklistToken_NilClient(t *testing.T) {
	repo := repository.NewCacheRepository(nil)
	err := repo.BlacklistToken(context.Background(), "jti-1", time.Hour)
	if err != repository.ErrCacheUnavailable {
		t.Errorf("expected ErrCacheUnavailable, got %v", err)
	}
}

func TestCacheRepository_IsBlacklisted_NilClient(t *testing.T) {
	repo := repository.NewCacheRepository(nil)
	_, err := repo.IsBlacklisted(context.Background(), "jti-1")
	if err != repository.ErrCacheUnavailable {
		t.Errorf("expected ErrCacheUnavailable, got %v", err)
	}
}

// --- Miniredis tests ---

func TestCacheRepository_RefreshToken_RoundTrip(t *testing.T) {
	repo, _ := setupMiniRedis(t)
	ctx := context.Background()

	err := repo.SetRefreshToken(ctx, "user1", "my-token", time.Hour)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	token, err := repo.GetRefreshToken(ctx, "user1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if token != "my-token" {
		t.Errorf("expected my-token, got %s", token)
	}

	err = repo.DeleteRefreshToken(ctx, "user1")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = repo.GetRefreshToken(ctx, "user1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCacheRepository_GetRefreshToken_Missing(t *testing.T) {
	repo, _ := setupMiniRedis(t)
	_, err := repo.GetRefreshToken(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestCacheRepository_Blacklist_RoundTrip(t *testing.T) {
	repo, _ := setupMiniRedis(t)
	ctx := context.Background()

	// Not blacklisted yet
	blacklisted, err := repo.IsBlacklisted(ctx, "jti-1")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if blacklisted {
		t.Error("expected not blacklisted initially")
	}

	// Blacklist
	err = repo.BlacklistToken(ctx, "jti-1", time.Hour)
	if err != nil {
		t.Fatalf("blacklist: %v", err)
	}

	// Should be blacklisted now
	blacklisted, err = repo.IsBlacklisted(ctx, "jti-1")
	if err != nil {
		t.Fatalf("check after blacklist: %v", err)
	}
	if !blacklisted {
		t.Error("expected blacklisted after BlacklistToken")
	}
}

func TestCacheRepository_RefreshToken_Expiry(t *testing.T) {
	repo, s := setupMiniRedis(t)
	ctx := context.Background()

	err := repo.SetRefreshToken(ctx, "user1", "expiring-token", time.Second)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// Fast-forward time
	s.FastForward(2 * time.Second)

	_, err = repo.GetRefreshToken(ctx, "user1")
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestCacheRepository_WidgetDataRoundTrip(t *testing.T) {
	repo, _ := setupMiniRedis(t)
	ctx := context.Background()

	if err := repo.SetWidgetData(ctx, "project-1:throughput", `{"points":[]}`, time.Minute); err != nil {
		t.Fatalf("SetWidgetData() error = %v", err)
	}
	value, err := repo.GetWidgetData(ctx, "project-1:throughput")
	if err != nil {
		t.Fatalf("GetWidgetData() error = %v", err)
	}
	if value != `{"points":[]}` {
		t.Fatalf("widget payload = %s", value)
	}
}
