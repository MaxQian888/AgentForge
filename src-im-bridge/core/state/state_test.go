package state

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestStore(t *testing.T, opts ...func(*Config)) *Store {
	t.Helper()
	cfg := Config{
		Path:            filepath.Join(t.TempDir(), "state.db"),
		CleanupInterval: -1, // disable background cleanup in tests unless opted-in
		RateRetention:   time.Hour,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	s, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDedupeSeen_FirstCallReturnsFalseSecondReturnsTrue(t *testing.T) {
	s := newTestStore(t)
	dup, err := s.Seen("d1", "/im/send", time.Minute)
	if err != nil || dup {
		t.Fatalf("first Seen got (dup=%v, err=%v); want (false, nil)", dup, err)
	}
	dup, err = s.Seen("d1", "/im/send", time.Minute)
	if err != nil || !dup {
		t.Fatalf("second Seen got (dup=%v, err=%v); want (true, nil)", dup, err)
	}
}

func TestDedupeSeen_ExpiredRecordIsRecycled(t *testing.T) {
	tick := atomic.Int64{}
	tick.Store(time.Now().Unix())
	s := newTestStore(t, func(c *Config) {
		c.Now = func() time.Time { return time.Unix(tick.Load(), 0) }
	})
	if dup, _ := s.Seen("d1", "x", time.Second); dup {
		t.Fatalf("first Seen unexpectedly duplicate")
	}
	// advance past ttl
	tick.Add(5)
	dup, err := s.Seen("d1", "x", time.Second)
	if err != nil || dup {
		t.Fatalf("after expiry: got (dup=%v, err=%v); want (false, nil)", dup, err)
	}
}

func TestDedupeSeen_ConcurrentSingleSuccess(t *testing.T) {
	s := newTestStore(t)
	const workers = 16
	var firstTime atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			dup, err := s.Seen("race", "surface", time.Minute)
			if err != nil {
				t.Errorf("Seen err: %v", err)
				return
			}
			if !dup {
				firstTime.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := firstTime.Load(); got != 1 {
		t.Fatalf("expected exactly one non-duplicate, got %d", got)
	}
}

func TestDedupeSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(Config{Path: path, CleanupInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	if dup, _ := s.Seen("d1", "x", time.Minute); dup {
		t.Fatal("first Seen duplicate")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(Config{Path: path, CleanupInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	dup, err := s2.Seen("d1", "x", time.Minute)
	if err != nil || !dup {
		t.Fatalf("after reopen Seen got (dup=%v, err=%v); want duplicate", dup, err)
	}
}

func TestNonceConsume_ReplayReturnsFalse(t *testing.T) {
	s := newTestStore(t)
	ok, err := s.Consume("n1", "control_plane", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first Consume got (ok=%v, err=%v); want (true, nil)", ok, err)
	}
	ok, err = s.Consume("n1", "control_plane", time.Minute)
	if err != nil || ok {
		t.Fatalf("replay Consume got (ok=%v, err=%v); want (false, nil)", ok, err)
	}
}

func TestNonceConsume_ScopeIsolation(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.Consume("n1", "a", time.Minute)
	ok, err := s.Consume("n1", "b", time.Minute)
	if err != nil || !ok {
		t.Fatalf("different scope should accept: ok=%v err=%v", ok, err)
	}
}

func TestRateRecordAndCount(t *testing.T) {
	s := newTestStore(t)
	base := time.Now()
	for i := 0; i < 5; i++ {
		if err := s.Record("user:bob", "write-action", base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	n, err := s.Count("user:bob", "write-action", base)
	if err != nil || n != 5 {
		t.Fatalf("Count got (%d, %v); want (5, nil)", n, err)
	}
	// window that excludes first two events
	n, err = s.Count("user:bob", "write-action", base.Add(2*time.Second))
	if err != nil || n != 3 {
		t.Fatalf("Count windowed got (%d, %v); want (3, nil)", n, err)
	}
	// different policy id isolates
	n, _ = s.Count("user:bob", "other", base)
	if n != 0 {
		t.Fatalf("policy isolation broken: got %d want 0", n)
	}
}

func TestCleanupRemovesExpired(t *testing.T) {
	tick := atomic.Int64{}
	tick.Store(time.Unix(1_000_000, 0).Unix())
	s := newTestStore(t, func(c *Config) {
		c.Now = func() time.Time { return time.Unix(tick.Load(), 0) }
		c.RateRetention = 10 * time.Second
	})
	_, _ = s.Seen("d1", "x", time.Second)
	_, _ = s.Consume("n1", "s", time.Second)
	_ = s.Record("scope", "policy", time.Unix(tick.Load(), 0))
	tick.Add(30)
	if err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	// dedupe record should be gone → next Seen returns false
	dup, _ := s.Seen("d1", "x", time.Minute)
	if dup {
		t.Fatalf("expired dedupe should be cleaned")
	}
	// nonce record should be gone → next Consume succeeds
	ok, _ := s.Consume("n1", "s", time.Minute)
	if !ok {
		t.Fatalf("expired nonce should be cleaned")
	}
	// rate count should be zero
	n, _ := s.Count("scope", "policy", time.Unix(0, 0))
	if n != 0 {
		t.Fatalf("expired rate rows should be cleaned: got %d", n)
	}
}

func TestCleanupDoesNotTouchLiveRows(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.Seen("d1", "x", time.Hour)
	_, _ = s.Consume("n1", "s", time.Hour)
	_ = s.Record("scope", "policy", time.Now())
	if err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	dup, _ := s.Seen("d1", "x", time.Hour)
	if !dup {
		t.Fatalf("live dedupe row should survive cleanup")
	}
	ok, _ := s.Consume("n1", "s", time.Hour)
	if ok {
		t.Fatalf("live nonce row should survive cleanup")
	}
	n, _ := s.Count("scope", "policy", time.Now().Add(-time.Hour))
	if n != 1 {
		t.Fatalf("live rate row should survive cleanup, got %d", n)
	}
}

func TestSettingsGetPut(t *testing.T) {
	s := newTestStore(t)
	if _, ok, _ := s.SettingsGet("missing"); ok {
		t.Fatalf("expected missing key")
	}
	if err := s.SettingsPut("audit_salt", "deadbeef"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.SettingsGet("audit_salt")
	if err != nil || !ok || got != "deadbeef" {
		t.Fatalf("SettingsGet got (%q, %v, %v)", got, ok, err)
	}
	// overwrite
	_ = s.SettingsPut("audit_salt", "cafebabe")
	got, _, _ = s.SettingsGet("audit_salt")
	if got != "cafebabe" {
		t.Fatalf("overwrite failed: %s", got)
	}
}

func TestOpen_FailsOnUnwritableDir(t *testing.T) {
	// Using an invalid path (null byte on Unix, invalid chars) is OS-dependent;
	// on Windows a missing drive letter path triggers a clear error.
	_, err := Open(Config{Path: "\x00/bad/state.db"})
	if err == nil {
		t.Fatalf("expected Open to fail on invalid path")
	}
}
