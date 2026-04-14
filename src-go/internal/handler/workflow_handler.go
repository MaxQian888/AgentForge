package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type workflowRepository interface {
	GetByProject(ctx context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error)
	Upsert(ctx context.Context, projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) (*model.WorkflowConfig, error)
}

// DAGWorkflowServiceInterface defines the DAG workflow execution methods needed by the handler.
type DAGWorkflowServiceInterface interface {
	StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID) (*model.WorkflowExecution, error)
	AdvanceExecution(ctx context.Context, executionID uuid.UUID) error
	CancelExecution(ctx context.Context, executionID uuid.UUID) error
	ResolveHumanReview(ctx context.Context, executionID uuid.UUID, nodeID string, decision string, comment string) error
	HandleExternalEvent(ctx context.Context, executionID uuid.UUID, nodeID string, payload json.RawMessage) error
}

// WorkflowTemplateServiceInterface defines the template service methods needed by the handler.
type WorkflowTemplateServiceInterface interface {
	ListTemplates(ctx context.Context, category string) ([]*model.WorkflowDefinition, error)
	CloneTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, overrides map[string]any) (*model.WorkflowDefinition, error)
	CreateFromTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, taskID *uuid.UUID, variables map[string]any) (*model.WorkflowExecution, error)
}

// DAGWorkflowDefRepoInterface defines the definition CRUD methods needed by the handler.
type DAGWorkflowDefRepoInterface interface {
	Create(ctx context.Context, def *model.WorkflowDefinition) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowDefinition, error)
	Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// DAGWorkflowExecRepoInterface defines the execution read methods needed by the handler.
type DAGWorkflowExecRepoInterface interface {
	GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
	ListExecutions(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowExecution, error)
}

// DAGWorkflowNodeExecRepoInterface defines the node execution read methods needed by the handler.
type DAGWorkflowNodeExecRepoInterface interface {
	ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error)
}

// WorkflowReviewRepoInterface defines the review persistence methods needed by the handler.
type WorkflowReviewRepoInterface interface {
	ListPendingByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowPendingReview, error)
}

type WorkflowHandler struct {
	repo        workflowRepository
	dagSvc      DAGWorkflowServiceInterface
	dagDefRepo  DAGWorkflowDefRepoInterface
	dagExecRepo DAGWorkflowExecRepoInterface
	dagNodeRepo DAGWorkflowNodeExecRepoInterface
	templateSvc WorkflowTemplateServiceInterface
	reviewRepo  WorkflowReviewRepoInterface
}

func NewWorkflowHandler(repo workflowRepository) *WorkflowHandler {
	return &WorkflowHandler{repo: repo}
}

// WithDAGService wires the DAG workflow service and repositories.
func (h *WorkflowHandler) WithDAGService(
	svc DAGWorkflowServiceInterface,
	defRepo DAGWorkflowDefRepoInterface,
	execRepo DAGWorkflowExecRepoInterface,
	nodeRepo DAGWorkflowNodeExecRepoInterface,
) *WorkflowHandler {
	h.dagSvc = svc
	h.dagDefRepo = defRepo
	h.dagExecRepo = execRepo
	h.dagNodeRepo = nodeRepo
	return h
}

// WithTemplateService wires the workflow template service.
func (h *WorkflowHandler) WithTemplateService(svc WorkflowTemplateServiceInterface) *WorkflowHandler {
	h.templateSvc = svc
	return h
}

// WithReviewRepo wires the pending review repository.
func (h *WorkflowHandler) WithReviewRepo(repo WorkflowReviewRepoInterface) *WorkflowHandler {
	h.reviewRepo = repo
	return h
}

// --- Existing workflow config endpoints ---

func (h *WorkflowHandler) Get(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	wf, err := h.repo.GetByProject(c.Request().Context(), projectID)
	if err != nil {
		// Return default empty config if not found
		return c.JSON(http.StatusOK, model.WorkflowConfigDTO{
			ProjectID:   projectID.String(),
			Transitions: make(map[string][]string),
			Triggers:    make([]model.WorkflowTrigger, 0),
		})
	}
	return c.JSON(http.StatusOK, wf.ToDTO())
}

