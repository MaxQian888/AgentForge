package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/agentforge/server/internal/trigger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type workflowRepository interface {
	GetByProject(ctx context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error)
	Upsert(ctx context.Context, projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) (*model.WorkflowConfig, error)
}

// DAGWorkflowServiceInterface defines the DAG workflow execution methods needed by the handler.
type DAGWorkflowServiceInterface interface {
	StartExecution(ctx context.Context, workflowID uuid.UUID, taskID *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error)
	AdvanceExecution(ctx context.Context, executionID uuid.UUID) error
	CancelExecution(ctx context.Context, executionID uuid.UUID) error
	ResolveHumanReview(ctx context.Context, executionID uuid.UUID, nodeID string, decision string, comment string) error
	HandleExternalEvent(ctx context.Context, executionID uuid.UUID, nodeID string, payload json.RawMessage) error
}

// WorkflowTemplateServiceInterface defines the template service methods needed by the handler.
type WorkflowTemplateServiceInterface interface {
	ListTemplates(ctx context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error)
	CloneTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, overrides map[string]any) (*model.WorkflowDefinition, error)
	CreateFromTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, taskID *uuid.UUID, variables map[string]any) (*model.WorkflowExecution, error)
	PublishDefinitionAsTemplate(ctx context.Context, definitionID uuid.UUID, projectID uuid.UUID, name string, description string) (*model.WorkflowDefinition, error)
	DuplicateTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, name string, description string) (*model.WorkflowDefinition, error)
	DeleteTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID) error
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

// triggerSyncer is the subset of *trigger.Registrar the handler uses.
// Keeping it local means main.go can wire any implementation that
// matches, including a no-op for test configurations.
type triggerSyncer interface {
	SyncFromDefinition(ctx context.Context, workflowID, projectID uuid.UUID, nodes []model.WorkflowNode, createdBy *uuid.UUID) ([]trigger.SyncOutcome, error)
}

// subWorkflowLinkReader is the read-only view of the parent-link repository the
// handler uses to surface sub-workflow linkage on the execution read DTO. Kept
// local so main.go can wire any implementation (production repo, in-memory
// fake) without tying the handler to a concrete package.
type subWorkflowLinkReader interface {
	GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error)
	ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*model.WorkflowRunParentLink, error)
}

type WorkflowHandler struct {
	repo          workflowRepository
	dagSvc        DAGWorkflowServiceInterface
	dagDefRepo    DAGWorkflowDefRepoInterface
	dagExecRepo   DAGWorkflowExecRepoInterface
	dagNodeRepo   DAGWorkflowNodeExecRepoInterface
	templateSvc   WorkflowTemplateServiceInterface
	reviewRepo    WorkflowReviewRepoInterface
	triggerSyncer triggerSyncer
	linkReader    subWorkflowLinkReader
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

// SetTriggerSyncer wires the trigger syncer (called from Task 18 startup wiring).
// Passing nil is safe — the handler skips sync when no syncer is wired.
func (h *WorkflowHandler) SetTriggerSyncer(s triggerSyncer) {
	h.triggerSyncer = s
}

// WithParentLinkReader wires the sub-workflow parent-link reader used by the
// execution read DTO to expose parent↔child linkage. Passing nil is safe —
// the handler omits linkage fields when no reader is wired (useful for tests
// and for deployments where linkage is not yet enabled).
func (h *WorkflowHandler) WithParentLinkReader(r subWorkflowLinkReader) *WorkflowHandler {
	h.linkReader = r
	return h
}

// --- Sub-workflow save-time validation -------------------------------------

// Rejection reasons emitted by validateSubWorkflowNodes. Machine-readable so
// clients can render a targeted message per failure mode without parsing the
// human-facing sentence. See bridge-sub-workflow-invocation task 7.2.
const (
	subWorkflowRejectInvalidTargetKind = "invalid_target_kind"
	subWorkflowRejectUnknownTarget     = "unknown_target"
	subWorkflowRejectCrossProject      = "cross_project"
	subWorkflowRejectTrivialSelfLoop   = "trivial_self_loop"
	subWorkflowRejectMissingTarget     = "missing_target"
)

// subWorkflowRejection is the JSON body returned on a save-time rejection.
// Fields are deliberately narrow so the frontend's error surface can drive
// per-reason UI (e.g. highlighting the offending node in the canvas).
type subWorkflowRejection struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
	NodeID string `json:"nodeId,omitempty"`
	Target string `json:"target,omitempty"`
}

