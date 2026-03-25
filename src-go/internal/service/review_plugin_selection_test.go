package service_test

import (
	"context"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type reviewPluginCatalogStub struct {
	records []*model.PluginRecord
}

func (s reviewPluginCatalogStub) List(_ context.Context, filter service.PluginListFilter) ([]*model.PluginRecord, error) {
	if filter.Kind == "" {
		return s.records, nil
	}
	filtered := make([]*model.PluginRecord, 0, len(s.records))
	for _, record := range s.records {
		if record.Kind == filter.Kind {
			filtered = append(filtered, record)
		}
	}
	return filtered, nil
}

func TestReviewExecutionPlanner_BuildPlanSelectsMatchingPlugins(t *testing.T) {
	t.Parallel()

	planner := service.NewReviewExecutionPlanner(reviewPluginCatalogStub{
		records: []*model.PluginRecord{
			{
				PluginManifest: model.PluginManifest{
					Kind: model.PluginKindReview,
					Metadata: model.PluginMetadata{
						ID:   "review.typescript",
						Name: "TypeScript Review",
					},
					Spec: model.PluginSpec{
						Runtime: model.PluginRuntimeMCP,
						Review: &model.ReviewPluginSpec{
							Entrypoint: "review:run",
							Triggers: model.ReviewPluginTrigger{
								Events:       []string{"pull_request.updated"},
								FilePatterns: []string{"src/**/*.ts"},
							},
							Output: model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
					Source: model.PluginSource{Type: model.PluginSourceNPM},
				},
				LifecycleState: model.PluginStateEnabled,
			},
			{
				PluginManifest: model.PluginManifest{
					Kind: model.PluginKindReview,
					Metadata: model.PluginMetadata{
						ID:   "review.docs",
						Name: "Docs Review",
					},
					Spec: model.PluginSpec{
						Runtime: model.PluginRuntimeMCP,
						Review: &model.ReviewPluginSpec{
							Triggers: model.ReviewPluginTrigger{
								Events:       []string{"pull_request.updated"},
								FilePatterns: []string{"docs/**/*.md"},
							},
							Output: model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
				},
				LifecycleState: model.PluginStateEnabled,
			},
		},
	})

	plan, err := planner.BuildPlan(context.Background(), &model.TriggerReviewRequest{
		PRURL:      "https://github.com/acme/project/pull/42",
		Trigger:    model.ReviewTriggerManual,
		Dimensions: []string{"logic", "security"},
		Diff: "diff --git a/src/app/review.ts b/src/app/review.ts\n" +
			"index 123..456 100644\n" +
			"--- a/src/app/review.ts\n" +
			"+++ b/src/app/review.ts\n",
	})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	if plan.TriggerEvent != "pull_request.updated" {
		t.Fatalf("TriggerEvent = %q, want pull_request.updated", plan.TriggerEvent)
	}
	if len(plan.ChangedFiles) != 1 || plan.ChangedFiles[0] != "src/app/review.ts" {
		t.Fatalf("ChangedFiles = %#v, want [src/app/review.ts]", plan.ChangedFiles)
	}
	if len(plan.Plugins) != 1 || plan.Plugins[0].ID != "review.typescript" {
		t.Fatalf("Plugins = %#v, want only review.typescript", plan.Plugins)
	}
}

func TestReviewExecutionPlanner_BuildPlanSkipsDisabledAndNonMatchingPlugins(t *testing.T) {
	t.Parallel()

	planner := service.NewReviewExecutionPlanner(reviewPluginCatalogStub{
		records: []*model.PluginRecord{
			{
				PluginManifest: model.PluginManifest{
					Kind:     model.PluginKindReview,
					Metadata: model.PluginMetadata{ID: "review.disabled", Name: "Disabled Review"},
					Spec: model.PluginSpec{
						Review: &model.ReviewPluginSpec{
							Triggers: model.ReviewPluginTrigger{Events: []string{"pull_request.updated"}},
							Output:   model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
				},
				LifecycleState: model.PluginStateDisabled,
			},
			{
				PluginManifest: model.PluginManifest{
					Kind:     model.PluginKindReview,
					Metadata: model.PluginMetadata{ID: "review.manual", Name: "Manual Review"},
					Spec: model.PluginSpec{
						Review: &model.ReviewPluginSpec{
							Triggers: model.ReviewPluginTrigger{
								Events:       []string{"review.manual"},
								FilePatterns: []string{"docs/**/*.md"},
							},
							Output: model.ReviewPluginOutput{Format: "findings/v1"},
						},
					},
				},
				LifecycleState: model.PluginStateEnabled,
			},
		},
	})

	plan, err := planner.BuildPlan(context.Background(), &model.TriggerReviewRequest{
		Event:        "pull_request.updated",
		ChangedFiles: []string{"src/server/routes.go"},
	})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}

	if len(plan.Plugins) != 0 {
		t.Fatalf("Plugins = %#v, want none", plan.Plugins)
	}
}
