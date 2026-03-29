package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	log "github.com/sirupsen/logrus"
)

type imControlPlane interface {
	RegisterBridge(ctx context.Context, req *model.IMBridgeRegisterRequest) (*model.IMBridgeInstance, error)
	RecordHeartbeat(ctx context.Context, bridgeID string) (*model.IMBridgeHeartbeatResponse, error)
	UnregisterBridge(ctx context.Context, bridgeID string) error
	BindAction(ctx context.Context, binding *model.IMActionBinding) error
	AckDelivery(ctx context.Context, bridgeID string, cursor int64, deliveryID string, downgradeReason string) error
	ListChannels(ctx context.Context) ([]*model.IMChannel, error)
	UpsertChannel(ctx context.Context, channel *model.IMChannel) (*model.IMChannel, error)
	DeleteChannel(ctx context.Context, channelID string) error
	GetBridgeStatus(ctx context.Context) (*model.IMBridgeStatus, error)
	ListDeliveryHistory(ctx context.Context) ([]*model.IMDelivery, error)
	ListEventTypes(ctx context.Context) ([]string, error)
	RetryDelivery(ctx context.Context, deliveryID string) (*model.IMDelivery, error)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := h.control.AckDelivery(c.Request().Context(), req.BridgeID, req.Cursor, req.DeliveryID, req.DowngradeReason); err != nil {
		log.WithFields(log.Fields{
			"remoteAddr": c.RealIP(),
			"bridgeId":   req.BridgeID,
			"cursor":     req.Cursor,
			"deliveryId": req.DeliveryID,
		}).WithError(err).Warn("IM control delivery ack failed")
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr":      c.RealIP(),
		"bridgeId":        req.BridgeID,
		"cursor":          req.Cursor,
		"deliveryId":      req.DeliveryID,
		"downgradeReason": req.DowngradeReason,
	}).Debug("IM control delivery acknowledged")
	return c.JSON(http.StatusOK, map[string]string{"message": "delivery acknowledged"})
}

func (h *IMControlHandler) ListChannels(c echo.Context) error {
	channels, err := h.control.ListChannels(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, channels)
}

func (h *IMControlHandler) SaveChannel(c echo.Context) error {
	req := new(model.IMChannel)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		req.ID = id
	}
	channel, err := h.control.UpsertChannel(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	status := http.StatusOK
	if c.Param("id") == "" {
		status = http.StatusCreated
	}
	return c.JSON(status, channel)
}

func (h *IMControlHandler) DeleteChannel(c echo.Context) error {
	if err := h.control.DeleteChannel(c.Request().Context(), c.Param("id")); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "channel deleted"})
}

func (h *IMControlHandler) GetStatus(c echo.Context) error {
	status, err := h.control.GetBridgeStatus(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, status)
}

func (h *IMControlHandler) ListDeliveries(c echo.Context) error {
	history, err := h.control.ListDeliveryHistory(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, history)
}

func (h *IMControlHandler) ListEventTypes(c echo.Context) error {
	eventTypes, err := h.control.ListEventTypes(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, eventTypes)
}

func (h *IMControlHandler) RetryDelivery(c echo.Context) error {
	delivery, err := h.control.RetryDelivery(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, delivery)
}
