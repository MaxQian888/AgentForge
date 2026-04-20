package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

type fakeViewExecRepo struct {
	execs []*model.WorkflowExecution
}

func (f *fakeViewExecRepo) ListByProjectFiltered(_ context.Context, projectID uuid.UUID, filter repository.WorkflowExecutionListFilter, limit int) ([]*model.WorkflowExecution, error) {
	statusSet := map[string]struct{}{}
	for _, s := range filter.Statuses {
		statusSet[s] = struct{}{}
	}
	out := make([]*model.WorkflowExecution, 0)
	for _, e := range f.execs {
		if e.ProjectID != projectID {
			continue
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[e.Status]; !ok {
				continue
			}
		}
		if filter.ActingEmployeeID != nil {
			if e.ActingEmployeeID == nil || *e.ActingEmployeeID != *filter.ActingEmployeeID {
				continue
			}
		}
		if filter.TriggerID != nil {
			if e.TriggeredBy == nil || *e.TriggeredBy != *filter.TriggerID {
				continue
			}
		}
		if filter.TriggeredByKind == "trigger" && e.TriggeredBy == nil {
			continue
		}
		if filter.TriggeredByKind == "manual" && e.TriggeredBy != nil {
			continue
		}
		if filter.StartedAfter != nil {
			if e.StartedAt == nil || !e.StartedAt.After(*filter.StartedAfter) {
				continue
			}
		}
		if filter.StartedBefore != nil {
			if e.StartedAt == nil || !e.StartedAt.Before(*filter.StartedBefore) {
				continue
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		ti := timeOrZero(out[i].StartedAt)
		tj := timeOrZero(out[j].StartedAt)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeViewExecRepo) GetExecution(_ context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	for _, e := range f.execs {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, repository.ErrNotFound
}

type fakeViewPluginRepo struct {
	runs []*model.WorkflowPluginRun
}

func (f *fakeViewPluginRepo) ListByProject(_ context.Context, projectID uuid.UUID, filter repository.WorkflowPluginRunListFilter, limit int) ([]*model.WorkflowPluginRun, error) {
	statusSet := map[model.WorkflowRunStatus]struct{}{}
	for _, s := range filter.Statuses {
		statusSet[s] = struct{}{}
	}
	out := make([]*model.WorkflowPluginRun, 0)
	for _, r := range f.runs {
		if r.ProjectID != projectID {
			continue
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[r.Status]; !ok {
				continue
			}
		}
		if filter.ActingEmployeeID != nil {
			if r.ActingEmployeeID == nil || *r.ActingEmployeeID != *filter.ActingEmployeeID {
				continue
			}
		}
		if filter.TriggerID != nil {
			if r.TriggerID == nil || *r.TriggerID != *filter.TriggerID {
				continue
			}
		}
		if filter.TriggeredByKind == "trigger" && r.TriggerID == nil {
			continue
		}
		if filter.TriggeredByKind == "manual" && r.TriggerID != nil {
			continue
		}
		if filter.StartedAfter != nil && !r.StartedAt.After(*filter.StartedAfter) {
			continue
		}
		if filter.StartedBefore != nil && !r.StartedAt.Before(*filter.StartedBefore) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].StartedAt.After(out[j].StartedAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeViewPluginRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowPluginRun, error) {
	for _, r := range f.runs {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, repository.ErrNotFound
}

type fakeViewDefRepo struct {
	defs map[uuid.UUID]*model.WorkflowDefinition
}

func (f *fakeViewDefRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if d, ok := f.defs[id]; ok {
		return d, nil
	}
	return nil, repository.ErrNotFound
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func viewPtrTime(t time.Time) *time.Time { return &t }
func viewPtrUUID(u uuid.UUID) *uuid.UUID { return &u }

// --- Tests ---

func TestWorkflowRunViewService_ListRuns_DAGOnly(t *testing.T) {
	projectID := uuid.New()
	workflowID := uuid.New()
	execID := uuid.New()
	started := time.Now().UTC().Add(-time.Minute)
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{{
		ID:         execID,
		WorkflowID: workflowID,
		ProjectID:  projectID,
		Status:     model.WorkflowExecStatusRunning,
		StartedAt:  viewPtrTime(started),
	}}}
	pluginRepo := &fakeViewPluginRepo{}
	defRepo := &fakeViewDefRepo{defs: map[uuid.UUID]*model.WorkflowDefinition{
		workflowID: {ID: workflowID, Name: "Sample DAG"},
	}}
	svc := NewWorkflowRunViewService(execRepo, pluginRepo, defRepo)

	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, "", 20)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	row := result.Rows[0]
	if row.Engine != UnifiedRunEngineDAG {
		t.Errorf("engine = %q, want %q", row.Engine, UnifiedRunEngineDAG)
	}
	if row.Status != UnifiedRunStatusRunning {
		t.Errorf("status = %q, want running", row.Status)
	}
	if row.WorkflowRef.Name != "Sample DAG" {
		t.Errorf("workflow name = %q", row.WorkflowRef.Name)
	}
	if result.Summary.Running != 1 {
		t.Errorf("summary.Running = %d, want 1", result.Summary.Running)
	}
}

func TestWorkflowRunViewService_ListRuns_PluginOnly(t *testing.T) {
	projectID := uuid.New()
	pluginRepo := &fakeViewPluginRepo{runs: []*model.WorkflowPluginRun{{
		ID:        uuid.New(),
		PluginID:  "my-plugin",
		ProjectID: projectID,
		Status:    model.WorkflowRunStatusCompleted,
		StartedAt: time.Now().UTC().Add(-time.Hour),
	}}}
	svc := NewWorkflowRunViewService(&fakeViewExecRepo{}, pluginRepo, &fakeViewDefRepo{})

	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, "", 20)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Engine != UnifiedRunEnginePlugin {
		t.Errorf("engine = %q", result.Rows[0].Engine)
	}
	if result.Rows[0].Status != UnifiedRunStatusCompleted {
		t.Errorf("status = %q", result.Rows[0].Status)
	}
}

func TestWorkflowRunViewService_ListRuns_MixedSortedDesc(t *testing.T) {
	projectID := uuid.New()
	now := time.Now().UTC()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{{
		ID:         uuid.New(),
		WorkflowID: uuid.New(),
		ProjectID:  projectID,
		Status:     model.WorkflowExecStatusRunning,
		StartedAt:  viewPtrTime(now.Add(-30 * time.Minute)),
	}}}
	pluginRepo := &fakeViewPluginRepo{runs: []*model.WorkflowPluginRun{{
		ID:        uuid.New(),
		PluginID:  "plugin-a",
		ProjectID: projectID,
		Status:    model.WorkflowRunStatusRunning,
		StartedAt: now.Add(-5 * time.Minute),
	}}}
	svc := NewWorkflowRunViewService(execRepo, pluginRepo, &fakeViewDefRepo{})
	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, "", 20)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	if result.Rows[0].Engine != UnifiedRunEnginePlugin {
		t.Errorf("row[0] should be plugin (newer), got %s", result.Rows[0].Engine)
	}
	if result.Rows[1].Engine != UnifiedRunEngineDAG {
		t.Errorf("row[1] should be dag, got %s", result.Rows[1].Engine)
	}
}

func TestWorkflowRunViewService_ListRuns_EngineFilter(t *testing.T) {
	projectID := uuid.New()
	now := time.Now().UTC()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{{
		ID:         uuid.New(),
		WorkflowID: uuid.New(),
		ProjectID:  projectID,
		Status:     model.WorkflowExecStatusRunning,
		StartedAt:  viewPtrTime(now),
	}}}
	pluginRepo := &fakeViewPluginRepo{runs: []*model.WorkflowPluginRun{{
		ID:        uuid.New(),
		PluginID:  "plugin-a",
		ProjectID: projectID,
		Status:    model.WorkflowRunStatusRunning,
		StartedAt: now,
	}}}
	svc := NewWorkflowRunViewService(execRepo, pluginRepo, &fakeViewDefRepo{})

	dagOnly, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{Engine: UnifiedRunEngineDAG}, "", 20)
	if err != nil {
		t.Fatalf("dag filter error: %v", err)
	}
	if len(dagOnly.Rows) != 1 || dagOnly.Rows[0].Engine != UnifiedRunEngineDAG {
		t.Errorf("expected 1 dag row, got %+v", dagOnly.Rows)
	}
	pluginOnly, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{Engine: UnifiedRunEnginePlugin}, "", 20)
	if err != nil {
		t.Fatalf("plugin filter error: %v", err)
	}
	if len(pluginOnly.Rows) != 1 || pluginOnly.Rows[0].Engine != UnifiedRunEnginePlugin {
		t.Errorf("expected 1 plugin row, got %+v", pluginOnly.Rows)
	}
}

