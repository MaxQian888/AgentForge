package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/trigger"
)

// triggerRouter is the subset of *trigger.Router that the handler uses.
// Keeping this local keeps the handler unit-testable with a mock.
type triggerRouter interface {
	Route(ctx context.Context, ev trigger.Event) (int, error)
	RouteWithOutcomes(ctx context.Context, ev trigger.Event) ([]trigger.Outcome, error)
}

// triggerQueryRepo is the read/toggle subset of the trigger repository that
// the read-side endpoints consume.
type triggerQueryRepo interface {
	ListByWorkflow(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowTrigger, error)
	SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error
}

// triggerCRUDService is the orchestration seam consumed by the Spec 1C
// CRUD endpoints (POST/PATCH/DELETE /api/v1/triggers and the per-employee
// list + dry-run). Satisfied in production by *trigger.CRUDService.
type triggerCRUDService interface {
	Create(ctx context.Context, in trigger.CreateTriggerInput) (*model.WorkflowTrigger, error)
	Patch(ctx context.Context, id uuid.UUID, in trigger.PatchTriggerInput) (*model.WorkflowTrigger, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByEmployee(ctx context.Context, employeeID uuid.UUID) ([]*model.WorkflowTrigger, error)
	Test(ctx context.Context, id uuid.UUID, event map[string]any) (*trigger.DryRunResult, error)
}

// TriggerHandler handles incoming trigger events (IM commands, webhooks, etc.)
// and dispatches them to matching workflow triggers via the Router.
type TriggerHandler struct {
	router triggerRouter
	repo   triggerQueryRepo
	crud   triggerCRUDService
}

// NewTriggerHandler constructs a TriggerHandler backed by the given router.
func NewTriggerHandler(router triggerRouter) *TriggerHandler {
	return &TriggerHandler{router: router}
}

// WithQueryRepo wires the trigger repository for list/toggle endpoints.
// Nil repo disables those routes but leaves /im/events functional.
func (h *TriggerHandler) WithQueryRepo(repo triggerQueryRepo) *TriggerHandler {
	h.repo = repo
	return h
}

// WithCRUDService wires the spec1-1C CRUD orchestration seam.
// When set, the handler exposes POST/PATCH/DELETE /api/v1/triggers,
// GET /api/v1/employees/:employeeId/triggers, and POST /api/v1/triggers/:id/test.
// Nil service leaves those routes unregistered.
func (h *TriggerHandler) WithCRUDService(svc triggerCRUDService) *TriggerHandler {
	h.crud = svc
	return h
}

// RegisterRoutes registers the trigger endpoints on the Echo instance.
func (h *TriggerHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/triggers")
	g.POST("/im/events", h.HandleIMEvent)
	if h.repo != nil {
		// Deprecated: SetEnabled is kept for legacy callers that have not yet
		// migrated to PATCH /api/v1/triggers/:id with {"enabled": ...}. The
		// FE no longer calls this endpoint after spec1-1C; remove in a follow-
		// up once external IM bridges have been audited.
		g.POST("/:id/enabled", h.SetEnabled)
		e.GET("/api/v1/workflows/:workflowId/triggers", h.ListByWorkflow)
	}
	if h.crud != nil {
		g.POST("", h.Create)
		g.PATCH("/:id", h.Patch)
		g.DELETE("/:id", h.Delete)
		g.POST("/:id/test", h.Test)
		e.GET("/api/v1/employees/:employeeId/triggers", h.ListByEmployee)
	}
}

// imEventRequest is the normalized IM event payload the IM Bridge POSTs.
type imEventRequest struct {
	Platform    string          `json:"platform"`              // feishu, slack, discord, etc.
	Command     string          `json:"command"`               // e.g. "/review"
	Content     string          `json:"content"`               // full message body for match_regex
	Args        []any           `json:"args"`                  // parsed command arguments
	ChatID      string          `json:"chatId"`                // chat scope for allowlists
	ThreadID    string          `json:"threadId,omitempty"`
	UserID      string          `json:"userId,omitempty"`
	UserName    string          `json:"userName,omitempty"`
	TenantID    string          `json:"tenantId,omitempty"`
	ReplyTarget json.RawMessage `json:"replyTarget,omitempty"` // bridge reply context for workflow completion
	MessageID   string          `json:"messageId,omitempty"`   // used as default idempotency key

	// Passthrough for platform-specific extras. Router templates can read
	// this via {{$event.extra.*}} if needed.
	Extra map[string]any `json:"extra,omitempty"`
}

// HandleIMEvent receives a normalized IM event from the IM Bridge and routes it
// to matching workflow triggers. Returns 202 with {"started": N} on success,
// 404 when no trigger matched, or 500 when routing failed without any execution
// starting.
func (h *TriggerHandler) HandleIMEvent(c echo.Context) error {
	if h.router == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	var req imEventRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Platform == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgMissingIMPlatform)
	}

	// Compose the event data map the Router matchers + templates consume.
	data := map[string]any{
		"platform":     req.Platform,
		"command":      req.Command,
		"content":      req.Content,
		"args":         req.Args,
		"chat_id":      req.ChatID,
		"thread_id":    req.ThreadID,
		"user_id":      req.UserID,
		"user_name":    req.UserName,
		"tenant_id":    req.TenantID,
		"message_id":   req.MessageID,
		"reply_target": req.ReplyTarget, // stays json.RawMessage — passthrough
	}
	if len(req.Extra) > 0 {
		data["extra"] = req.Extra
	}

	outcomes, err := h.router.RouteWithOutcomes(c.Request().Context(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   data,
	})
	started := 0
	for _, o := range outcomes {
		if o.Status == trigger.OutcomeStarted {
			started++
		}
	}
	if err != nil {
		c.Logger().Errorf("trigger router: im event dispatch: %v", err)
		// Partial success is possible — err is the last error but started > 0
		// means at least one execution did start.  Return 500 only when NO
		// execution started and an error occurred.
		if started == 0 {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRouteIMEvent)
		}
	}
	if len(outcomes) == 0 {
		// No matching trigger; not an error but the bridge needs to know
		// so it can reply "unknown command" to the user.
		return c.JSON(http.StatusNotFound, map[string]any{
			"started":  0,
			"outcomes": []trigger.Outcome{},
			"message":  "no matching workflow trigger",
		})
	}
	return c.JSON(http.StatusAccepted, map[string]any{
		"started":  started,
		"outcomes": outcomes,
	})
}

