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

// TriggerHandler handles incoming trigger events (IM commands, webhooks, etc.)
// and dispatches them to matching workflow triggers via the Router.
type TriggerHandler struct {
	router triggerRouter
	repo   triggerQueryRepo
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

// RegisterRoutes registers the trigger endpoints on the Echo instance.
func (h *TriggerHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/triggers")
	g.POST("/im/events", h.HandleIMEvent)
	if h.repo != nil {
		g.POST("/:id/enabled", h.SetEnabled)
		e.GET("/api/v1/workflows/:workflowId/triggers", h.ListByWorkflow)
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