func TestWorkflowRunViewService_ListRuns_StatusFilter(t *testing.T) {
	projectID := uuid.New()
	now := time.Now().UTC()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(now)},
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusFailed, StartedAt: viewPtrTime(now.Add(-time.Minute))},
	}}
	svc := NewWorkflowRunViewService(execRepo, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{Statuses: []string{UnifiedRunStatusRunning}}, "", 20)
	if err != nil {
		t.Fatalf("status filter error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Status != UnifiedRunStatusRunning {
		t.Errorf("status = %q", result.Rows[0].Status)
	}
}

func TestWorkflowRunViewService_ListRuns_ActingEmployeeFilter(t *testing.T) {
	projectID := uuid.New()
	employeeID := uuid.New()
	now := time.Now().UTC()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(now), ActingEmployeeID: viewPtrUUID(employeeID)},
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(now.Add(-time.Minute))},
	}}
	pluginRepo := &fakeViewPluginRepo{runs: []*model.WorkflowPluginRun{
		{ID: uuid.New(), PluginID: "p1", ProjectID: projectID, Status: model.WorkflowRunStatusRunning, StartedAt: now, ActingEmployeeID: viewPtrUUID(employeeID)},
	}}
	svc := NewWorkflowRunViewService(execRepo, pluginRepo, &fakeViewDefRepo{})

	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{ActingEmployeeID: viewPtrUUID(employeeID)}, "", 20)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows (one per engine), got %d", len(result.Rows))
	}
	for _, r := range result.Rows {
		if r.ActingEmployeeID != employeeID.String() {
			t.Errorf("row acting employee = %q", r.ActingEmployeeID)
		}
	}
}

