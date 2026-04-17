package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHashID_DeterministicAndEmptyPreserved(t *testing.T) {
	salt := "deadbeef"
	if got := HashID(salt, ""); got != "" {
		t.Fatalf("empty input should yield empty hash, got %q", got)
	}
	a := HashID(salt, "chat-42")
	b := HashID(salt, "chat-42")
	if a != b {
		t.Fatalf("same input should yield same hash: %s vs %s", a, b)
	}
	c := HashID(salt, "chat-43")
	if a == c {
		t.Fatalf("different inputs should yield different hashes")
	}
	if len(a) != 16 {
		t.Fatalf("expected 16 hex chars, got %d (%q)", len(a), a)
	}
}

func TestHashID_DifferentSalt(t *testing.T) {
	a := HashID("salt-a", "user-1")
	b := HashID("salt-b", "user-1")
	if a == b {
		t.Fatalf("different salts must produce different hashes")
	}
}

func TestFileWriter_EmitAppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	events := []Event{
		{Direction: DirectionIngress, Surface: "/im/send", DeliveryID: "d1", Status: StatusDelivered, Platform: "feishu"},
		{Direction: DirectionEgress, Surface: "/im/notify", DeliveryID: "d2", Status: StatusDuplicate, Platform: "feishu"},
	}
	for _, e := range events {
		if err := w.Emit(e); err != nil {
			t.Fatal(err)
		}
	}

	_ = w.Close()
	f, err := os.Open(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) != len(events) {
		t.Fatalf("lines = %d, want %d", len(lines), len(events))
	}
	for i, line := range lines {
		var got Event
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d unmarshal: %v", i, err)
		}
		if got.V != SchemaVersion {
			t.Fatalf("line %d V = %d, want %d", i, got.V, SchemaVersion)
		}
		if got.Ts.IsZero() {
			t.Fatalf("line %d missing ts", i)
		}
		if got.DeliveryID != events[i].DeliveryID {
			t.Fatalf("line %d deliveryId mismatch: %s", i, got.DeliveryID)
		}
	}
}

func TestFileWriter_EmitConcurrent(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	const workers = 16
	const perWorker = 64
	var wg sync.WaitGroup
	for g := 0; g < workers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if err := w.Emit(Event{Direction: DirectionIngress, DeliveryID: "dx"}); err != nil {
					t.Errorf("emit: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	_ = w.Close()

	data, _ := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != workers*perWorker {
		t.Fatalf("lines = %d, want %d", len(lines), workers*perWorker)
	}
	// All lines should be valid JSON objects (no interleaving).
	for i, line := range lines {
		var any map[string]any
		if err := json.Unmarshal([]byte(line), &any); err != nil {
			t.Fatalf("line %d invalid json: %v", i, err)
		}
	}
}

func TestRotatingWriter_RotatesBySize(t *testing.T) {
	dir := t.TempDir()
	tick := atomic.Int64{}
	tick.Store(time.Now().Unix())
	clock := func() time.Time { return time.Unix(tick.Load(), 0) }

	w, err := New(RotatingConfig{
		Dir:          dir,
		MaxSizeBytes: 1024, // small for testing
		Now:          clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// Emit ~200 events; each ~120 bytes → crosses 1KB threshold several times.
	for i := 0; i < 200; i++ {
		if err := w.Emit(Event{Direction: DirectionIngress, DeliveryID: "d", Metadata: map[string]string{"i": strings.Repeat("x", 50)}}); err != nil {
			t.Fatal(err)
		}
		tick.Add(1) // advance one sec per event so rotated file names don't collide
	}

	rot := w.(*RotatingWriter)
	names, err := rot.ListRotated()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatalf("expected at least one rotated file")
	}
	// Verify current file still exists.
	if _, err := os.Stat(filepath.Join(dir, "audit.jsonl")); err != nil {
		t.Fatalf("current file should exist: %v", err)
	}
}

func TestRotatingWriter_RetentionRemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed some files with mtimes before the retention window.
	old := filepath.Join(dir, "audit.2020-01-01-0000.jsonl")
	if err := os.WriteFile(old, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(old, time.Now().Add(-30*24*time.Hour), time.Now().Add(-30*24*time.Hour))

	fresh := filepath.Join(dir, "audit.2026-04-17-1200.jsonl")
	if err := os.WriteFile(fresh, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tick := atomic.Int64{}
	tick.Store(time.Now().Unix())
	clock := func() time.Time { return time.Unix(tick.Load(), 0) }

	w, err := New(RotatingConfig{
		Dir:            dir,
		MaxSizeBytes:   256,
		RetainDuration: 14 * 24 * time.Hour,
		Now:            clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// Push a batch to trigger at least one rotate → which calls prune.
	for i := 0; i < 50; i++ {
		_ = w.Emit(Event{Direction: DirectionIngress, Metadata: map[string]string{"i": strings.Repeat("z", 30)}})
		tick.Add(1)
	}

	if _, err := os.Stat(old); err == nil {
		t.Fatalf("old rotated file should be pruned")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("recent rotated file should be preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "audit.jsonl")); err != nil {
		t.Fatalf("current file must never be pruned")
	}
}

func TestRotatingWriter_DisabledReturnsNop(t *testing.T) {
	w, err := New(RotatingConfig{DisableWriter: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := w.(NopWriter); !ok {
		t.Fatalf("expected NopWriter, got %T", w)
	}
	if err := w.Emit(Event{Action: "whatever"}); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
}

func TestGenerateSalt(t *testing.T) {
	a, err := GenerateSalt()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateSalt()
	if a == b {
		t.Fatalf("salts should differ")
	}
	if len(a) != 64 {
		t.Fatalf("salt length = %d, want 64 hex chars", len(a))
	}
}
