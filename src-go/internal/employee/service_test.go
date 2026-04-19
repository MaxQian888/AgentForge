package employee_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/employee"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

// ---------------------------------------------------------------------------
// Mock Repository
// ---------------------------------------------------------------------------

type mockEmployeeRepo struct {
	employees map[uuid.UUID]*model.Employee
	skills    map[uuid.UUID][]model.EmployeeSkill

	createErr error
	updateErr error
	getErr    error

	createCalls  int
	addSkillCalls []model.EmployeeSkill
}

func newMockEmployeeRepo() *mockEmployeeRepo {
	return &mockEmployeeRepo{
		employees: make(map[uuid.UUID]*model.Employee),
		skills:    make(map[uuid.UUID][]model.EmployeeSkill),
	}
}

func (m *mockEmployeeRepo) Create(_ context.Context, e *model.Employee) error {
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	copy := *e
	m.employees[e.ID] = &copy
	return nil
}

func (m *mockEmployeeRepo) Get(_ context.Context, id uuid.UUID) (*model.Employee, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	e, ok := m.employees[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := *e
	copy.Skills = append([]model.EmployeeSkill(nil), m.skills[id]...)
	return &copy, nil
}

func (m *mockEmployeeRepo) ListByProject(_ context.Context, projectID uuid.UUID, _ repository.EmployeeFilter) ([]*model.Employee, error) {
	var out []*model.Employee
	for _, e := range m.employees {
		if e.ProjectID == projectID {
			copy := *e
			out = append(out, &copy)
		}
	}
	return out, nil
}

func (m *mockEmployeeRepo) Update(_ context.Context, e *model.Employee) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	_, ok := m.employees[e.ID]
	if !ok {
		return repository.ErrNotFound
	}
	copy := *e
	m.employees[e.ID] = &copy
	return nil
}

func (m *mockEmployeeRepo) SetState(_ context.Context, id uuid.UUID, state model.EmployeeState) error {
	e, ok := m.employees[id]
	if !ok {
		return repository.ErrNotFound
	}
	e.State = state
	return nil
}

func (m *mockEmployeeRepo) Delete(_ context.Context, id uuid.UUID) error {
	_, ok := m.employees[id]
	if !ok {
		return repository.ErrNotFound
	}
	delete(m.employees, id)
	return nil
}

func (m *mockEmployeeRepo) AddSkill(_ context.Context, employeeID uuid.UUID, s model.EmployeeSkill) error {
	s.EmployeeID = employeeID
	if s.AddedAt.IsZero() {
		s.AddedAt = time.Now()
	}
	m.addSkillCalls = append(m.addSkillCalls, s)
	m.skills[employeeID] = append(m.skills[employeeID], s)
	return nil
}

func (m *mockEmployeeRepo) RemoveSkill(_ context.Context, employeeID uuid.UUID, skillPath string) error {
	cur := m.skills[employeeID]
	out := cur[:0]
	for _, sk := range cur {
		if sk.SkillPath != skillPath {
			out = append(out, sk)
		}
	}
	m.skills[employeeID] = out
	return nil
}

func (m *mockEmployeeRepo) ListSkills(_ context.Context, employeeID uuid.UUID) ([]model.EmployeeSkill, error) {
	return append([]model.EmployeeSkill(nil), m.skills[employeeID]...), nil
}

// ---------------------------------------------------------------------------
// Mock Role Registry
// ---------------------------------------------------------------------------

type mockRoleRegistry struct {
	known map[string]bool
}

func newMockRoleRegistry(ids ...string) *mockRoleRegistry {
	m := &mockRoleRegistry{known: make(map[string]bool)}
	for _, id := range ids {
		m.known[id] = true
	}
	return m
}