func TestWorkflowRunViewService_ListRuns_TriggerIDFilter(t *testing.T) {
	projectID := uuid.New()
	triggerID := uuid.New()
	now := time.Now().UTC()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(now), TriggeredBy: viewPtrUUID(triggerID)},
	}}
	pluginRepo := &fakeViewPluginRepo{runs: []*model.WorkflowPluginRun{
		{ID: uuid.New(), PluginID: "p1", ProjectID: projectID, Status: model.WorkflowRunStatusRunning, StartedAt: now, TriggerID: viewPtrUUID(triggerID)},
		{ID: uuid.New(), PluginID: "p2", ProjectID: projectID, Status: model.WorkflowRunStatusRunning, StartedAt: now},
	}}
	svc := NewWorkflowRunViewService(execRepo, pluginRepo, &fakeViewDefRepo{})

	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{TriggerID: viewPtrUUID(triggerID), TriggeredByKind: "trigger"}, "", 20)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
}

func TestWorkflowRunViewService_ListRuns_StartedAfterBefore(t *testing.T) {
	projectID := uuid.New()
	base := time.Now().UTC().Truncate(time.Second)
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusCompleted, StartedAt: viewPtrTime(base.Add(-2 * time.Hour))},
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusCompleted, StartedAt: viewPtrTime(base.Add(-30 * time.Minute))},
		{ID: uuid.New(), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(base)},
	}}
	svc := NewWorkflowRunViewService(execRepo, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	after := base.Add(-1 * time.Hour)
	before := base.Add(-1 * time.Minute)
	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{StartedAfter: &after, StartedBefore: &before}, "", 20)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestWorkflowRunViewService_ListRuns_EmptyResult(t *testing.T) {
	projectID := uuid.New()
	svc := NewWorkflowRunViewService(&fakeViewExecRepo{}, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	result, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, "", 20)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected empty, got %d", len(result.Rows))
	}
	if result.NextCursor != "" {
		t.Errorf("nextCursor should be empty")
	}
}