// validateSubWorkflowNodes walks nodes in the incoming save payload and
// rejects any `sub_workflow` node whose target is unresolvable, cross-project,
// or a trivial self-loop. Called before the DB write so bad configs never
// persist. Deep plugin-runtime validation (e.g. plugin not currently enabled)
// stays at dispatch time in the applier; save-time validation only enforces
// invariants the handler can check without pulling the plugin control plane.
func (h *WorkflowHandler) validateSubWorkflowNodes(ctx context.Context, workflowID uuid.UUID, projectID uuid.UUID, nodes []model.WorkflowNode) *subWorkflowRejection {
	for i := range nodes {
		node := &nodes[i]
		if node.Type != model.NodeTypeSubWorkflow {
			continue
		}
		rej := h.validateSubWorkflowNode(ctx, workflowID, projectID, node)
		if rej != nil {
			return rej
		}
	}
	return nil
}

// validateSubWorkflowNode validates a single sub_workflow node's config.
func (h *WorkflowHandler) validateSubWorkflowNode(ctx context.Context, workflowID uuid.UUID, projectID uuid.UUID, node *model.WorkflowNode) *subWorkflowRejection {
	var cfg struct {
		TargetKind            string `json:"targetKind"`
		TargetKindSnake       string `json:"target_kind"`
		TargetWorkflowID      string `json:"targetWorkflowId"`
		TargetWorkflowIDSnake string `json:"target_workflow_id"`
		WorkflowID            string `json:"workflowId"`
	}
	if len(node.Config) > 0 {
		_ = json.Unmarshal(node.Config, &cfg)
	}

	kind := strings.ToLower(strings.TrimSpace(cfg.TargetKind))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(cfg.TargetKindSnake))
	}
	if kind == "" {
		kind = "dag"
	}
	if kind != "dag" && kind != "plugin" {
		return &subWorkflowRejection{
			Error:  fmt.Sprintf("sub_workflow node %q declares unknown targetKind %q", node.ID, kind),
			Reason: subWorkflowRejectInvalidTargetKind,
			NodeID: node.ID,
		}
	}

	target := strings.TrimSpace(cfg.TargetWorkflowID)
	if target == "" {
		target = strings.TrimSpace(cfg.TargetWorkflowIDSnake)
	}
	if target == "" {
		target = strings.TrimSpace(cfg.WorkflowID)
	}
	if target == "" {
		return &subWorkflowRejection{
			Error:  fmt.Sprintf("sub_workflow node %q is missing targetWorkflowId", node.ID),
			Reason: subWorkflowRejectMissingTarget,
			NodeID: node.ID,
		}
	}

	if kind == "dag" {
		targetID, err := uuid.Parse(target)
		if err != nil {
			return &subWorkflowRejection{
				Error:  fmt.Sprintf("sub_workflow node %q has invalid DAG target %q: %v", node.ID, target, err),
				Reason: subWorkflowRejectUnknownTarget,
				NodeID: node.ID,
				Target: target,
			}
		}
		if workflowID != uuid.Nil && targetID == workflowID {
			return &subWorkflowRejection{
				Error:  fmt.Sprintf("sub_workflow node %q targets its own workflow", node.ID),
				Reason: subWorkflowRejectTrivialSelfLoop,
				NodeID: node.ID,
				Target: target,
			}
		}
		if h.dagDefRepo != nil {
			def, err := h.dagDefRepo.GetByID(ctx, targetID)
			if err != nil || def == nil {
				return &subWorkflowRejection{
					Error:  fmt.Sprintf("sub_workflow node %q targets unknown DAG workflow %s", node.ID, target),
					Reason: subWorkflowRejectUnknownTarget,
					NodeID: node.ID,
					Target: target,
				}
			}
			if def.ProjectID != projectID {
				return &subWorkflowRejection{
					Error:  fmt.Sprintf("sub_workflow node %q targets cross-project workflow %s", node.ID, target),
					Reason: subWorkflowRejectCrossProject,
					NodeID: node.ID,
					Target: target,
				}
			}
		}
	}
	// Plugin-kind targets intentionally skip deep resolution here — the applier
	// rejects unknown plugins and cross-project plugins at dispatch time with
	// the same reason taxonomy.
	return nil
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
			Triggers:    make([]model.TaskWorkflowTrigger, 0),
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

	// Validate sub_workflow node targets before persisting. Same-project,
	// resolvable, no trivial self-loop (uuid.Nil is passed for workflowID
	// because the def doesn't exist yet — the self-loop check degrades to a
	// no-op on create, which is correct: a brand-new workflow can't target
	// itself yet).
	if rej := h.validateSubWorkflowNodes(c.Request().Context(), uuid.Nil, projectID, req.Nodes); rej != nil {
		return c.JSON(http.StatusBadRequest, rej)
	}

	if err := h.dagDefRepo.Create(c.Request().Context(), def); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateWorkflow)
	}

	// Sync trigger subscriptions from the saved definition's nodes.
	if h.triggerSyncer != nil {
		var nodes []model.WorkflowNode
		if len(def.Nodes) > 0 {
			if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
				// Don't fail the create over this — log and continue.
				// Returning 500 after a successful DB insert would leave the
				// caller with an orphan workflow they can't easily discover.
				c.Logger().Errorf("workflow %s: parse nodes for trigger sync: %v", def.ID, err)
			}
		}
		// TODO: thread createdBy from auth context
		if _, syncErr := h.triggerSyncer.SyncFromDefinition(c.Request().Context(), def.ID, def.ProjectID, nodes, nil); syncErr != nil {
			c.Logger().Errorf("workflow %s: sync triggers after create: %v", def.ID, syncErr)
			// Same rationale: the workflow was created; trigger sync can be
			// retried by re-saving. Don't roll back.
		}
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

	// Validate sub_workflow node targets on node-set changes. The current
	// workflow's project id must be looked up so cross-project checks use the
	// persistent project, not a caller-supplied override.
	if req.Nodes != nil {
		existing, lookupErr := h.dagDefRepo.GetByID(c.Request().Context(), id)
		if lookupErr != nil {
			return localizedError(c, http.StatusNotFound, i18n.MsgWorkflowNotFound)
		}
		if rej := h.validateSubWorkflowNodes(c.Request().Context(), id, existing.ProjectID, *req.Nodes); rej != nil {
			return c.JSON(http.StatusBadRequest, rej)
		}
	}

	if err := h.dagDefRepo.Update(c.Request().Context(), id, updates); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateWorkflow)
	}

	updated, err := h.dagDefRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWorkflowNotFound)
	}

	// Only sync triggers when the node set changed; metadata-only updates skip sync.
	if h.triggerSyncer != nil && req.Nodes != nil {
		var nodes []model.WorkflowNode
		if len(updated.Nodes) > 0 {
			if err := json.Unmarshal(updated.Nodes, &nodes); err != nil {
				// Don't fail the update over this — log and continue.
				c.Logger().Errorf("workflow %s: parse nodes for trigger sync: %v", updated.ID, err)
			}
		}
		// TODO: thread createdBy from auth context
		if _, syncErr := h.triggerSyncer.SyncFromDefinition(c.Request().Context(), updated.ID, updated.ProjectID, nodes, nil); syncErr != nil {
			c.Logger().Errorf("workflow %s: sync triggers after update: %v", updated.ID, syncErr)
			// Trigger sync can be retried by re-saving. Don't roll back.
		}
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

	exec, err := h.dagSvc.StartExecution(c.Request().Context(), id, taskID, service.StartOptions{})
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

	// Resolve sub-workflow linkage when a reader is wired:
	//   - subInvocations: parent-link rows originating at this execution
	//     (this exec started zero or more child runs).
	//   - invokedByParent: single parent-link row where this exec is the child
	//     (this exec was started by a parent's sub_workflow node).
	// Missing reader is a graceful no-op so legacy deployments still respond.
	var subInvocations []model.WorkflowRunParentLinkDTO
	var invokedByParent *model.WorkflowRunParentLinkDTO
	if h.linkReader != nil {
		ctx := c.Request().Context()
		if links, listErr := h.linkReader.ListByParentExecution(ctx, id); listErr == nil {
			subInvocations = make([]model.WorkflowRunParentLinkDTO, 0, len(links))
			for _, l := range links {
				if l == nil {
					continue
				}
				subInvocations = append(subInvocations, l.ToDTO())
			}
		}
		if parentLink, getErr := h.linkReader.GetByChild(ctx, model.SubWorkflowEngineDAG, id); getErr == nil && parentLink != nil {
			dto := parentLink.ToDTO()
			invokedByParent = &dto
		}
	}

	// Also include node executions if available
	if h.dagNodeRepo != nil {
		nodeExecs, err := h.dagNodeRepo.ListNodeExecutions(c.Request().Context(), id)
		if err == nil {
			nodeExecDTOs := make([]model.WorkflowNodeExecutionDTO, len(nodeExecs))
			for i, ne := range nodeExecs {
				nodeExecDTOs[i] = ne.ToDTO()
			}
			return c.JSON(http.StatusOK, map[string]any{
				"execution":       execDTO,
				"nodeExecutions":  nodeExecDTOs,
				"subInvocations":  subInvocations,
				"invokedByParent": invokedByParent,
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"execution":       execDTO,
		"subInvocations":  subInvocations,
		"invokedByParent": invokedByParent,
	})
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
	projectID := appMiddleware.GetProjectID(c)
	query := c.QueryParam("q")
	category := c.QueryParam("category")
	source := c.QueryParam("source")
	templates, err := h.templateSvc.ListTemplates(c.Request().Context(), projectID, query, category, source)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	result := make([]model.WorkflowDefinitionDTO, len(templates))
	for i, t := range templates {
		result[i] = t.ToDTO()
	}
	return c.JSON(http.StatusOK, result)
}

// PublishTemplate creates a project-owned reusable template from a workflow definition.
func (h *WorkflowHandler) PublishTemplate(c echo.Context) error {
	if h.templateSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	definitionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}
	projectID := appMiddleware.GetProjectID(c)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = c.Bind(&body)

	template, err := h.templateSvc.PublishDefinitionAsTemplate(c.Request().Context(), definitionID, projectID, body.Name, body.Description)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, template.ToDTO())
}

