package repository

import (
	"context"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestPluginInstanceRepository_UpsertAndGetCurrentInMemory(t *testing.T) {
	ctx := context.Background()
	repo := NewPluginInstanceRepository()

	snapshot := &model.PluginInstanceSnapshot{
		PluginID:           "feishu",
		RuntimeHost:        model.PluginHostGoOrchestrator,
		LifecycleState:     model.PluginStateActive,
		ResolvedSourcePath: "./dist/feishu.wasm",
		RestartCount:       1,
		LastError:          "",
	}

	if err := repo.UpsertCurrent(ctx, snapshot); err != nil {
		t.Fatalf("upsert current: %v", err)
	}

	loaded, err := repo.GetCurrentByPluginID(ctx, "feishu")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if loaded.PluginID != "feishu" {
		t.Fatalf("plugin id = %q, want feishu", loaded.PluginID)
	}
	if loaded.RestartCount != 1 {
		t.Fatalf("restart count = %d, want 1", loaded.RestartCount)
	}

	snapshot.LifecycleState = model.PluginStateDegraded
	snapshot.LastError = "runtime failure"
	if err := repo.UpsertCurrent(ctx, snapshot); err != nil {
		t.Fatalf("upsert degraded current: %v", err)
	}

	updated, err := repo.GetCurrentByPluginID(ctx, "feishu")
	if err != nil {
		t.Fatalf("get updated current: %v", err)
	}
	if updated.LifecycleState != model.PluginStateDegraded {
		t.Fatalf("lifecycle state = %s, want degraded", updated.LifecycleState)
	}
	if updated.LastError != "runtime failure" {
		t.Fatalf("last error = %q, want runtime failure", updated.LastError)
	}
}

func TestPluginInstanceRepository_DeleteCurrentInMemory(t *testing.T) {
	ctx := context.Background()
	repo := NewPluginInstanceRepository()

	if err := repo.UpsertCurrent(ctx, &model.PluginInstanceSnapshot{
		PluginID:       "repo-search",
		RuntimeHost:    model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
	}); err != nil {
		t.Fatalf("seed current instance: %v", err)
	}

	if err := repo.DeleteByPluginID(ctx, "repo-search"); err != nil {
		t.Fatalf("delete current instance: %v", err)
	}

	if _, err := repo.GetCurrentByPluginID(ctx, "repo-search"); err == nil {
		t.Fatal("expected deleted current instance lookup to fail")
	}
}