func (m *mockRoleRegistry) Has(roleID string) bool {
	return m.known[roleID]
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newService(repo *mockEmployeeRepo, roles *mockRoleRegistry) *employee.Service {
	return employee.NewService(repo, roles, nil)
}

func newServiceWithSpawner(repo *mockEmployeeRepo, roles *mockRoleRegistry, spawner employee.AgentSpawner) *employee.Service {
	return employee.NewService(repo, roles, spawner)
}

func seedEmployee(repo *mockEmployeeRepo, projectID uuid.UUID, roleID string) *model.Employee {
	e := &model.Employee{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        "worker-" + uuid.New().String()[:8],
		DisplayName: "Worker",
		RoleID:      roleID,
		State:       model.EmployeeStateActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	repo.employees[e.ID] = e
	return e
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_Create_RejectsUnknownRole(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("known-role")
	svc := newService(repo, roles)

	_, err := svc.Create(context.Background(), employee.CreateInput{
		ProjectID: uuid.New(),
		Name:      "bot",
		RoleID:    "unknown-role",
	})

	if !errors.Is(err, employee.ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("repo.Create should not have been called, got %d call(s)", repo.createCalls)
	}
}

func TestService_Create_Success(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	e, err := svc.Create(context.Background(), employee.CreateInput{
		ProjectID: uuid.New(),
		Name:      "bot",
		RoleID:    "dev-role",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.State != model.EmployeeStateActive {
		t.Errorf("expected State=active, got %s", e.State)
	}
	if e.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}

func TestService_Create_NameConflictMappedToServiceError(t *testing.T) {
	repo := newMockEmployeeRepo()
	repo.createErr = repository.ErrEmployeeNameConflict
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	_, err := svc.Create(context.Background(), employee.CreateInput{
		ProjectID: uuid.New(),
		Name:      "dup-name",
		RoleID:    "dev-role",
	})
	if !errors.Is(err, employee.ErrEmployeeNameExists) {
		t.Fatalf("expected ErrEmployeeNameExists, got %v", err)
	}
}

func TestService_Create_PropagatesSkillInserts(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	skills := []model.EmployeeSkill{
		{SkillPath: "skill/alpha", AutoLoad: true},
		{SkillPath: "skill/beta", AutoLoad: false},
	}

	_, err := svc.Create(context.Background(), employee.CreateInput{
		ProjectID: uuid.New(),
		Name:      "skilled-bot",
		RoleID:    "dev-role",
		Skills:    skills,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.addSkillCalls) != 2 {
		t.Fatalf("expected 2 AddSkill calls, got %d", len(repo.addSkillCalls))
	}
	paths := map[string]bool{}
	for _, sk := range repo.addSkillCalls {
		paths[sk.SkillPath] = true
	}
	if !paths["skill/alpha"] || !paths["skill/beta"] {
		t.Errorf("unexpected skill paths: %v", paths)
	}
}

func TestService_Get_Passthrough(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	projectID := uuid.New()
	seeded := seedEmployee(repo, projectID, "dev-role")
	repo.skills[seeded.ID] = []model.EmployeeSkill{
		{SkillPath: "skill/x", AutoLoad: true},
	}

	got, err := svc.Get(context.Background(), seeded.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != seeded.ID {
		t.Errorf("expected ID %s, got %s", seeded.ID, got.ID)
	}
	if len(got.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(got.Skills))
	}
}

func TestService_Update_RoleValidation(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("existing-role")
	svc := newService(repo, roles)

	projectID := uuid.New()
	seeded := seedEmployee(repo, projectID, "existing-role")

	badRole := "no-such-role"
	_, err := svc.Update(context.Background(), seeded.ID, employee.UpdateInput{
		RoleID: &badRole,
	})
	if !errors.Is(err, employee.ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
	// repo.Update must NOT have been called
	stored := repo.employees[seeded.ID]
	if stored.RoleID != "existing-role" {
		t.Errorf("RoleID should be unchanged, got %s", stored.RoleID)
	}
}

func TestService_Update_AppliesPartialFields(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("existing-role")
	svc := newService(repo, roles)

	projectID := uuid.New()
	seeded := seedEmployee(repo, projectID, "existing-role")
	seeded.DisplayName = "Old Name"
	repo.employees[seeded.ID] = seeded

	newName := "New Name"
	updated, err := svc.Update(context.Background(), seeded.ID, employee.UpdateInput{
		DisplayName: &newName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.DisplayName != "New Name" {
		t.Errorf("expected DisplayName=New Name, got %s", updated.DisplayName)
	}
	// RoleID and Config must be preserved
	if updated.RoleID != "existing-role" {
		t.Errorf("RoleID should be preserved, got %s", updated.RoleID)
	}
}

func TestService_SetState_Passthrough(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	projectID := uuid.New()
	seeded := seedEmployee(repo, projectID, "dev-role")

	err := svc.SetState(context.Background(), seeded.ID, model.EmployeeStatePaused)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.employees[seeded.ID].State != model.EmployeeStatePaused {
		t.Errorf("expected state=paused, got %s", repo.employees[seeded.ID].State)
	}

	// ErrNotFound passes through for unknown IDs
	err = svc.SetState(context.Background(), uuid.New(), model.EmployeeStatePaused)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Delete_Passthrough(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	projectID := uuid.New()
	seeded := seedEmployee(repo, projectID, "dev-role")

	if err := svc.Delete(context.Background(), seeded.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := repo.employees[seeded.ID]; exists {
		t.Error("employee should be deleted from mock store")
	}

	// Second delete returns ErrNotFound
	err := svc.Delete(context.Background(), seeded.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_AddSkill_Passthrough(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	empID := uuid.New()
	sk := model.EmployeeSkill{SkillPath: "skill/z", AutoLoad: true}

	if err := svc.AddSkill(context.Background(), empID, sk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.addSkillCalls) != 1 || repo.addSkillCalls[0].SkillPath != "skill/z" {
		t.Errorf("unexpected addSkillCalls: %v", repo.addSkillCalls)
	}
}

func TestService_RemoveSkill_Passthrough(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	svc := newService(repo, roles)

	empID := uuid.New()
	repo.skills[empID] = []model.EmployeeSkill{
		{SkillPath: "skill/z", AutoLoad: true},
	}

	if err := svc.RemoveSkill(context.Background(), empID, "skill/z"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.skills[empID]) != 0 {
		t.Errorf("expected skill to be removed, still have %d", len(repo.skills[empID]))
	}
}

func TestService_Invoke_ArchivedRejected(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	spawner := &fakeSpawner{}
	svc := newServiceWithSpawner(repo, roles, spawner)

	projectID := uuid.New()
	emp := seedEmployee(repo, projectID, "dev-role")
	emp.State = model.EmployeeStateArchived
	repo.employees[emp.ID] = emp

	_, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: emp.ID,
		TaskID:     uuid.New(),
	})
	if !errors.Is(err, employee.ErrEmployeeArchived) {
		t.Fatalf("expected ErrEmployeeArchived, got %v", err)
	}
	if spawner.called {
		t.Error("spawner should NOT have been called for archived employee")
	}
}

func TestService_Invoke_PausedRejected(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	spawner := &fakeSpawner{}
	svc := newServiceWithSpawner(repo, roles, spawner)

	projectID := uuid.New()
	emp := seedEmployee(repo, projectID, "dev-role")
	emp.State = model.EmployeeStatePaused
	repo.employees[emp.ID] = emp

	_, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: emp.ID,
		TaskID:     uuid.New(),
	})
	if !errors.Is(err, employee.ErrEmployeePaused) {
		t.Fatalf("expected ErrEmployeePaused, got %v", err)
	}
	if spawner.called {
		t.Error("spawner should NOT have been called for paused employee")
	}
}

func TestService_Invoke_ActiveSpawns(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")

	runID := uuid.New()
	spawner := &fakeSpawner{result: &model.AgentRun{ID: runID}}
	svc := newServiceWithSpawner(repo, roles, spawner)

	projectID := uuid.New()
	emp := seedEmployee(repo, projectID, "dev-role")
	// Seed runtime prefs
	emp.RuntimePrefs = []byte(`{"runtime":"claude_code","provider":"anthropic","model":"claude-3-5-sonnet","budgetUsd":12.0}`)
	repo.employees[emp.ID] = emp
	// Seed skills
	repo.skills[emp.ID] = []model.EmployeeSkill{
		{SkillPath: "skill/alpha", AutoLoad: true},
	}

	taskID := uuid.New()
	result, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: emp.ID,
		TaskID:     taskID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AgentRunID != runID {
		t.Errorf("expected AgentRunID=%s, got %s", runID, result.AgentRunID)
	}
	if !spawner.called {
		t.Fatal("spawner was not called")
	}
	if spawner.last.EmployeeID != emp.ID {
		t.Errorf("EmployeeID mismatch: got %s", spawner.last.EmployeeID)
	}
	if spawner.last.TaskID != taskID {
		t.Errorf("TaskID mismatch: got %s", spawner.last.TaskID)
	}
	if spawner.last.Runtime != "claude_code" {
		t.Errorf("Runtime mismatch: got %s", spawner.last.Runtime)
	}
	if spawner.last.BudgetUsd != 12.0 {
		t.Errorf("BudgetUsd mismatch: got %f", spawner.last.BudgetUsd)
	}
	if len(spawner.last.ExtraSkills) != 1 || spawner.last.ExtraSkills[0].SkillPath != "skill/alpha" {
		t.Errorf("ExtraSkills mismatch: got %v", spawner.last.ExtraSkills)
	}
}

func TestService_Invoke_BudgetOverride(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	runID := uuid.New()
	spawner := &fakeSpawner{result: &model.AgentRun{ID: runID}}
	svc := newServiceWithSpawner(repo, roles, spawner)

	projectID := uuid.New()
	emp := seedEmployee(repo, projectID, "dev-role")
	emp.RuntimePrefs = []byte(`{"budgetUsd":5.0}`)
	repo.employees[emp.ID] = emp

	override := 10.0
	_, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID:     emp.ID,
		TaskID:         uuid.New(),
		BudgetOverride: &override,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.last.BudgetUsd != 10.0 {
		t.Errorf("expected BudgetUsd=10.0, got %f", spawner.last.BudgetUsd)
	}
}

func TestService_Invoke_DefaultBudgetWhenPrefsEmpty(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	runID := uuid.New()
	spawner := &fakeSpawner{result: &model.AgentRun{ID: runID}}
	svc := newServiceWithSpawner(repo, roles, spawner)

	projectID := uuid.New()
	emp := seedEmployee(repo, projectID, "dev-role")
	// Empty RuntimePrefs — no budget set
	emp.RuntimePrefs = nil
	repo.employees[emp.ID] = emp

	_, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: emp.ID,
		TaskID:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.last.BudgetUsd != 5.0 {
		t.Errorf("expected default BudgetUsd=5.0, got %f", spawner.last.BudgetUsd)
	}
}

func TestService_Invoke_SystemPromptOverrideExtracted(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	runID := uuid.New()
	spawner := &fakeSpawner{result: &model.AgentRun{ID: runID}}
	svc := newServiceWithSpawner(repo, roles, spawner)

	projectID := uuid.New()
	emp := seedEmployee(repo, projectID, "dev-role")
	emp.Config = []byte(`{"system_prompt_override":"Be formal"}`)
	repo.employees[emp.ID] = emp

	_, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: emp.ID,
		TaskID:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.last.SystemPromptOverride != "Be formal" {
		t.Errorf("expected SystemPromptOverride='Be formal', got %q", spawner.last.SystemPromptOverride)
	}
}

func TestService_Invoke_EmployeeNotFound(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry("dev-role")
	spawner := &fakeSpawner{}
	svc := newServiceWithSpawner(repo, roles, spawner)

	_, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: uuid.New(), // does not exist
		TaskID:     uuid.New(),
	})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if spawner.called {
		t.Error("spawner should NOT have been called")
	}
}

// ---------------------------------------------------------------------------
// fakeSpawner
// ---------------------------------------------------------------------------

type fakeSpawner struct {
	called bool
	last   employee.SpawnForEmployeeInput
	result *model.AgentRun
	err    error
}

func (f *fakeSpawner) SpawnForEmployee(_ context.Context, in employee.SpawnForEmployeeInput) (*model.AgentRun, error) {
	f.called = true
	f.last = in
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}
