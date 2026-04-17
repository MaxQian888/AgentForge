package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStorePersistsHistoryAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")

	store, err := Open(Config{Path: path, CleanupInterval: -1})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s := NewSessionStore(store)
	for _, msg := range []string{"hi", "what's up", "deploy prod"} {
		if err := s.Append("acme", "sess1", msg); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	// Close and reopen to simulate a restart.
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	store2, err := Open(Config{Path: path, CleanupInterval: -1})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()
	s2 := NewSessionStore(store2)
	got := s2.Recent("acme", "sess1", 10)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	// Oldest first.
	if got[0] != "hi" || got[2] != "deploy prod" {
		t.Fatalf("wrong order: %v", got)
	}
}

func TestSessionStoreTenantScoped(t *testing.T) {
	s := NewSessionStore(nil)
	_ = s.Append("acme", "s", "hello")
	_ = s.Append("beta", "s", "bonjour")
	a := s.Recent("acme", "s", 10)
	b := s.Recent("beta", "s", 10)
	if len(a) != 1 || a[0] != "hello" {
		t.Fatalf("acme got %v", a)
	}
	if len(b) != 1 || b[0] != "bonjour" {
		t.Fatalf("beta got %v", b)
	}
}

func TestSessionStoreHistoryLimit(t *testing.T) {
	s := NewSessionStore(nil)
	s.SetHistoryLimit(3)
	for i := 0; i < 10; i++ {
		_ = s.Append("acme", "s", string(rune('a'+i)))
	}
	got := s.Recent("acme", "s", 100)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries after limit, got %d: %v", len(got), got)
	}
	// Should be the 3 most recent.
	if got[0] != "h" || got[1] != "i" || got[2] != "j" {
		t.Fatalf("expected last 3 entries, got %v", got)
	}
}

func TestIntentCacheTTL(t *testing.T) {
	s := NewSessionStore(nil)
	s.SetIntentTTL(50 * time.Millisecond)
	_ = s.IntentCacheSet("acme", "hash1", "deploy", 0.9)
	if _, _, ok := s.IntentCacheGet("acme", "hash1"); !ok {
		t.Fatal("expected cache hit")
	}
	time.Sleep(80 * time.Millisecond)
	if _, _, ok := s.IntentCacheGet("acme", "hash1"); ok {
		t.Fatal("expected cache expiry")
	}
}

func TestReplyBindingDelete(t *testing.T) {
	s := NewSessionStore(nil)
	if err := s.ReplyBindingPut("acme", "task-1", "{}", time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, ok := s.ReplyBindingGet("acme", "task-1"); !ok {
		t.Fatal("expected get hit")
	}
	if err := s.ReplyBindingDelete("acme", "task-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := s.ReplyBindingGet("acme", "task-1"); ok {
		t.Fatal("expected deleted binding to miss")
	}
}