func TestWorkflowRunViewService_ListRuns_Paginated(t *testing.T) {
	projectID := uuid.New()
	now := time.Now().UTC()
	execs := make([]*model.WorkflowExecution, 5)
	for i := 0; i < 5; i++ {
		execs[i] = &model.WorkflowExecution{
			ID:         uuid.New(),
			WorkflowID: uuid.New(),
			ProjectID:  projectID,
			Status:     model.WorkflowExecStatusRunning,
			StartedAt:  viewPtrTime(now.Add(-time.Duration(i) * time.Minute)),
		}
	}
	execRepo := &fakeViewExecRepo{execs: execs}
	svc := NewWorkflowRunViewService(execRepo, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	page1, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, "", 2)
	if err != nil {
		t.Fatalf("page1 error: %v", err)
	}
	if len(page1.Rows) != 2 {
		t.Fatalf("page1 expected 2 rows, got %d", len(page1.Rows))
	}
	if page1.NextCursor == "" {
		t.Fatalf("page1 expected nextCursor")
	}

	page2, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, page1.NextCursor, 2)
	if err != nil {
		t.Fatalf("page2 error: %v", err)
	}
	if len(page2.Rows) != 2 {
		t.Fatalf("page2 expected 2 rows, got %d", len(page2.Rows))
	}
	// Verify no duplicates between pages
	page1IDs := map[string]bool{}
	for _, r := range page1.Rows {
		page1IDs[r.RunID] = true
	}
	for _, r := range page2.Rows {
		if page1IDs[r.RunID] {
			t.Errorf("row %s appears on both pages", r.RunID)
		}
	}
}

func TestWorkflowRunViewService_ListRuns_CursorStableUnderConcurrentInsert(t *testing.T) {
	projectID := uuid.New()
	base := time.Now().UTC().Truncate(time.Millisecond)
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{
		{ID: mustParseUUID("11111111-1111-1111-1111-111111111111"), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(base.Add(-3 * time.Second))},
		{ID: mustParseUUID("22222222-2222-2222-2222-222222222222"), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(base.Add(-2 * time.Second))},
		{ID: mustParseUUID("33333333-3333-3333-3333-333333333333"), WorkflowID: uuid.New(), ProjectID: projectID, Status: model.WorkflowExecStatusRunning, StartedAt: viewPtrTime(base.Add(-1 * time.Second))},
	}}
	svc := NewWorkflowRunViewService(execRepo, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	page1, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, "", 2)
	if err != nil || page1.NextCursor == "" {
		t.Fatalf("page1 setup failed: %v", err)
	}

	// Simulate a concurrent insert of a newer row between page fetches.
	execRepo.execs = append(execRepo.execs, &model.WorkflowExecution{
		ID:         mustParseUUID("44444444-4444-4444-4444-444444444444"),
		WorkflowID: uuid.New(),
		ProjectID:  projectID,
		Status:     model.WorkflowExecStatusRunning,
		StartedAt:  viewPtrTime(base),
	})

	page2, err := svc.ListRuns(context.Background(), projectID, UnifiedRunListFilter{}, page1.NextCursor, 2)
	if err != nil {
		t.Fatalf("page2 error: %v", err)
	}

	// Page 2 must not contain any row from page 1, even though the result set
	// grew between fetches. Cursor keyed on (StartedAt, RunID) keeps the
	// pagination stable under concurrent insertion.
	page1IDs := map[string]bool{}
	for _, r := range page1.Rows {
		page1IDs[r.RunID] = true
	}
	for _, r := range page2.Rows {
		if page1IDs[r.RunID] {
			t.Errorf("row %s appears on both pages after concurrent insert", r.RunID)
		}
	}
}