func (h *WorkflowHandler) Put(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)

	req := new(model.UpdateWorkflowRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	transitionsJSON, err := json.Marshal(req.Transitions)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTransitions)
	}
	triggersJSON, err := json.Marshal(req.Triggers)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTriggers)
	}

	wf, err := h.repo.Upsert(c.Request().Context(), projectID, transitionsJSON, triggersJSON)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSaveWorkflowConfig)
	}

	return c.JSON(http.StatusOK, wf.ToDTO())
}

// --- DAG Workflow Definition endpoints ---

// CreateDefinition creates a new workflow definition for the current project.
func (h *WorkflowHandler) CreateDefinition(c echo.Context) error {
	if h.dagDefRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	projectID := appMiddleware.GetProjectID(c)

	req := new(model.CreateWorkflowDefinitionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	nodesJSON, _ := json.Marshal(req.Nodes)
	edgesJSON, _ := json.Marshal(req.Edges)

	def := &model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		Status:      model.WorkflowDefStatusDraft,
		Nodes:       nodesJSON,
		Edges:       edgesJSON,
	}

	if err := h.dagDefRepo.Create(c.Request().Context(), def); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateWorkflow)
	}

	created, err := h.dagDefRepo.GetByID(c.Request().Context(), def.ID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateWorkflow)
	}
	return c.JSON(http.StatusCreated, created.ToDTO())
}

// ListDefinitions lists all workflow definitions for the current project.
func (h *WorkflowHandler) ListDefinitions(c echo.Context) error {
	if h.dagDefRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	projectID := appMiddleware.GetProjectID(c)

	defs, err := h.dagDefRepo.ListByProject(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListWorkflows)
	}

	result := make([]model.WorkflowDefinitionDTO, len(defs))
	for i, def := range defs {
		result[i] = def.ToDTO()
	}
	return c.JSON(http.StatusOK, result)
}

// GetDefinition gets a single workflow definition by ID.
func (h *WorkflowHandler) GetDefinition(c echo.Context) error {
	if h.dagDefRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}

	def, err := h.dagDefRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWorkflowNotFound)
	}
	return c.JSON(http.StatusOK, def.ToDTO())
}

// UpdateDefinition updates a workflow definition.
func (h *WorkflowHandler) UpdateDefinition(c echo.Context) error {
	if h.dagDefRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}

	req := new(model.UpdateWorkflowDefinitionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	updates := &model.WorkflowDefinition{}
	if req.Name != nil {
		updates.Name = *req.Name
	}
	if req.Description != nil {
		updates.Description = *req.Description
	}
	if req.Status != nil {
		updates.Status = *req.Status
	}
	if req.Nodes != nil {
		nodesJSON, _ := json.Marshal(*req.Nodes)
		updates.Nodes = nodesJSON
	}
	if req.Edges != nil {
		edgesJSON, _ := json.Marshal(*req.Edges)
		updates.Edges = edgesJSON
	}

	if err := h.dagDefRepo.Update(c.Request().Context(), id, updates); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateWorkflow)
	}

	updated, err := h.dagDefRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWorkflowNotFound)
	}
	return c.JSON(http.StatusOK, updated.ToDTO())
}

// DeleteDefinition deletes a workflow definition.
func (h *WorkflowHandler) DeleteDefinition(c echo.Context) error {
	if h.dagDefRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}

	if err := h.dagDefRepo.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteWorkflow)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- DAG Workflow Execution endpoints ---

// StartExecution starts a new workflow execution.
func (h *WorkflowHandler) StartExecution(c echo.Context) error {
	if h.dagSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}

	req := new(model.StartWorkflowExecutionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	var taskID *uuid.UUID
	if req.TaskID != nil {
		parsed, err := uuid.Parse(*req.TaskID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
		}
		taskID = &parsed
	}

	exec, err := h.dagSvc.StartExecution(c.Request().Context(), id, taskID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, exec.ToDTO())
}

// ListExecutions lists executions for a workflow.
func (h *WorkflowHandler) ListExecutions(c echo.Context) error {
	if h.dagExecRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}

	execs, err := h.dagExecRepo.ListExecutions(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListExecutions)
	}

	result := make([]model.WorkflowExecutionDTO, len(execs))
	for i, exec := range execs {
		result[i] = exec.ToDTO()
	}
	return c.JSON(http.StatusOK, result)
}