// DuplicateTemplate creates a project-owned custom template copy from an existing template.
func (h *WorkflowHandler) DuplicateTemplate(c echo.Context) error {
	if h.templateSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}
	projectID := appMiddleware.GetProjectID(c)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = c.Bind(&body)

	template, err := h.templateSvc.DuplicateTemplate(c.Request().Context(), templateID, projectID, body.Name, body.Description)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, template.ToDTO())
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

// DeleteTemplate deletes a project-owned custom template.
func (h *WorkflowHandler) DeleteTemplate(c echo.Context) error {
	if h.templateSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}
	projectID := appMiddleware.GetProjectID(c)
	if err := h.templateSvc.DeleteTemplate(c.Request().Context(), templateID, projectID); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
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
// Used by IM card callbacks waking wait_event nodes and by future webhook receivers.
func (h *WorkflowHandler) HandleExternalEvent(c echo.Context) error {
	if h.dagSvc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDAGWorkflowServiceUnavailable)
	}
	executionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidExecutionID)
	}
	var body struct {
		NodeID  string          `json:"nodeId"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := c.Bind(&body); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	// NodeID validation
	if body.NodeID == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowNodeID)
	}

	// Default empty payload to {}
	if len(body.Payload) == 0 {
		body.Payload = json.RawMessage("{}")
	}

	if err := h.dagSvc.HandleExternalEvent(c.Request().Context(), executionID, body.NodeID, body.Payload); err != nil {
		c.Logger().Errorf("HandleExternalEvent exec=%s node=%s: %v", executionID, body.NodeID, err)
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToHandleWorkflowEvent)
	}
	return c.NoContent(http.StatusAccepted)
}
