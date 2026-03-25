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

func TestPluginInstanceRepository_ClonesNestedMCPMetadataInMemory(t *testing.T) {
	ctx := context.Background()
	repo := NewPluginInstanceRepository()

	snapshot := &model.PluginInstanceSnapshot{
		PluginID:       "repo-search",
		RuntimeHost:    model.PluginHostTSBridge,
		LifecycleState: model.PluginStateActive,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			Compatible: true,
			MCP: &model.PluginMCPRuntimeMetadata{
				Transport: "stdio",
				ToolCount: 2,
				LatestInteraction: &model.MCPInteractionSummary{
					Operation: model.MCPInteractionRefresh,
					Status:    model.MCPInteractionSucceeded,
					Summary:   "tools=2",
				},
			},
		},
	}

	if err := repo.UpsertCurrent(ctx, snapshot); err != nil {
		t.Fatalf("upsert current: %v", err)
	}

	snapshot.RuntimeMetadata.MCP.ToolCount = 99
	snapshot.RuntimeMetadata.MCP.LatestInteraction.Summary = "mutated"

	loaded, err := repo.GetCurrentByPluginID(ctx, "repo-search")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if loaded.RuntimeMetadata == nil || loaded.RuntimeMetadata.MCP == nil {
		t.Fatalf("expected MCP metadata, got %+v", loaded.RuntimeMetadata)
	}
	if loaded.RuntimeMetadata.MCP.ToolCount != 2 {
		t.Fatalf("tool count = %d, want 2", loaded.RuntimeMetadata.MCP.ToolCount)
	}
	if loaded.RuntimeMetadata.MCP.LatestInteraction == nil || loaded.RuntimeMetadata.MCP.LatestInteraction.Summary != "tools=2" {
		t.Fatalf("latest interaction = %+v, want preserved summary", loaded.RuntimeMetadata.MCP.LatestInteraction)
	}
}