// GetExecution gets a single execution with node executions.
func (h *WorkflowHandler) GetExecution(c echo.Context) error {
	if h.dagExecRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidExecutionID)
	}

	exec, err := h.dagExecRepo.GetExecution(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgExecutionNotFound)
	}

	execDTO := exec.ToDTO()

	// Also include node executions if available
	if h.dagNodeRepo != nil {
		nodeExecs, err := h.dagNodeRepo.ListNodeExecutions(c.Request().Context(), id)
		if err == nil {
			nodeExecDTOs := make([]model.WorkflowNodeExecutionDTO, len(nodeExecs))
			for i, ne := range nodeExecs {
				nodeExecDTOs[i] = ne.ToDTO()
			}
			return c.JSON(http.StatusOK, map[string]any{
				"execution":      execDTO,
				"nodeExecutions": nodeExecDTOs,
			})
		}
	}

	return c.JSON(http.StatusOK, execDTO)
}

// CancelExecution cancels a running execution.
func (h *WorkflowHandler) CancelExecution(c echo.Context) error {
	if h.dagSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidExecutionID)
	}

	if err := h.dagSvc.CancelExecution(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Template endpoints ---

// ListTemplates lists workflow templates, optionally filtered by category.
func (h *WorkflowHandler) ListTemplates(c echo.Context) error {
	if h.templateSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	category := c.QueryParam("category")
	templates, err := h.templateSvc.ListTemplates(c.Request().Context(), category)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	result := make([]model.WorkflowDefinitionDTO, len(templates))
	for i, t := range templates {
		result[i] = t.ToDTO()
	}
	return c.JSON(http.StatusOK, result)
}

// CloneTemplate creates a new workflow from a template.
func (h *WorkflowHandler) CloneTemplate(c echo.Context) error {
	if h.templateSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}
	projectID := appMiddleware.GetProjectID(c)

	var body struct {
		Overrides map[string]any `json:"overrides"`
	}
	_ = c.Bind(&body)

	clone, err := h.templateSvc.CloneTemplate(c.Request().Context(), templateID, projectID, body.Overrides)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, clone.ToDTO())
}

// ExecuteTemplate clones a template and immediately starts execution.
func (h *WorkflowHandler) ExecuteTemplate(c echo.Context) error {
	if h.templateSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}
	projectID := appMiddleware.GetProjectID(c)

	var body struct {
		TaskID    *string        `json:"taskId"`
		Variables map[string]any `json:"variables"`
	}
	_ = c.Bind(&body)

	var taskID *uuid.UUID
	if body.TaskID != nil {
		parsed, err := uuid.Parse(*body.TaskID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
		}
		taskID = &parsed
	}

	exec, err := h.templateSvc.CreateFromTemplate(c.Request().Context(), templateID, projectID, taskID, body.Variables)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, exec.ToDTO())
}

// --- Human review and external event endpoints ---

// ListPendingReviews lists pending human reviews for the current project.
func (h *WorkflowHandler) ListPendingReviews(c echo.Context) error {
	if h.reviewRepo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	projectID := appMiddleware.GetProjectID(c)
	reviews, err := h.reviewRepo.ListPendingByProject(c.Request().Context(), projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	result := make([]model.WorkflowPendingReviewDTO, len(reviews))
	for i, r := range reviews {
		result[i] = r.ToDTO()
	}
	return c.JSON(http.StatusOK, result)
}

// ResolveHumanReview resolves a pending human review to resume execution.
func (h *WorkflowHandler) ResolveHumanReview(c echo.Context) error {
	if h.dagSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	executionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidExecutionID)
	}
	var body struct {
		NodeID   string `json:"nodeId" validate:"required"`
		Decision string `json:"decision" validate:"required"` // approved, rejected
		Comment  string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	if err := h.dagSvc.ResolveHumanReview(c.Request().Context(), executionID, body.NodeID, body.Decision, body.Comment); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.NoContent(http.StatusOK)
}

// HandleExternalEvent receives an external event to resume a waiting node.
func (h *WorkflowHandler) HandleExternalEvent(c echo.Context) error {
	if h.dagSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	executionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidExecutionID)
	}
	var body struct {
		NodeID  string          `json:"nodeId" validate:"required"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := c.Bind(&body); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	if err := h.dagSvc.HandleExternalEvent(c.Request().Context(), executionID, body.NodeID, body.Payload); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.NoContent(http.StatusOK)
}
