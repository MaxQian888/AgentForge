package service

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestReviewPlannerHelpers(t *testing.T) {
	if got := normalizeReviewDimensions(nil); !reflect.DeepEqual(got, defaultDeepReviewDimensions) {
		t.Fatalf("normalizeReviewDimensions(nil) = %#v, want default dimensions", got)
	}
	if got := normalizeReviewDimensions([]string{" logic ", "", "logic", "security"}); !reflect.DeepEqual(got, []string{"logic", "security"}) {
		t.Fatalf("normalizeReviewDimensions(dedupe) = %#v", got)
	}

	if got := deriveReviewTriggerEvent(nil); got != "review.manual" {
		t.Fatalf("deriveReviewTriggerEvent(nil) = %q, want review.manual", got)
	}
	if got := deriveReviewTriggerEvent(&model.TriggerReviewRequest{Event: " review.manual "}); got != "review.manual" {
		t.Fatalf("deriveReviewTriggerEvent(event) = %q", got)
	}
	if got := deriveReviewTriggerEvent(&model.TriggerReviewRequest{PRURL: "https://example.test/pr/1"}); got != "pull_request.updated" {
		t.Fatalf("deriveReviewTriggerEvent(pr) = %q", got)
	}
	if got := deriveReviewTriggerEvent(&model.TriggerReviewRequest{Trigger: model.ReviewTriggerLayer1}); got != "review.layer1_escalated" {
		t.Fatalf("deriveReviewTriggerEvent(layer1) = %q", got)
	}
	if got := deriveReviewTriggerEvent(&model.TriggerReviewRequest{Trigger: model.ReviewTriggerAgent}); got != "review.agent_requested" {
		t.Fatalf("deriveReviewTriggerEvent(agent) = %q", got)
	}

	req := &model.TriggerReviewRequest{
		ChangedFiles: []string{` a\src\main.ts `, "src/main.ts", " ", "b/docs/readme.md"},
	}
	if got := normalizeChangedFiles(req); !reflect.DeepEqual(got, []string{"src/main.ts", "docs/readme.md"}) {
		t.Fatalf("normalizeChangedFiles(explicit) = %#v", got)
	}

	diffReq := &model.TriggerReviewRequest{
		Diff: "diff --git a/src/app.ts b/src/app.ts\n" +
			"diff --git a/docs/guide.md b/docs/guide.md\n" +
			"diff --git a/docs/guide.md b/docs/guide.md\n",
	}
	if got := normalizeChangedFiles(diffReq); !reflect.DeepEqual(got, []string{"src/app.ts", "docs/guide.md"}) {
		t.Fatalf("normalizeChangedFiles(diff) = %#v", got)
	}

	if got := normalizeReviewPath(` a\src\foo.ts `); got != "src/foo.ts" {
		t.Fatalf("normalizeReviewPath() = %q, want src/foo.ts", got)
	}
	if got := extractChangedFilesFromDiff(""); got != nil {
		t.Fatalf("extractChangedFilesFromDiff(empty) = %#v, want nil", got)
	}
	if !matchReviewPattern("src/server/routes.go", "src/**/*.go") {
		t.Fatal("matchReviewPattern(go glob) = false, want true")
	}
	if matchesAnyReviewPattern("docs/readme.md", []string{"src/**/*.ts", "docs/**/*.md"}) != true {
		t.Fatal("matchesAnyReviewPattern() = false, want true")
	}

	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			Kind: model.PluginKindReview,
			Spec: model.PluginSpec{
				Review: &model.ReviewPluginSpec{
					Triggers: model.ReviewPluginTrigger{
						Events:       []string{"pull_request.updated"},
						FilePatterns: []string{"src/**/*.go"},
					},
				},
			},
		},
		LifecycleState: model.PluginStateEnabled,
	}
	if !matchesReviewExecutionCandidate(record, "pull_request.updated", []string{"src/server/routes.go"}) {
		t.Fatal("matchesReviewExecutionCandidate(match) = false, want true")
	}
	if matchesReviewExecutionCandidate(record, "review.manual", []string{"src/server/routes.go"}) {
		t.Fatal("matchesReviewExecutionCandidate(event mismatch) = true, want false")
	}
	if matchesReviewExecutionCandidate(record, "pull_request.updated", nil) {
		t.Fatal("matchesReviewExecutionCandidate(no changed files) = true, want false")
	}
	if !isEnabledReviewPluginState(model.PluginStateActive) || isEnabledReviewPluginState(model.PluginStateDisabled) {
		t.Fatal("isEnabledReviewPluginState() mismatch")
	}
}

