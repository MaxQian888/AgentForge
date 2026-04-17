package handler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
)

type imService interface {
	HandleIncoming(ctx context.Context, req *model.IMMessageRequest) (*model.IMMessageResponse, error)
	HandleCommand(ctx context.Context, req *model.IMCommandRequest) (*model.IMCommandResponse, error)
	HandleIntent(ctx context.Context, req *model.IMIntentRequest) (*model.IMIntentResponse, error)
	HandleAction(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error)
	HandleReaction(ctx context.Context, req *model.IMReactionRequest) error
	BindReactionShortcut(ctx context.Context, req *model.IMReactionShortcutBinding) error
	Send(ctx context.Context, req *model.IMSendRequest) error
	Notify(ctx context.Context, req *model.IMNotifyRequest) error
}

type IMHandler struct {
	service imService
}

func NewIMHandler(svc imService) *IMHandler {
	return &IMHandler{service: svc}
}

func (h *IMHandler) HandleMessage(c echo.Context) error {
	req := new(model.IMMessageRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.service.HandleIncoming(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *IMHandler) HandleCommand(c echo.Context) error {
	req := new(model.IMCommandRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.service.HandleCommand(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *IMHandler) Send(c echo.Context) error {
	req := new(model.IMSendRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	if err := h.service.Send(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "sent"})
}

func (h *IMHandler) Notify(c echo.Context) error {
	req := new(model.IMNotifyRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	if err := h.service.Notify(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "notification sent"})
}

func (h *IMHandler) HandleAction(c echo.Context) error {
	req := new(model.IMActionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.service.HandleAction(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *IMHandler) HandleIntent(c echo.Context) error {
	req := new(model.IMIntentRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	resp, err := h.service.HandleIntent(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

// HandleReaction persists an inbound reaction event forwarded by the IM
// bridge. The service layer decides whether the reaction also triggers a
// review shortcut.
func (h *IMHandler) HandleReaction(c echo.Context) error {
	req := new(model.IMReactionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.HandleReaction(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusAccepted, map[string]string{"status": "recorded"})
}

// BindReactionShortcut registers a mapping from a unified emoji code on a
// reply target to a review decision.
func (h *IMHandler) BindReactionShortcut(c echo.Context) error {
	req := new(model.IMReactionShortcutBinding)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if err := h.service.BindReactionShortcut(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]string{"status": "bound"})
}
