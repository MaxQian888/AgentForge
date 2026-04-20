package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/employee"
	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// employeeServiceMock satisfies the handler's private employeeService interface.
type employeeServiceMock struct {
	employees map[uuid.UUID]*model.Employee

	// error injection
	createErr      error
	listErr        error
	updateErr      error
	deleteErr      error
	setStateErr    error
	addSkillErr    error
	removeSkillErr error

	// captured call args
	lastFilter    repository.EmployeeFilter
	lastUpdateIn  employee.UpdateInput
	lastState     model.EmployeeState
	lastSkill     model.EmployeeSkill
	lastSkillPath string
}

func newEmployeeServiceMock() *employeeServiceMock {
	return &employeeServiceMock{employees: make(map[uuid.UUID]*model.Employee)}
}

func (m *employeeServiceMock) Create(_ context.Context, in employee.CreateInput) (*model.Employee, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	emp := &model.Employee{
		ID:          uuid.New(),
		ProjectID:   in.ProjectID,
		Name:        in.Name,
		DisplayName: in.DisplayName,
		RoleID:      in.RoleID,
		State:       model.EmployeeStateActive,
	}
	m.employees[emp.ID] = emp
	return emp, nil
}

func (m *employeeServiceMock) Get(_ context.Context, id uuid.UUID) (*model.Employee, error) {
	emp, ok := m.employees[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return emp, nil
}

func (m *employeeServiceMock) ListByProject(_ context.Context, _ uuid.UUID, f repository.EmployeeFilter) ([]*model.Employee, error) {
	m.lastFilter = f
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []*model.Employee
	for _, e := range m.employees {
		out = append(out, e)
	}
	return out, nil
}

func (m *employeeServiceMock) Update(_ context.Context, id uuid.UUID, in employee.UpdateInput) (*model.Employee, error) {
	m.lastUpdateIn = in
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	emp, ok := m.employees[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if in.DisplayName != nil {
		emp.DisplayName = *in.DisplayName
	}
	if in.RoleID != nil {
		emp.RoleID = *in.RoleID
	}
	return emp, nil
}

func (m *employeeServiceMock) SetState(_ context.Context, id uuid.UUID, state model.EmployeeState) error {
	m.lastState = state
	if m.setStateErr != nil {
		return m.setStateErr
	}
	emp, ok := m.employees[id]
	if !ok {
		return repository.ErrNotFound
	}
	emp.State = state
	return nil
}

func (m *employeeServiceMock) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.employees[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.employees, id)
	return nil
}

func (m *employeeServiceMock) AddSkill(_ context.Context, employeeID uuid.UUID, sk model.EmployeeSkill) error {
	m.lastSkill = sk
	if m.addSkillErr != nil {
		return m.addSkillErr
	}
	emp, ok := m.employees[employeeID]
	if !ok {
		return repository.ErrNotFound
	}
	emp.Skills = append(emp.Skills, sk)
	return nil
}

func (m *employeeServiceMock) RemoveSkill(_ context.Context, employeeID uuid.UUID, skillPath string) error {
	m.lastSkillPath = skillPath
	if m.removeSkillErr != nil {
		return m.removeSkillErr
	}
	if _, ok := m.employees[employeeID]; !ok {
		return repository.ErrNotFound
	}
	return nil
}

// setupEmployeeEcho returns a configured Echo instance and a fixed projectID set in context.
func setupEmployeeEcho(t *testing.T) (*echo.Echo, uuid.UUID) {
	t.Helper()
	e := echo.New()
	return e, uuid.New()
}

func setProjectIDOnCtx(c echo.Context, pid uuid.UUID) {
	c.Set(appMiddleware.ProjectIDContextKey, pid)
}

// TestEmployeeHandler_Create_Success: POST → 201 + body.
func TestEmployeeHandler_Create_Success(t *testing.T) {
	e, pid := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	h := handler.NewEmployeeHandler(svc)

	body := `{"name":"alice","roleId":"developer"}`
	req := httptest.NewRequest(http.MethodPost, "/employees", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setProjectIDOnCtx(c, pid)

	if err := h.Create(c); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["name"] != "alice" {
		t.Errorf("unexpected name: %v", out["name"])
	}
}

// TestEmployeeHandler_Create_UnknownRole: service returns ErrRoleNotFound → 400.
func TestEmployeeHandler_Create_UnknownRole(t *testing.T) {
	e, pid := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	svc.createErr = employee.ErrRoleNotFound
	h := handler.NewEmployeeHandler(svc)

	body := `{"name":"bob","roleId":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/employees", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setProjectIDOnCtx(c, pid)

	if err := h.Create(c); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestEmployeeHandler_Create_NameConflict: service returns ErrEmployeeNameExists → 409.
func TestEmployeeHandler_Create_NameConflict(t *testing.T) {
	e, pid := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	svc.createErr = employee.ErrEmployeeNameExists
	h := handler.NewEmployeeHandler(svc)

	body := `{"name":"alice","roleId":"developer"}`
	req := httptest.NewRequest(http.MethodPost, "/employees", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setProjectIDOnCtx(c, pid)

	if err := h.Create(c); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

// TestEmployeeHandler_List_Filters: GET with ?state=paused → filter passed to service.
func TestEmployeeHandler_List_Filters(t *testing.T) {
	e, pid := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	h := handler.NewEmployeeHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/employees?state=paused", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setProjectIDOnCtx(c, pid)

	if err := h.List(c); err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.lastFilter.State == nil || *svc.lastFilter.State != model.EmployeeStatePaused {
		t.Errorf("expected filter.State = paused, got %v", svc.lastFilter.State)
	}
}

// TestEmployeeHandler_Get_NotFound: service returns ErrNotFound → 404.
func TestEmployeeHandler_Get_NotFound(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	h := handler.NewEmployeeHandler(svc)

	unknownID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/employees/"+unknownID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(unknownID.String())

	if err := h.Get(c); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestEmployeeHandler_Update_PartialFields: PATCH with only displayName → service sees only that field.
func TestEmployeeHandler_Update_PartialFields(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	empID := uuid.New()
	svc.employees[empID] = &model.Employee{ID: empID, Name: "alice", RoleID: "developer", State: model.EmployeeStateActive}
	h := handler.NewEmployeeHandler(svc)

	body := `{"displayName":"Alice Smith"}`
	req := httptest.NewRequest(http.MethodPatch, "/employees/"+empID.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(empID.String())

	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if svc.lastUpdateIn.DisplayName == nil || *svc.lastUpdateIn.DisplayName != "Alice Smith" {
		t.Errorf("expected DisplayName = 'Alice Smith', got %v", svc.lastUpdateIn.DisplayName)
	}
	if svc.lastUpdateIn.RoleID != nil {
		t.Errorf("expected RoleID to be nil (not sent), got %v", svc.lastUpdateIn.RoleID)
	}
}

// TestEmployeeHandler_SetState_Invalid: unknown state → 400.
func TestEmployeeHandler_SetState_Invalid(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	empID := uuid.New()
	svc.employees[empID] = &model.Employee{ID: empID, State: model.EmployeeStateActive}
	h := handler.NewEmployeeHandler(svc)

	body := `{"state":"broken"}`
	req := httptest.NewRequest(http.MethodPost, "/employees/"+empID.String()+"/state", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(empID.String())

	if err := h.SetState(c); err != nil {
		t.Fatalf("SetState() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestEmployeeHandler_SetState_Valid: valid state → 204.
func TestEmployeeHandler_SetState_Valid(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	empID := uuid.New()
	svc.employees[empID] = &model.Employee{ID: empID, State: model.EmployeeStateActive}
	h := handler.NewEmployeeHandler(svc)

	body := `{"state":"paused"}`
	req := httptest.NewRequest(http.MethodPost, "/employees/"+empID.String()+"/state", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(empID.String())

	if err := h.SetState(c); err != nil {
		t.Fatalf("SetState() error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if svc.lastState != model.EmployeeStatePaused {
		t.Errorf("expected state paused, got %v", svc.lastState)
	}
}

// TestEmployeeHandler_Delete_NotFound: employee absent → 404.
func TestEmployeeHandler_Delete_NotFound(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	h := handler.NewEmployeeHandler(svc)

	unknownID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/employees/"+unknownID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(unknownID.String())

	if err := h.Delete(c); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestEmployeeHandler_AddSkill_Success: valid skill body → 204.
func TestEmployeeHandler_AddSkill_Success(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	empID := uuid.New()
	svc.employees[empID] = &model.Employee{ID: empID, State: model.EmployeeStateActive}
	h := handler.NewEmployeeHandler(svc)

	body := fmt.Sprintf(`{"skillPath":"tools/search","autoLoad":true}`)
	req := httptest.NewRequest(http.MethodPost, "/employees/"+empID.String()+"/skills", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(empID.String())

	if err := h.AddSkill(c); err != nil {
		t.Fatalf("AddSkill() error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if svc.lastSkill.SkillPath != "tools/search" {
		t.Errorf("expected skillPath 'tools/search', got %q", svc.lastSkill.SkillPath)
	}
}

// TestEmployeeHandler_RemoveSkill: valid remove → 204.
func TestEmployeeHandler_RemoveSkill(t *testing.T) {
	e, _ := setupEmployeeEcho(t)
	svc := newEmployeeServiceMock()
	empID := uuid.New()
	svc.employees[empID] = &model.Employee{ID: empID, State: model.EmployeeStateActive}
	h := handler.NewEmployeeHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/employees/"+empID.String()+"/skills/tools%2Fsearch", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "skillPath")
	c.SetParamValues(empID.String(), "tools/search")

	if err := h.RemoveSkill(c); err != nil {
		t.Fatalf("RemoveSkill() error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if svc.lastSkillPath != "tools/search" {
		t.Errorf("expected skillPath 'tools/search', got %q", svc.lastSkillPath)
	}
}
