package employee_test

import (
	"context"
	"errors"
	"strings"
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

func TestService_Invoke_StubReturnsNotImplementedError(t *testing.T) {
	repo := newMockEmployeeRepo()
	roles := newMockRoleRegistry()
	svc := newService(repo, roles)

	result, err := svc.Invoke(context.Background(), employee.InvokeInput{
		EmployeeID: uuid.New(),
		TaskID:     uuid.New(),
	})
	if err == nil {
		t.Fatal("expected non-nil error from Invoke stub")
	}
	if result != nil {
		t.Error("expected nil result from Invoke stub")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("expected 'not implemented' in error message, got: %v", err)
	}
}
