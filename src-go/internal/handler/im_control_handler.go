package handler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

type imControlPlane interface {
	RegisterBridge(ctx context.Context, req *model.IMBridgeRegisterRequest) (*model.IMBridgeInstance, error)
	RecordHeartbeat(ctx context.Context, bridgeID string) (*model.IMBridgeHeartbeatResponse, error)
	UnregisterBridge(ctx context.Context, bridgeID string) error
	BindAction(ctx context.Context, binding *model.IMActionBinding) error
	AckDelivery(ctx context.Context, bridgeID string, cursor int64, deliveryID string) error
}

type IMControlHandler struct {
	control imControlPlane
}

func NewIMControlHandler(control imControlPlane) *IMControlHandler {
	return &IMControlHandler{control: control}
}

func (h *IMControlHandler) Register(c echo.Context) error {
	req := new(model.IMBridgeRegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	instance, err := h.control.RegisterBridge(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, instance)
}

func (h *IMControlHandler) Heartbeat(c echo.Context) error {
	req := struct {
		BridgeID string `json:"bridgeId"`
	}{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	resp, err := h.control.RecordHeartbeat(c.Request().Context(), req.BridgeID)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *IMControlHandler) Unregister(c echo.Context) error {
	req := struct {
		BridgeID string `json:"bridgeId"`
	}{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.control.UnregisterBridge(c.Request().Context(), req.BridgeID); err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "bridge unregistered"})
}

func (h *IMControlHandler) BindAction(c echo.Context) error {
	req := new(model.IMActionBinding)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.control.BindAction(c.Request().Context(), req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "binding stored"})
}

func (h *IMControlHandler) AckDelivery(c echo.Context) error {
	req := new(model.IMDeliveryAck)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.control.AckDelivery(c.Request().Context(), req.BridgeID, req.Cursor, req.DeliveryID); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "delivery acknowledged"})
}