// ListByWorkflow returns the materialized workflow_triggers rows for a workflow.
// Used by the frontend Triggers tab.
func (h *TriggerHandler) ListByWorkflow(c echo.Context) error {
	if h.repo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidWorkflowID)
	}
	triggers, err := h.repo.ListByWorkflow(c.Request().Context(), workflowID)
	if err != nil {
		c.Logger().Errorf("list triggers by workflow: %v", err)
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRouteIMEvent)
	}
	if triggers == nil {
		triggers = []*model.WorkflowTrigger{}
	}
	return c.JSON(http.StatusOK, triggers)
}

// SetEnabled toggles the enabled flag on a single trigger row.
// Body: {"enabled": bool}.
//
// Deprecated: spec1-1C migrates the FE to PATCH /api/v1/triggers/:id
// {"enabled": ...}. This endpoint is retained for legacy IM-bridge callers
// only; new code should use the CRUD service path.
func (h *TriggerHandler) SetEnabled(c echo.Context) error {
	if h.repo == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&body); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := h.repo.SetEnabled(c.Request().Context(), id, body.Enabled); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		c.Logger().Errorf("set trigger enabled: %v", err)
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRouteIMEvent)
	}
	return c.NoContent(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Spec 1C — Trigger CRUD endpoints
// ---------------------------------------------------------------------------

// createTriggerRequest is the typed body shape POST /api/v1/triggers accepts.
// All fields are JSON-decoded as raw values; the handler converts to the
// CreateTriggerInput struct after validation.
type createTriggerRequest struct {
	WorkflowID       string          `json:"workflowId"`
	Source           string          `json:"source"`
	Config           json.RawMessage `json:"config"`
	InputMapping     json.RawMessage `json:"inputMapping"`
	ActingEmployeeID *string         `json:"actingEmployeeId,omitempty"`
	DisplayName      string          `json:"displayName"`
	Description      string          `json:"description"`
}

// Create POST /api/v1/triggers — author a new manual trigger row.
func (h *TriggerHandler) Create(c echo.Context) error {
	if h.crud == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	var req createTriggerRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	wfID, err := uuid.Parse(req.WorkflowID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"code":    "trigger:invalid_workflow_id",
			"message": "workflowId must be a valid UUID",
		})
	}
	source := model.TriggerSource(req.Source)
	if source != model.TriggerSourceIM && source != model.TriggerSourceSchedule {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"code":    "trigger:invalid_source",
			"message": "source must be 'im' or 'schedule'",
		})
	}
	in := trigger.CreateTriggerInput{
		WorkflowID:   wfID,
		Source:       source,
		Config:       req.Config,
		InputMapping: req.InputMapping,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
	}
	if req.ActingEmployeeID != nil && *req.ActingEmployeeID != "" {
		empID, err := uuid.Parse(*req.ActingEmployeeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"code":    "trigger:invalid_acting_employee_id",
				"message": "actingEmployeeId must be a valid UUID",
			})
		}
		in.ActingEmployeeID = &empID
	}
	row, err := h.crud.Create(c.Request().Context(), in)
	if err != nil {
		return mapTriggerErr(c, err)
	}
	return c.JSON(http.StatusCreated, row)
}

