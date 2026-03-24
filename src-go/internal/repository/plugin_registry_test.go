package repository_test

import (
	"context"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestPluginRegistryRepository_PreservesExtendedSourceTrustAndReleaseMetadata(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewPluginRegistryRepository()

	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "workflow.release-train",
				Name:    "Release Train",
				Version: "1.2.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/release-train.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: "sequential",
					Roles: []model.WorkflowRoleBinding{
						{ID: "coder"},
					},
					Steps: []model.WorkflowStepDefinition{
						{ID: "implement", Role: "coder", Action: "agent"},
					},
				},
			},
			Source: model.PluginSource{
				Type:       model.PluginSourceGit,
				Path:       "./plugins/workflow/manifest.yaml",
				Repository: "https://github.com/example/release-train.git",
				Ref:        "refs/tags/v1.2.0",
				Digest:     "sha256:workflow",
				Trust: &model.PluginTrustMetadata{
					Status:        model.PluginTrustVerified,
					ApprovalState: model.PluginApprovalApproved,
					Source:        "cosign",
				},
				Release: &model.PluginReleaseMetadata{
					Version:  "1.2.0",
					Channel:  "stable",
					Artifact: "https://github.com/example/release-train/releases/download/v1.2.0/plugin.wasm",
				},
			},
		},
		LifecycleState: model.PluginStateInstalled,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}

	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("save plugin record: %v", err)
	}

	stored, err := repo.GetByID(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("get plugin record: %v", err)
	}

	if stored.Source.Repository != record.Source.Repository {
		t.Fatalf("repository = %q, want %q", stored.Source.Repository, record.Source.Repository)
	}
	if stored.Source.Trust == nil || stored.Source.Trust.Status != model.PluginTrustVerified {
		t.Fatalf("trust metadata = %+v, want verified", stored.Source.Trust)
	}
	if stored.Source.Release == nil || stored.Source.Release.Channel != "stable" {
		t.Fatalf("release metadata = %+v, want stable channel", stored.Source.Release)
	}
	if stored.Spec.Workflow == nil || stored.Spec.Workflow.Process != "sequential" {
		t.Fatalf("workflow spec = %+v, want sequential", stored.Spec.Workflow)
	}
}

func TestPluginRegistryRepository_ListFiltersBySourceAndTrustState(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewPluginRegistryRepository()

	records := []*model.PluginRecord{
		{
			PluginManifest: model.PluginManifest{
				APIVersion: "agentforge/v1",
				Kind:       model.PluginKindReview,
				Metadata: model.PluginMetadata{
					ID:      "review.typescript",
					Name:    "TypeScript Review",
					Version: "1.0.0",
				},
				Spec: model.PluginSpec{
					Runtime: model.PluginRuntimeMCP,
					Review: &model.ReviewPluginSpec{
						Entrypoint: "review:run",
						Triggers: model.ReviewPluginTrigger{
							Events: []string{"pull_request.updated"},
						},
						Output: model.ReviewPluginOutput{Format: "findings/v1"},
					},
				},
				Source: model.PluginSource{
					Type:    model.PluginSourceNPM,
					Package: "@agentforge/review-typescript",
					Trust: &model.PluginTrustMetadata{
						Status: model.PluginTrustVerified,
					},
				},
			},
			LifecycleState: model.PluginStateEnabled,
			RuntimeHost:    model.PluginHostTSBridge,
		},
		{
			PluginManifest: model.PluginManifest{
				APIVersion: "agentforge/v1",
				Kind:       model.PluginKindTool,
				Metadata: model.PluginMetadata{
					ID:      "tool.local",
					Name:    "Local Tool",
					Version: "1.0.0",
				},
				Spec: model.PluginSpec{
					Runtime: model.PluginRuntimeMCP,
				},
				Source: model.PluginSource{
					Type: model.PluginSourceLocal,
					Path: "./plugins/tool/manifest.yaml",
					Trust: &model.PluginTrustMetadata{
						Status: model.PluginTrustUntrusted,
					},
				},
			},
			LifecycleState: model.PluginStateInstalled,
			RuntimeHost:    model.PluginHostTSBridge,
		},
	}

	for _, record := range records {
		if err := repo.Save(ctx, record); err != nil {
			t.Fatalf("save plugin record %s: %v", record.Metadata.ID, err)
		}
	}

	filtered, err := repo.List(ctx, model.PluginFilter{
		SourceType: model.PluginSourceNPM,
		TrustState: model.PluginTrustVerified,
	})
	if err != nil {
		t.Fatalf("list plugin records: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].Metadata.ID != "review.typescript" {
		t.Fatalf("filtered plugin id = %q, want review.typescript", filtered[0].Metadata.ID)
	}
}
