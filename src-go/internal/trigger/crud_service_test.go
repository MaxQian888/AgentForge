package trigger_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/trigger"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockCRUDRepo struct {
	rows         map[uuid.UUID]*model.WorkflowTrigger
	createErr    error
	updateErr    error
	deleteErr    error
	getErr       error
	listByEmpErr error
}

func newMockCRUDRepo() *mockCRUDRepo {
	return &mockCRUDRepo{rows: map[uuid.UUID]*model.WorkflowTrigger{}}
}

func (m *mockCRUDRepo) Create(_ context.Context, t *model.WorkflowTrigger) error {
	if m.createErr != nil {
		return m.createErr
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	cp := *t
	m.rows[t.ID] = &cp
	return nil
}

func (m *mockCRUDRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowTrigger, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	row, ok := m.rows[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *row
	return &cp, nil
}

func (m *mockCRUDRepo) Update(_ context.Context, t *model.WorkflowTrigger) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.rows[t.ID]; !ok {
		return repository.ErrNotFound
	}
	cp := *t
	m.rows[t.ID] = &cp
	return nil
}

func (m *mockCRUDRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.rows[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.rows, id)
	return nil
}

func (m *mockCRUDRepo) ListByActingEmployee(_ context.Context, employeeID uuid.UUID) ([]*model.WorkflowTrigger, error) {
	if m.listByEmpErr != nil {
		return nil, m.listByEmpErr
	}
	out := []*model.WorkflowTrigger{}
	for _, r := range m.rows {
		if r.ActingEmployeeID != nil && *r.ActingEmployeeID == employeeID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

type mockDefLookup struct {
	defs map[uuid.UUID]*model.WorkflowDefinition
}

func (m *mockDefLookup) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if d, ok := m.defs[id]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}

type mockEmpLookup struct {
	emps map[uuid.UUID]*model.Employee
}

func (m *mockEmpLookup) Get(_ context.Context, id uuid.UUID) (*model.Employee, error) {
	if e, ok := m.emps[id]; ok {
		return e, nil
	}
	return nil, repository.ErrNotFound
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestTriggerService_Create_HappyPath(t *testing.T) {
	wfID := uuid.New()
	projID := uuid.New()
	defs := &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{
		wfID: {ID: wfID, ProjectID: projID, Status: model.WorkflowDefStatusActive},
	}}
	emps := &mockEmpLookup{emps: map[uuid.UUID]*model.Employee{}}
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, defs, emps)

	got, err := svc.Create(context.Background(), trigger.CreateTriggerInput{
		WorkflowID:   wfID,
		Source:       model.TriggerSourceIM,
		Config:       json.RawMessage(`{"platform":"feishu","command":"/echo"}`),
		InputMapping: json.RawMessage(`{}`),
		DisplayName:  "echo",
		Description:  "test row",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.CreatedVia != model.TriggerCreatedViaManual {
		t.Errorf("CreatedVia: want manual, got %q", got.CreatedVia)
	}
	if got.ProjectID != projID {
		t.Errorf("ProjectID: want %s, got %s", projID, got.ProjectID)
	}
	if !got.Enabled {
		t.Errorf("expected Enabled=true on new trigger")
	}
}

func TestTriggerService_Create_WorkflowNotFound(t *testing.T) {
	defs := &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{}}
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, defs, nil)

	_, err := svc.Create(context.Background(), trigger.CreateTriggerInput{
		WorkflowID: uuid.New(),
		Source:     model.TriggerSourceIM,
		Config:     json.RawMessage(`{}`),
	})
	if !errors.Is(err, trigger.ErrTriggerWorkflowNotFound) {
		t.Errorf("expected ErrTriggerWorkflowNotFound, got %v", err)
	}
}

func TestTriggerService_Create_ActingEmployeeArchived(t *testing.T) {
	wfID := uuid.New()
	projID := uuid.New()
	empID := uuid.New()
	defs := &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{
		wfID: {ID: wfID, ProjectID: projID},
	}}
	emps := &mockEmpLookup{emps: map[uuid.UUID]*model.Employee{
		empID: {ID: empID, ProjectID: projID, State: model.EmployeeStateArchived},
	}}
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, defs, emps)

	_, err := svc.Create(context.Background(), trigger.CreateTriggerInput{
		WorkflowID:       wfID,
		Source:           model.TriggerSourceIM,
		Config:           json.RawMessage(`{}`),
		ActingEmployeeID: &empID,
	})
	if !errors.Is(err, trigger.ErrTriggerActingEmployeeArchived) {
		t.Errorf("expected ErrTriggerActingEmployeeArchived, got %v", err)
	}
}

