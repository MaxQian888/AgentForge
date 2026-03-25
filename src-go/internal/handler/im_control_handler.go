package handler

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	log "github.com/sirupsen/logrus"
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
		log.WithField("remoteAddr", c.RealIP()).WithError(err).Warn("IM control register rejected: invalid request body")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	instance, err := h.control.RegisterBridge(c.Request().Context(), req)
	if err != nil {
		log.WithFields(log.Fields{
			"remoteAddr": c.RealIP(),
			"bridgeId":   req.BridgeID,
			"platform":   req.Platform,
			"transport":  req.Transport,
		}).WithError(err).Warn("IM control register failed")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr":   c.RealIP(),
		"bridgeId":     instance.BridgeID,
		"platform":     instance.Platform,
		"transport":    instance.Transport,
		"projectCount": len(instance.ProjectIDs),
	}).Info("IM control bridge registered")
	return c.JSON(http.StatusCreated, instance)
}

func (h *IMControlHandler) Heartbeat(c echo.Context) error {
	req := struct {
		BridgeID string `json:"bridgeId"`
	}{}
	if err := c.Bind(&req); err != nil {
		log.WithField("remoteAddr", c.RealIP()).WithError(err).Warn("IM control heartbeat rejected: invalid request body")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	resp, err := h.control.RecordHeartbeat(c.Request().Context(), req.BridgeID)
	if err != nil {
		log.WithFields(log.Fields{
			"remoteAddr": c.RealIP(),
			"bridgeId":   req.BridgeID,
		}).WithError(err).Warn("IM control heartbeat failed")
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr": c.RealIP(),
		"bridgeId":   resp.BridgeID,
		"status":     resp.Status,
		"expiresAt":  resp.ExpiresAt,
	}).Debug("IM control heartbeat recorded")
	return c.JSON(http.StatusOK, resp)
}

func (h *IMControlHandler) Unregister(c echo.Context) error {
	req := struct {
		BridgeID string `json:"bridgeId"`
	}{}
	if err := c.Bind(&req); err != nil {
		log.WithField("remoteAddr", c.RealIP()).WithError(err).Warn("IM control unregister rejected: invalid request body")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.control.UnregisterBridge(c.Request().Context(), req.BridgeID); err != nil {
		log.WithFields(log.Fields{
			"remoteAddr": c.RealIP(),
			"bridgeId":   req.BridgeID,
		}).WithError(err).Warn("IM control unregister failed")
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr": c.RealIP(),
		"bridgeId":   req.BridgeID,
	}).Info("IM control bridge unregistered")
	return c.JSON(http.StatusOK, map[string]string{"message": "bridge unregistered"})
}

func (h *IMControlHandler) BindAction(c echo.Context) error {
	req := new(model.IMActionBinding)
	if err := c.Bind(req); err != nil {
		log.WithField("remoteAddr", c.RealIP()).WithError(err).Warn("IM control bind action rejected: invalid request body")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.control.BindAction(c.Request().Context(), req); err != nil {
		log.WithFields(log.Fields{
			"remoteAddr": c.RealIP(),
			"bridgeId":   req.BridgeID,
			"platform":   req.Platform,
			"projectId":  req.ProjectID,
			"taskId":     req.TaskID,
			"runId":      req.RunID,
			"reviewId":   req.ReviewID,
		}).WithError(err).Warn("IM control bind action failed")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr": c.RealIP(),
		"bridgeId":   req.BridgeID,
		"platform":   req.Platform,
		"projectId":  req.ProjectID,
		"taskId":     req.TaskID,
		"runId":      req.RunID,
		"reviewId":   req.ReviewID,
	}).Info("IM control action binding stored")
	return c.JSON(http.StatusCreated, map[string]string{"message": "binding stored"})
}

func (h *IMControlHandler) AckDelivery(c echo.Context) error {
	req := new(model.IMDeliveryAck)
	if err := c.Bind(req); err != nil {
		log.WithField("remoteAddr", c.RealIP()).WithError(err).Warn("IM control delivery ack rejected: invalid request body")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.control.AckDelivery(c.Request().Context(), req.BridgeID, req.Cursor, req.DeliveryID); err != nil {
		log.WithFields(log.Fields{
			"remoteAddr": c.RealIP(),
			"bridgeId":   req.BridgeID,
			"cursor":     req.Cursor,
			"deliveryId": req.DeliveryID,
		}).WithError(err).Warn("IM control delivery ack failed")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr": c.RealIP(),
		"bridgeId":   req.BridgeID,
		"cursor":     req.Cursor,
		"deliveryId": req.DeliveryID,
	}).Debug("IM control delivery acknowledged")
	return c.JSON(http.StatusOK, map[string]string{"message": "delivery acknowledged"})
}