// patchTriggerRequest carries only the fields the FE may modify. workflowId,
// source, and createdVia are intentionally absent; if a client sends them
// they are silently ignored at JSON-decode time and the handler responds
// with a contract violation if they appear in the raw body.
type patchTriggerRequest struct {
	Config           json.RawMessage  `json:"config,omitempty"`
	InputMapping     json.RawMessage  `json:"inputMapping,omitempty"`
	ActingEmployeeID *string          `json:"actingEmployeeId,omitempty"`
	DisplayName      *string          `json:"displayName,omitempty"`
	Description      *string          `json:"description,omitempty"`
	Enabled          *bool            `json:"enabled,omitempty"`

	// Disallowed fields — present here purely to detect clients sending them
	// so we can return a clean 400 instead of silently ignoring.
	WorkflowID *string `json:"workflowId,omitempty"`
	Source     *string `json:"source,omitempty"`
	CreatedVia *string `json:"createdVia,omitempty"`
}

// Patch PATCH /api/v1/triggers/:id — update a manual trigger row.
func (h *TriggerHandler) Patch(c echo.Context) error {
	if h.crud == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	var req patchTriggerRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.WorkflowID != nil || req.Source != nil || req.CreatedVia != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"code":    "trigger:immutable_field",
			"message": "workflowId, source, and createdVia are immutable; create a new trigger instead",
		})
	}
	in := trigger.PatchTriggerInput{
		Config:       req.Config,
		InputMapping: req.InputMapping,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		Enabled:      req.Enabled,
	}
	if req.ActingEmployeeID != nil {
		in.IncludeActingEmployeeID = true
		if *req.ActingEmployeeID != "" {
			empID, err := uuid.Parse(*req.ActingEmployeeID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"code":    "trigger:invalid_acting_employee_id",
					"message": "actingEmployeeId must be a valid UUID or empty string to clear",
				})
			}
			in.ActingEmployeeID = &empID
		}
	}
	row, err := h.crud.Patch(c.Request().Context(), id, in)
	if err != nil {
		return mapTriggerErr(c, err)
	}
	return c.JSON(http.StatusOK, row)
}

// Delete DELETE /api/v1/triggers/:id — remove a manual trigger row.
func (h *TriggerHandler) Delete(c echo.Context) error {
	if h.crud == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := h.crud.Delete(c.Request().Context(), id); err != nil {
		return mapTriggerErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListByEmployee GET /api/v1/employees/:employeeId/triggers — return all
// triggers whose acting_employee_id is the given employee.
func (h *TriggerHandler) ListByEmployee(c echo.Context) error {
	if h.crud == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	empID, err := uuid.Parse(c.Param("employeeId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	rows, err := h.crud.ListByEmployee(c.Request().Context(), empID)
	if err != nil {
		c.Logger().Errorf("list triggers by employee: %v", err)
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRouteIMEvent)
	}
	return c.JSON(http.StatusOK, rows)
}

// testTriggerRequest is the body for POST /api/v1/triggers/:id/test.
type testTriggerRequest struct {
	Event map[string]any `json:"event"`
}

// Test POST /api/v1/triggers/:id/test — preview whether a sample event
// would match this trigger and what input the engine would receive. Never
// dispatches; never touches the idempotency store.
func (h *TriggerHandler) Test(c echo.Context) error {
	if h.crud == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTriggerRouterUnavailable)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	var req testTriggerRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Event == nil {
		req.Event = map[string]any{}
	}
	res, err := h.crud.Test(c.Request().Context(), id, req.Event)
	if err != nil {
		return mapTriggerErr(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

// mapTriggerErr translates trigger-package sentinels into the error-code
// responses spec1 §10 specifies. Falls back to a 500 with the localized
// router-unavailable string for unknown errors.
func mapTriggerErr(c echo.Context, err error) error {
	switch {
	case errors.Is(err, trigger.ErrTriggerWorkflowNotFound):
		return c.JSON(http.StatusBadRequest, map[string]any{
			"code":    "trigger:workflow_not_found",
			"message": "workflow does not exist",
		})
	case errors.Is(err, trigger.ErrTriggerActingEmployeeArchived):
		return c.JSON(http.StatusBadRequest, map[string]any{
			"code":    "trigger:acting_employee_archived",
			"message": "acting employee is archived, cross-project, or unknown",
		})
	case errors.Is(err, trigger.ErrTriggerCannotDeleteDAGManaged):
		return c.JSON(http.StatusConflict, map[string]any{
			"code":    "trigger:cannot_delete_dag_managed",
			"message": "this trigger is owned by a DAG node; remove it from the workflow definition instead",
		})
	case errors.Is(err, trigger.ErrTriggerNotFound):
		return c.NoContent(http.StatusNotFound)
	}
	c.Logger().Errorf("trigger crud: %v", err)
	return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRouteIMEvent)
}