func TestTriggerService_Create_ActingEmployeeCrossProject(t *testing.T) {
	wfID := uuid.New()
	projID := uuid.New()
	otherProj := uuid.New()
	empID := uuid.New()
	defs := &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{
		wfID: {ID: wfID, ProjectID: projID},
	}}
	emps := &mockEmpLookup{emps: map[uuid.UUID]*model.Employee{
		empID: {ID: empID, ProjectID: otherProj, State: model.EmployeeStateActive},
	}}
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, defs, emps)

	_, err := svc.Create(context.Background(), trigger.CreateTriggerInput{
		WorkflowID:       wfID,
		Source:           model.TriggerSourceIM,
		Config:           json.RawMessage(`{}`),
		ActingEmployeeID: &empID,
	})
	if !errors.Is(err, trigger.ErrTriggerActingEmployeeArchived) {
		t.Errorf("expected ErrTriggerActingEmployeeArchived (single sentinel for invalid emp), got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Patch
// ---------------------------------------------------------------------------

func TestTriggerService_Patch_UpdatesDisplayName(t *testing.T) {
	wfID := uuid.New()
	projID := uuid.New()
	defs := &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{
		wfID: {ID: wfID, ProjectID: projID},
	}}
	repo := newMockCRUDRepo()
	id := uuid.New()
	wfRef := wfID
	repo.rows[id] = &model.WorkflowTrigger{
		ID: id, WorkflowID: &wfRef, ProjectID: projID,
		Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
		Config: json.RawMessage(`{}`), CreatedVia: model.TriggerCreatedViaManual,
		DisplayName: "old", Enabled: true,
	}
	svc := trigger.NewCRUDService(repo, defs, nil)

	newName := "renamed"
	got, err := svc.Patch(context.Background(), id, trigger.PatchTriggerInput{
		DisplayName: &newName,
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if got.DisplayName != "renamed" {
		t.Errorf("DisplayName: want renamed, got %q", got.DisplayName)
	}
	if repo.rows[id].DisplayName != "renamed" {
		t.Errorf("repo row not updated; got %q", repo.rows[id].DisplayName)
	}
}

func TestTriggerService_Patch_ActingEmployeeRevalidated(t *testing.T) {
	wfID := uuid.New()
	projID := uuid.New()
	otherProj := uuid.New()
	empID := uuid.New()
	defs := &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{
		wfID: {ID: wfID, ProjectID: projID},
	}}
	emps := &mockEmpLookup{emps: map[uuid.UUID]*model.Employee{
		empID: {ID: empID, ProjectID: otherProj, State: model.EmployeeStateActive},
	}}
	repo := newMockCRUDRepo()
	id := uuid.New()
	wfRef := wfID
	repo.rows[id] = &model.WorkflowTrigger{
		ID: id, WorkflowID: &wfRef, ProjectID: projID,
		Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
		Config: json.RawMessage(`{}`), CreatedVia: model.TriggerCreatedViaManual,
	}
	svc := trigger.NewCRUDService(repo, defs, emps)

	_, err := svc.Patch(context.Background(), id, trigger.PatchTriggerInput{
		IncludeActingEmployeeID: true,
		ActingEmployeeID:        &empID,
	})
	if !errors.Is(err, trigger.ErrTriggerActingEmployeeArchived) {
		t.Errorf("expected ErrTriggerActingEmployeeArchived, got %v", err)
	}
}

func TestTriggerService_Patch_NotFound(t *testing.T) {
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, &mockDefLookup{defs: map[uuid.UUID]*model.WorkflowDefinition{}}, nil)
	_, err := svc.Patch(context.Background(), uuid.New(), trigger.PatchTriggerInput{})
	if !errors.Is(err, trigger.ErrTriggerNotFound) {
		t.Errorf("expected ErrTriggerNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestTriggerService_Delete_ManualSucceeds(t *testing.T) {
	repo := newMockCRUDRepo()
	id := uuid.New()
	repo.rows[id] = &model.WorkflowTrigger{
		ID: id, CreatedVia: model.TriggerCreatedViaManual,
		Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
	}
	svc := trigger.NewCRUDService(repo, nil, nil)

	if err := svc.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := repo.rows[id]; ok {
		t.Error("row not removed from store")
	}
}

func TestTriggerService_Delete_DAGManagedRefused(t *testing.T) {
	repo := newMockCRUDRepo()
	id := uuid.New()
	repo.rows[id] = &model.WorkflowTrigger{
		ID: id, CreatedVia: model.TriggerCreatedViaDAGNode,
		Source: model.TriggerSourceIM, TargetKind: model.TriggerTargetDAG,
	}
	svc := trigger.NewCRUDService(repo, nil, nil)

	err := svc.Delete(context.Background(), id)
	if !errors.Is(err, trigger.ErrTriggerCannotDeleteDAGManaged) {
		t.Errorf("expected ErrTriggerCannotDeleteDAGManaged, got %v", err)
	}
	if _, ok := repo.rows[id]; !ok {
		t.Error("row should not have been deleted")
	}
}

func TestTriggerService_Delete_NotFound(t *testing.T) {
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, nil, nil)
	err := svc.Delete(context.Background(), uuid.New())
	if !errors.Is(err, trigger.ErrTriggerNotFound) {
		t.Errorf("expected ErrTriggerNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListByEmployee + Test (dry-run)
// ---------------------------------------------------------------------------

func TestTriggerService_ListByEmployee(t *testing.T) {
	repo := newMockCRUDRepo()
	empID := uuid.New()
	otherEmp := uuid.New()
	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()
	emp := empID
	other := otherEmp
	repo.rows[id1] = &model.WorkflowTrigger{ID: id1, ActingEmployeeID: &emp}
	repo.rows[id2] = &model.WorkflowTrigger{ID: id2, ActingEmployeeID: &emp}
	repo.rows[id3] = &model.WorkflowTrigger{ID: id3, ActingEmployeeID: &other}
	svc := trigger.NewCRUDService(repo, nil, nil)

	got, err := svc.ListByEmployee(context.Background(), empID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 rows, got %d", len(got))
	}
}

func TestTriggerService_Test_MatchesAndDispatches(t *testing.T) {
	repo := newMockCRUDRepo()
	id := uuid.New()
	repo.rows[id] = &model.WorkflowTrigger{
		ID:           id,
		Source:       model.TriggerSourceIM,
		Config:       json.RawMessage(`{"platform":"feishu","command":"/echo"}`),
		InputMapping: json.RawMessage(`{"text":"{{$event.content}}"}`),
		Enabled:      true,
	}
	svc := trigger.NewCRUDService(repo, nil, nil)

	res, err := svc.Test(context.Background(), id, map[string]any{
		"platform": "feishu",
		"command":  "/echo",
		"content":  "/echo hi",
	})
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if !res.Matched || !res.WouldDispatch {
		t.Errorf("expected matched+would_dispatch, got %+v", res)
	}
	if res.RenderedInput["text"] != "/echo hi" {
		t.Errorf("RenderedInput.text: got %v, want /echo hi", res.RenderedInput["text"])
	}
}

func TestTriggerService_Test_NoMatch(t *testing.T) {
	repo := newMockCRUDRepo()
	id := uuid.New()
	repo.rows[id] = &model.WorkflowTrigger{
		ID:      id,
		Source:  model.TriggerSourceIM,
		Config:  json.RawMessage(`{"platform":"feishu","command":"/echo"}`),
		Enabled: true,
	}
	svc := trigger.NewCRUDService(repo, nil, nil)

	res, err := svc.Test(context.Background(), id, map[string]any{
		"platform": "feishu",
		"command":  "/other",
	})
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if res.Matched || res.WouldDispatch {
		t.Errorf("expected no match, got %+v", res)
	}
	if res.SkipReason != "no_match" {
		t.Errorf("SkipReason: want no_match, got %q", res.SkipReason)
	}
}

func TestTriggerService_Test_DisabledTrigger(t *testing.T) {
	repo := newMockCRUDRepo()
	id := uuid.New()
	repo.rows[id] = &model.WorkflowTrigger{
		ID:      id,
		Source:  model.TriggerSourceIM,
		Config:  json.RawMessage(`{"platform":"feishu"}`),
		Enabled: false,
	}
	svc := trigger.NewCRUDService(repo, nil, nil)

	res, err := svc.Test(context.Background(), id, map[string]any{"platform": "feishu"})
	if err != nil {
		t.Fatalf("test: %v", err)
	}
	if !res.Matched {
		t.Error("expected matched=true")
	}
	if res.WouldDispatch {
		t.Error("expected would_dispatch=false for disabled trigger")
	}
	if res.SkipReason != "trigger_disabled" {
		t.Errorf("SkipReason: want trigger_disabled, got %q", res.SkipReason)
	}
}

func TestTriggerService_Test_NotFound(t *testing.T) {
	repo := newMockCRUDRepo()
	svc := trigger.NewCRUDService(repo, nil, nil)
	_, err := svc.Test(context.Background(), uuid.New(), map[string]any{})
	if !errors.Is(err, trigger.ErrTriggerNotFound) {
		t.Errorf("expected ErrTriggerNotFound, got %v", err)
	}
}