func TestWorkflowExecutionHelpers(t *testing.T) {
	if got := extractTriggerEvents(nil); got != nil {
		t.Fatalf("extractTriggerEvents(nil) = %#v, want nil", got)
	}
	events := extractTriggerEvents(map[string]any{"events": []any{"build", 7, "review"}})
	if !reflect.DeepEqual(events, map[string]bool{"build": true, "review": true}) {
		t.Fatalf("extractTriggerEvents(any) = %#v", events)
	}

	defs := []model.WorkflowStepDefinition{
		{ID: "build", Config: map[string]any{"wave_number": 2}},
		{ID: "review", Metadata: map[string]any{"wave_number": float64(3)}},
		{ID: "ship"},
	}
	steps := []model.WorkflowStepRun{
		{StepID: "build"},
		{StepID: "review"},
		{StepID: "ship"},
	}
	waves := groupStepsByWave(defs, steps)
	if !reflect.DeepEqual(waves, map[int][]int{2: {0}, 3: {1}, 0: {2}}) {
		t.Fatalf("groupStepsByWave() = %#v", waves)
	}
	if readWaveNumber(defs[0]) != 2 || readWaveNumber(defs[2]) != 0 {
		t.Fatal("readWaveNumber() mismatch")
	}
	if extractIntFromMap(map[string]any{"n": int64(4)}, "n") != 4 {
		t.Fatal("extractIntFromMap(int64) mismatch")
	}
	if got := sortedWaveNumbers(waves); !reflect.DeepEqual(got, []int{0, 2, 3}) {
		t.Fatalf("sortedWaveNumbers() = %#v", got)
	}

	run := &model.WorkflowPluginRun{
		ID:       uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		PluginID: "workflow.release-train",
		Process:  model.WorkflowProcessWave,
		Trigger: map[string]any{
			"projectId": "project-1",
			"nested":    map[string]any{"labels": []string{"ops", "release"}},
		},
	}
	step := model.WorkflowStepRun{StepID: "build", RoleID: "coder"}
	roleProfile := &model.RoleExecutionProfile{
		RoleID:         "coder",
		Name:           "Coder",
		Role:           "implement",
		Goal:           "ship code",
		Backstory:      "senior engineer",
		SystemPrompt:   "do the work",
		AllowedTools:   []string{"edit", "test"},
		MaxBudgetUsd:   5,
		MaxTurns:       8,
		PermissionMode: "workspace-write",
	}
	outputs := map[string]map[string]any{"plan": {"status": "done"}}
	input := buildWorkflowStepInput(run, step, roleProfile, 2, outputs)
	roleMap, ok := input["role"].(map[string]any)
	if !ok || roleMap["role_id"] != "coder" {
		t.Fatalf("buildWorkflowStepInput(role) = %#v", input["role"])
	}
	stepsMap, ok := input["steps"].(map[string]any)
	if !ok {
		t.Fatalf("buildWorkflowStepInput(steps) = %#v", input["steps"])
	}
	nestedPlan := stepsMap["plan"].(map[string]any)
	nestedPlan["status"] = "mutated"
	if outputs["plan"]["status"] != "done" {
		t.Fatal("expected step outputs to be deep-cloned")
	}

	clone := cloneWorkflowPayload(map[string]any{
		"nested": map[string]any{"count": 2},
		"list":   []any{map[string]any{"step": "plan"}, "done"},
	})
	clone["nested"].(map[string]any)["count"] = 9
	clone["list"].([]any)[0].(map[string]any)["step"] = "mutated"
	original := map[string]any{
		"nested": map[string]any{"count": 2},
		"list":   []any{map[string]any{"step": "plan"}, "done"},
	}
	if reflect.DeepEqual(clone, original) {
		t.Fatal("cloneWorkflowPayload() should allow independent mutation")
	}
}

func TestWorkflowStepRouterHelpers(t *testing.T) {
	trigger := map[string]any{
		"taskId":     uuid.MustParse("66666666-7777-8888-9999-aaaaaaaaaaaa").String(),
		"budgetUsd":  "4.5",
		"priority":   "3",
		"dimensions": []any{"logic", 7, "security"},
	}

	if got, err := workflowInputMap(map[string]any{"trigger": trigger}, "trigger"); err != nil || !reflect.DeepEqual(got, trigger) {
		t.Fatalf("workflowInputMap() = %#v, %v", got, err)
	}
	if _, err := workflowInputMap(nil, "trigger"); err == nil {
		t.Fatal("workflowInputMap(nil) expected error")
	}
	if _, err := workflowInputMap(map[string]any{"trigger": "bad"}, "trigger"); err == nil {
		t.Fatal("workflowInputMap(non-object) expected error")
	}

	if _, err := workflowUUID(trigger, "taskId"); err != nil {
		t.Fatalf("workflowUUID(valid) error = %v", err)
	}
	if _, err := workflowUUID(trigger, "missing"); err == nil {
		t.Fatal("workflowUUID(missing) expected error")
	}
	if got := workflowString(trigger, "taskId"); got == "" {
		t.Fatal("workflowString(taskId) should not be empty")
	}
	if got := workflowFloat(trigger, "budgetUsd"); got != 4.5 {
		t.Fatalf("workflowFloat(budgetUsd) = %v, want 4.5", got)
	}
	if got := workflowInt(trigger, "priority"); got != 3 {
		t.Fatalf("workflowInt(priority) = %d, want 3", got)
	}
	if got := workflowStringSlice(trigger, "dimensions"); !reflect.DeepEqual(got, []string{"logic", "security"}) {
		t.Fatalf("workflowStringSlice(dimensions) = %#v", got)
	}
	if got := workflowStringSlice(map[string]any{"dimensions": "bad"}, "dimensions"); got != nil {
		t.Fatalf("workflowStringSlice(non-array) = %#v, want nil", got)
	}
}
