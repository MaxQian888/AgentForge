package trigger_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentforge/server/internal/trigger"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupMiniRedis spins up an in-memory Redis and returns a connected client
// along with the miniredis server so tests can inspect internal state.
func setupMiniRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return client, s
}

func TestNewRedisIdempotencyStore_Constructs(t *testing.T) {
	client, _ := setupMiniRedis(t)
	store := trigger.NewRedisIdempotencyStore(client)
	if store == nil {
		t.Fatal("expected non-nil RedisIdempotencyStore")
	}
}

func TestIdempotencyStore_FirstSeenAllows(t *testing.T) {
	client, _ := setupMiniRedis(t)
	store := trigger.NewRedisIdempotencyStore(client)

	seen, err := store.SeenWithin(context.Background(), "event-1", 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Error("expected seen=false for a fresh key, got true")
	}
}

func TestIdempotencyStore_SecondWithinWindow(t *testing.T) {
	client, _ := setupMiniRedis(t)
	store := trigger.NewRedisIdempotencyStore(client)

	ctx := context.Background()
	_, _ = store.SeenWithin(ctx, "event-dup", 60*time.Second) // first call records the key

	seen, err := store.SeenWithin(ctx, "event-dup", 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if !seen {
		t.Error("expected seen=true on second call within window, got false")
	}
}

func TestIdempotencyStore_ZeroWindowDisabled(t *testing.T) {
	client, s := setupMiniRedis(t)
	store := trigger.NewRedisIdempotencyStore(client)

	ctx := context.Background()

	seen1, err := store.SeenWithin(ctx, "event-zero", 0)
	if err != nil {
		t.Fatalf("unexpected error (first call, zero window): %v", err)
	}
	if seen1 {
		t.Error("expected seen=false with zero window (first call), got true")
	}

	seen2, err := store.SeenWithin(ctx, "event-zero", 0)
	if err != nil {
		t.Fatalf("unexpected error (second call, zero window): %v", err)
	}
	if seen2 {
		t.Error("expected seen=false with zero window (second call), got true")
	}

	// Verify no Redis key was created.
	if len(s.Keys()) != 0 {
		t.Errorf("expected no Redis keys after zero-window calls, found: %v", s.Keys())
	}
}

func TestIdempotencyStore_NegativeWindowDisabled(t *testing.T) {
	client, s := setupMiniRedis(t)
	store := trigger.NewRedisIdempotencyStore(client)

	ctx := context.Background()

	seen1, err := store.SeenWithin(ctx, "event-neg", -1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error (first call, negative window): %v", err)
	}
	if seen1 {
		t.Error("expected seen=false with negative window (first call), got true")
	}

	seen2, err := store.SeenWithin(ctx, "event-neg", -1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error (second call, negative window): %v", err)
	}
	if seen2 {
		t.Error("expected seen=false with negative window (second call), got true")
	}

	// Verify no Redis key was created.
	if len(s.Keys()) != 0 {
		t.Errorf("expected no Redis keys after negative-window calls, found: %v", s.Keys())
	}
}

func TestIdempotencyStore_SeparateKeysIndependent(t *testing.T) {
	client, _ := setupMiniRedis(t)
	store := trigger.NewRedisIdempotencyStore(client)

	ctx := context.Background()

	seenA, err := store.SeenWithin(ctx, "key-A", 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error for key-A: %v", err)
	}
	if seenA {
		t.Error("expected seen=false for first call to key-A, got true")
	}

	seenB, err := store.SeenWithin(ctx, "key-B", 60*time.Second)
	if err != nil {
		t.Fatalf("unexpected error for key-B: %v", err)
	}
	if seenB {
		t.Error("expected seen=false for first call to key-B (independent of key-A), got true")
	}
}

func TestIdempotencyStore_NilClient_ReturnsError(t *testing.T) {
	store := trigger.NewRedisIdempotencyStore(nil)

	seen, err := store.SeenWithin(context.Background(), "any-key", 60*time.Second)
	if err == nil {
		t.Fatal("expected a non-nil error when client is nil, got nil")
	}
	if !errors.Is(err, trigger.ErrIdempotencyStoreUnavailable) {
		t.Errorf("expected ErrIdempotencyStoreUnavailable, got: %v", err)
	}
	if seen {
		t.Error("expected seen=false when client is nil, got true")
	}
}