func TestWorkflowRunViewService_GetRun_DAG(t *testing.T) {
	projectID := uuid.New()
	workflowID := uuid.New()
	execID := uuid.New()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{{
		ID:         execID,
		WorkflowID: workflowID,
		ProjectID:  projectID,
		Status:     model.WorkflowExecStatusRunning,
		StartedAt:  viewPtrTime(time.Now().UTC()),
	}}}
	svc := NewWorkflowRunViewService(execRepo, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	detail, err := svc.GetRun(context.Background(), projectID, UnifiedRunEngineDAG, execID)
	if err != nil {
		t.Fatalf("GetRun error: %v", err)
	}
	if detail.Row.Engine != UnifiedRunEngineDAG {
		t.Errorf("engine = %q", detail.Row.Engine)
	}
	body, ok := detail.Body.(map[string]any)
	if !ok {
		t.Fatalf("body type = %T", detail.Body)
	}
	if _, ok := body["execution"]; !ok {
		t.Error("body.execution missing")
	}
}

func TestWorkflowRunViewService_GetRun_Plugin(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	pluginRepo := &fakeViewPluginRepo{runs: []*model.WorkflowPluginRun{{
		ID:        runID,
		PluginID:  "p1",
		ProjectID: projectID,
		Status:    model.WorkflowRunStatusRunning,
		StartedAt: time.Now().UTC(),
	}}}
	svc := NewWorkflowRunViewService(&fakeViewExecRepo{}, pluginRepo, &fakeViewDefRepo{})

	detail, err := svc.GetRun(context.Background(), projectID, UnifiedRunEnginePlugin, runID)
	if err != nil {
		t.Fatalf("GetRun error: %v", err)
	}
	if detail.Row.Engine != UnifiedRunEnginePlugin {
		t.Errorf("engine = %q", detail.Row.Engine)
	}
}

func TestWorkflowRunViewService_GetRun_UnknownEngine(t *testing.T) {
	svc := NewWorkflowRunViewService(&fakeViewExecRepo{}, &fakeViewPluginRepo{}, &fakeViewDefRepo{})
	_, err := svc.GetRun(context.Background(), uuid.New(), "mystery", uuid.New())
	if err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestWorkflowRunViewService_GetRun_WrongProject(t *testing.T) {
	realProject := uuid.New()
	otherProject := uuid.New()
	execID := uuid.New()
	execRepo := &fakeViewExecRepo{execs: []*model.WorkflowExecution{{
		ID:         execID,
		WorkflowID: uuid.New(),
		ProjectID:  realProject,
		Status:     model.WorkflowExecStatusRunning,
	}}}
	svc := NewWorkflowRunViewService(execRepo, &fakeViewPluginRepo{}, &fakeViewDefRepo{})

	_, err := svc.GetRun(context.Background(), otherProject, UnifiedRunEngineDAG, execID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestNormalizeStatuses(t *testing.T) {
	cases := map[string]string{
		"pending":   UnifiedRunStatusPending,
		"running":   UnifiedRunStatusRunning,
		"paused":    UnifiedRunStatusPaused,
		"completed": UnifiedRunStatusCompleted,
		"failed":    UnifiedRunStatusFailed,
		"cancelled": UnifiedRunStatusCancelled,
		"gibberish": UnifiedRunStatusUnknown,
	}
	for in, want := range cases {
		if got := normalizeDAGExecutionStatus(in); got != want {
			t.Errorf("normalizeDAGExecutionStatus(%q) = %q, want %q", in, got, want)
		}
	}
	if got := normalizePluginRunStatus(model.WorkflowRunStatus("gibberish")); got != UnifiedRunStatusUnknown {
		t.Errorf("unknown plugin status should be %q, got %q", UnifiedRunStatusUnknown, got)
	}
}

func mustParseUUID(s string) uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
