package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/employee"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type employeeService interface {
	Create(ctx context.Context, in employee.CreateInput) (*model.Employee, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Employee, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, f repository.EmployeeFilter) ([]*model.Employee, error)
	Update(ctx context.Context, id uuid.UUID, in employee.UpdateInput) (*model.Employee, error)
	SetState(ctx context.Context, id uuid.UUID, state model.EmployeeState) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddSkill(ctx context.Context, employeeID uuid.UUID, sk model.EmployeeSkill) error
	RemoveSkill(ctx context.Context, employeeID uuid.UUID, skillPath string) error
}

// EmployeeHandler handles HTTP requests for the Employee resource.
type EmployeeHandler struct{ service employeeService }

// NewEmployeeHandler returns a new EmployeeHandler backed by the given service.
func NewEmployeeHandler(service employeeService) *EmployeeHandler {
	return &EmployeeHandler{service: service}
}

// Register attaches employee routes to a project-scoped Echo group.
// The group is already protected by the project middleware that sets the project ID.
func (h *EmployeeHandler) Register(g *echo.Group) {
	g.GET("/employees", h.List)
	g.POST("/employees", h.Create)
	g.GET("/employees/:id", h.Get)
	g.PATCH("/employees/:id", h.Update)
	g.DELETE("/employees/:id", h.Delete)
	g.POST("/employees/:id/state", h.SetState)
	g.GET("/employees/:id/skills", h.ListSkills)
	g.POST("/employees/:id/skills", h.AddSkill)
	g.DELETE("/employees/:id/skills/:skillPath", h.RemoveSkill)
}

// mapEmployeeError converts service-layer errors to HTTP status + i18n message key.
func mapEmployeeError(c echo.Context, err error, failMsg string) error {
	switch {
	case errors.Is(err, employee.ErrRoleNotFound):
		return localizedError(c, http.StatusBadRequest, i18n.MsgRoleNotFoundForEmployee)
	case errors.Is(err, employee.ErrEmployeeNameExists):
		return localizedError(c, http.StatusConflict, i18n.MsgEmployeeNameExists)
	case errors.Is(err, repository.ErrNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgEmployeeNotFound)
	default:
		return localizedError(c, http.StatusInternalServerError, failMsg)
	}
}

// List handles GET /employees
func (h *EmployeeHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	filter := repository.EmployeeFilter{}
	if stateStr := c.QueryParam("state"); stateStr != "" {
		s := model.EmployeeState(stateStr)
		filter.State = &s
	}
	employees, err := h.service.ListByProject(c.Request().Context(), projectID, filter)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListEmployees)
	}
	return c.JSON(http.StatusOK, employees)
}

// createEmployeeRequest is the request body for POST /employees.
type createEmployeeRequest struct {
	Name         string               `json:"name"`
	DisplayName  string               `json:"displayName,omitempty"`
	RoleID       string               `json:"roleId"`
	RuntimePrefs json.RawMessage      `json:"runtimePrefs,omitempty"`
	Config       json.RawMessage      `json:"config,omitempty"`
	Skills       []model.EmployeeSkill `json:"skills,omitempty"`
}

// Create handles POST /employees
func (h *EmployeeHandler) Create(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(createEmployeeRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	if req.Name == "" || req.RoleID == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	in := employee.CreateInput{
		ProjectID:    projectID,
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		RoleID:       req.RoleID,
		RuntimePrefs: req.RuntimePrefs,
		Config:       req.Config,
		Skills:       req.Skills,
	}
	emp, err := h.service.Create(c.Request().Context(), in)
	if err != nil {
		return mapEmployeeError(c, err, i18n.MsgFailedToCreateEmployee)
	}
	return c.JSON(http.StatusCreated, emp)
}

// Get handles GET /employees/:id
func (h *EmployeeHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	emp, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return mapEmployeeError(c, err, i18n.MsgEmployeeNotFound)
	}
	if emp == nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgEmployeeNotFound)
	}
	return c.JSON(http.StatusOK, emp)
}

// updateEmployeeRequest is the request body for PATCH /employees/:id.
type updateEmployeeRequest struct {
	DisplayName  *string         `json:"displayName"`
	RoleID       *string         `json:"roleId"`
	RuntimePrefs json.RawMessage `json:"runtimePrefs,omitempty"`
	Config       json.RawMessage `json:"config,omitempty"`
}

// Update handles PATCH /employees/:id
func (h *EmployeeHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	req := new(updateEmployeeRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	in := employee.UpdateInput{
		DisplayName:  req.DisplayName,
		RoleID:       req.RoleID,
		RuntimePrefs: req.RuntimePrefs,
		Config:       req.Config,
	}
	emp, err := h.service.Update(c.Request().Context(), id, in)
	if err != nil {
		return mapEmployeeError(c, err, i18n.MsgFailedToUpdateEmployee)
	}
	return c.JSON(http.StatusOK, emp)
}

// Delete handles DELETE /employees/:id
func (h *EmployeeHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return mapEmployeeError(c, err, i18n.MsgFailedToDeleteEmployee)
	}
	return c.NoContent(http.StatusNoContent)
}

// setStateRequest is the request body for POST /employees/:id/state.
type setStateRequest struct {
	State string `json:"state"`
}

// SetState handles POST /employees/:id/state
func (h *EmployeeHandler) SetState(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	req := new(setStateRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	state := model.EmployeeState(req.State)
	switch state {
	case model.EmployeeStateActive, model.EmployeeStatePaused, model.EmployeeStateArchived:
		// valid
	default:
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeState)
	}
	if err := h.service.SetState(c.Request().Context(), id, state); err != nil {
		return mapEmployeeError(c, err, i18n.MsgFailedToSetEmployeeState)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListSkills handles GET /employees/:id/skills
func (h *EmployeeHandler) ListSkills(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	emp, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return mapEmployeeError(c, err, i18n.MsgEmployeeNotFound)
	}
	if emp == nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgEmployeeNotFound)
	}
	skills := emp.Skills
	if skills == nil {
		skills = []model.EmployeeSkill{}
	}
	return c.JSON(http.StatusOK, skills)
}

// addSkillRequest is the request body for POST /employees/:id/skills.
type addSkillRequest struct {
	SkillPath string          `json:"skillPath"`
	AutoLoad  bool            `json:"autoLoad"`
	Overrides json.RawMessage `json:"overrides,omitempty"`
}

// AddSkill handles POST /employees/:id/skills
func (h *EmployeeHandler) AddSkill(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	req := new(addSkillRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	if req.SkillPath == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	sk := model.EmployeeSkill{
		EmployeeID: id,
		SkillPath:  req.SkillPath,
		AutoLoad:   req.AutoLoad,
		Overrides:  req.Overrides,
	}
	if err := h.service.AddSkill(c.Request().Context(), id, sk); err != nil {
		return mapEmployeeError(c, err, i18n.MsgFailedToAddEmployeeSkill)
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveSkill handles DELETE /employees/:id/skills/:skillPath
func (h *EmployeeHandler) RemoveSkill(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}
	skillPath := c.Param("skillPath")
	if skillPath == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}
	if err := h.service.RemoveSkill(c.Request().Context(), id, skillPath); err != nil {
		return mapEmployeeError(c, err, i18n.MsgFailedToRemoveEmployeeSkill)
	}
	return c.NoContent(http.StatusNoContent)
}
