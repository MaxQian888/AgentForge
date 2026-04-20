package handler

import (
	"errors"
	"net/http"

	"github.com/agentforge/server/internal/imcards"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type IMCardActionsHandler struct{ Router *imcards.Router }

func NewIMCardActionsHandler(r *imcards.Router) *IMCardActionsHandler {
	return &IMCardActionsHandler{Router: r}
}

type imCardActionRequest struct {
	CorrelationToken string         `json:"correlation_token"`
	ActionID         string         `json:"action_id"`
	Value            map[string]any `json:"value"`
	ReplyTarget      map[string]any `json:"replyTarget"`
	UserID           string         `json:"user_id"`
	TenantID         string         `json:"tenant_id"`
}

type imCardActionResponse struct {
	Outcome     string `json:"outcome"`
	ExecutionID string `json:"execution_id,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
}

// Handle is the single Echo handler for POST /api/v1/im/card-actions.
// Status mapping mirrors the spec error matrix:
// 410 expired, 409 consumed/not-waiting, 200 resumed/fallback.
func (h *IMCardActionsHandler) Handle(c echo.Context) error {
	var req imCardActionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}
	token, err := uuid.Parse(req.CorrelationToken)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid correlation_token",
		})
	}
	out, err := h.Router.Route(c.Request().Context(), imcards.RouteInput{
		Token:       token,
		ActionID:    req.ActionID,
		Value:       req.Value,
		ReplyTarget: req.ReplyTarget,
		UserID:      req.UserID,
		TenantID:    req.TenantID,
	})
	switch {
	case errors.Is(err, imcards.ErrCardActionExpired):
		return c.JSON(http.StatusGone, map[string]string{
			"code": "card_action:expired",
		})
	case errors.Is(err, imcards.ErrCardActionConsumed):
		return c.JSON(http.StatusConflict, map[string]string{
			"code": "card_action:consumed",
		})
	case errors.Is(err, imcards.ErrExecutionNotWaiting):
		return c.JSON(http.StatusConflict, map[string]string{
			"code": "card_action:execution_not_waiting",
		})
	case err != nil:
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	resp := imCardActionResponse{Outcome: string(out.Outcome)}
	if out.ExecutionID != uuid.Nil {
		resp.ExecutionID = out.ExecutionID.String()
	}
	resp.NodeID = out.NodeID
	return c.JSON(http.StatusOK, resp)
}
