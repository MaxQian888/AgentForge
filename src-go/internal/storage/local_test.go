package storage

import (
	"context"
	"io"
	"sort"
	"strings"
	"testing"
)

func TestLocalStorageRoundTripAndDelete(t *testing.T) {
	store := NewLocalStorage(t.TempDir())
	ctx := context.Background()
	key := "artifacts/report.txt"
	content := "build output"

	if err := store.Put(ctx, key, strings.NewReader(content), PutOptions{}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Fatal("Exists() = false, want true")
	}

	reader, info, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		_ = reader.Close()
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if string(data) != content {
		t.Fatalf("stored content = %q, want %q", string(data), content)
	}
	if info == nil {
		t.Fatal("Get() returned nil FileInfo")
	}
	if info.Key != key {
		t.Fatalf("info.Key = %q, want %q", info.Key, key)
	}
	if info.Size != int64(len(content)) {
		t.Fatalf("info.Size = %d, want %d", info.Size, len(content))
	}
	if info.CreatedAt.IsZero() {
		t.Fatal("info.CreatedAt should be populated")
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() after delete error = %v", err)
	}
	if exists {
		t.Fatal("Exists() after delete = true, want false")
	}
}

func TestLocalStorageListNormalizesPathsAndIgnoresMissingPrefixes(t *testing.T) {
	store := NewLocalStorage(t.TempDir())
	ctx := context.Background()

	files := map[string]string{
		"artifacts/a.txt":       "alpha",
		"artifacts/nested/b.md": "beta",
		"other/c.txt":           "gamma",
	}
	for key, content := range files {
		if err := store.Put(ctx, key, strings.NewReader(content), PutOptions{}); err != nil {
			t.Fatalf("Put(%q) error = %v", key, err)
		}
	}

	listed, err := store.List(ctx, "artifacts")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	sort.Slice(listed, func(i, j int) bool {
		return listed[i].Key < listed[j].Key
	})

	if len(listed) != 2 {
		t.Fatalf("len(List(artifacts)) = %d, want 2", len(listed))
	}
	if listed[0].Key != "artifacts/a.txt" {
		t.Fatalf("listed[0].Key = %q, want %q", listed[0].Key, "artifacts/a.txt")
	}
	if listed[1].Key != "artifacts/nested/b.md" {
		t.Fatalf("listed[1].Key = %q, want %q", listed[1].Key, "artifacts/nested/b.md")
	}
	if listed[0].Size != int64(len(files["artifacts/a.txt"])) {
		t.Fatalf("listed[0].Size = %d, want %d", listed[0].Size, len(files["artifacts/a.txt"]))
	}
	if listed[1].CreatedAt.IsZero() {
		t.Fatal("listed[1].CreatedAt should be populated")
	}

	missing, err := store.List(ctx, "missing")
	if err != nil {
		t.Fatalf("List(missing) error = %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("len(List(missing)) = %d, want 0", len(missing))
	}
}
