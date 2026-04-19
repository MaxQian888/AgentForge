package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/trigger"
)

// triggerRouter is the subset of *trigger.Router that the handler uses.
// Keeping this local keeps the handler unit-testable with a mock.
type triggerRouter interface {
	Route(ctx context.Context, ev trigger.Event) (int, error)
}

// TriggerHandler handles incoming trigger events (IM commands, webhooks, etc.)
// and dispatches them to matching workflow triggers via the Router.
type TriggerHandler struct {
	router triggerRouter
}

// NewTriggerHandler constructs a TriggerHandler backed by the given router.
func NewTriggerHandler(router triggerRouter) *TriggerHandler {
	return &TriggerHandler{router: router}
}

// RegisterRoutes registers the trigger endpoints on the Echo instance.
func (h *TriggerHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/triggers")
	g.POST("/im/events", h.HandleIMEvent)
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

	started, err := h.router.Route(c.Request().Context(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   data,
	})
	if err != nil {
		c.Logger().Errorf("trigger router: im event dispatch: %v", err)
		// Partial success is possible — err is the last error but started > 0
		// means at least one execution did start.  Return 500 only when NO
		// execution started and an error occurred.
		if started == 0 {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRouteIMEvent)
		}
	}
	if started == 0 {
		// No matching trigger; not an error but the bridge needs to know
		// so it can reply "unknown command" to the user.
		return c.JSON(http.StatusNotFound, map[string]any{
			"started": 0,
			"message": "no matching workflow trigger",
		})
	}
	return c.JSON(http.StatusAccepted, map[string]any{"started": started})
}
