package notify

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStagingStore_StageReturnsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStagingStore(dir)
	if err != nil {
		t.Fatalf("NewStagingStore: %v", err)
	}
	id, path, size, err := store.Stage("report.md", bytes.NewReader([]byte("hello")), 5)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if id == "" || path == "" {
		t.Fatalf("id/path empty: %q / %q", id, path)
	}
	if size != 5 {
		t.Fatalf("size = %d, want 5", size)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("path not absolute: %q", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q", got)
	}

	resolved, ok := store.Lookup(id)
	if !ok || resolved != path {
		t.Fatalf("Lookup mismatch: ok=%v resolved=%q", ok, resolved)
	}
	if err := store.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, exists := store.Lookup(id); exists {
		t.Fatalf("expected Lookup to fail after Remove")
	}
}

func TestStagingStore_SizeMismatchRejectsFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStagingStore(dir)
	if err != nil {
		t.Fatalf("NewStagingStore: %v", err)
	}
	_, _, _, err = store.Stage("mismatch.bin", bytes.NewReader([]byte("hi")), 99)
	if err == nil {
		t.Fatal("expected error for size mismatch")
	}
}

func TestStagingStore_TTLWorkerEvictsExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStagingStore(dir)
	if err != nil {
		t.Fatalf("NewStagingStore: %v", err)
	}
	store.ttl = time.Millisecond
	fixedNow := time.Now()
	store.now = func() time.Time { return fixedNow }

	id, _, _, err := store.Stage("t.txt", bytes.NewReader([]byte("x")), 1)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	store.now = func() time.Time { return fixedNow.Add(time.Hour) }
	store.collectExpired()
	if _, ok := store.Lookup(id); ok {
		t.Fatal("expected expired entry to be evicted")
	}
}

func TestStagingStore_StartupCleansResidualFiles(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "residual.bin")
	if err := os.WriteFile(old, []byte("stale"), 0o600); err != nil {
		t.Fatalf("seed residual: %v", err)
	}
	if _, err := NewStagingStore(dir); err != nil {
		t.Fatalf("NewStagingStore: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("residual file still present: err=%v", err)
	}
}
