package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type imControlPlane interface {
	RegisterBridge(ctx context.Context, req *model.IMBridgeRegisterRequest) (*model.IMBridgeInstance, error)
	RecordHeartbeat(ctx context.Context, bridgeID string, metadata map[string]string) (*model.IMBridgeHeartbeatResponse, error)
	UnregisterBridge(ctx context.Context, bridgeID string) error
	BindAction(ctx context.Context, binding *model.IMActionBinding) error
	AckDelivery(ctx context.Context, ack *model.IMDeliveryAck) error
	ListChannels(ctx context.Context) ([]*model.IMChannel, error)
	UpsertChannel(ctx context.Context, channel *model.IMChannel) (*model.IMChannel, error)
	DeleteChannel(ctx context.Context, channelID string) error
	GetBridgeStatus(ctx context.Context) (*model.IMBridgeStatus, error)
	ListDeliveryHistory(ctx context.Context, filters *model.IMDeliveryHistoryFilters) ([]*model.IMDelivery, error)
	ListEventTypes(ctx context.Context) ([]string, error)
	RetryDelivery(ctx context.Context, deliveryID string) (*model.IMDelivery, error)
}

func cloneIMStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

type imControlSender interface {
	Send(ctx context.Context, req *model.IMSendRequest) error
}

type IMControlHandler struct {
	control imControlPlane
	sender  imControlSender
}

func NewIMControlHandler(control imControlPlane, sender ...imControlSender) *IMControlHandler {
	handler := &IMControlHandler{control: control}
	if len(sender) > 0 {
		handler.sender = sender[0]
	}
	return handler
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
		BridgeID string            `json:"bridgeId"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}{}
	if err := c.Bind(&req); err != nil {
		log.WithField("remoteAddr", c.RealIP()).WithError(err).Warn("IM control heartbeat rejected: invalid request body")
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	resp, err := h.control.RecordHeartbeat(c.Request().Context(), req.BridgeID, req.Metadata)
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
	if err := h.control.AckDelivery(c.Request().Context(), req); err != nil {
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
	filters := &model.IMDeliveryHistoryFilters{
		DeliveryID: strings.TrimSpace(c.QueryParam("deliveryId")),
		Status:     strings.TrimSpace(c.QueryParam("status")),
		Platform:   strings.TrimSpace(c.QueryParam("platform")),
		EventType:  strings.TrimSpace(c.QueryParam("eventType")),
		Kind:       strings.TrimSpace(c.QueryParam("kind")),
		Since:      strings.TrimSpace(c.QueryParam("since")),
	}
	history, err := h.control.ListDeliveryHistory(c.Request().Context(), filters)
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

func (h *IMControlHandler) RetryBatchDeliveries(c echo.Context) error {
	req := new(model.IMRetryBatchRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	results := make([]model.IMRetryBatchItemResult, 0, len(req.DeliveryIDs))
	for _, deliveryID := range req.DeliveryIDs {
		trimmed := strings.TrimSpace(deliveryID)
		if trimmed == "" {
			continue
		}
		delivery, err := h.control.RetryDelivery(c.Request().Context(), trimmed)
		if err != nil {
			results = append(results, model.IMRetryBatchItemResult{
				DeliveryID: trimmed,
				Status:     model.IMDeliveryStatus("rejected"),
				Message:    err.Error(),
			})
			continue
		}
		results = append(results, model.IMRetryBatchItemResult{
			DeliveryID: trimmed,
			Status:     delivery.Status,
		})
	}
	return c.JSON(http.StatusOK, model.IMRetryBatchResponse{Results: results})
}

func (h *IMControlHandler) TestSend(c echo.Context) error {
	if h.sender == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "IM send service unavailable"})
	}
	req := new(model.IMTestSendRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	deliveryID := strings.TrimSpace(req.DeliveryID)
	if deliveryID == "" {
		platformSlug := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(req.Platform))), "-")
		if platformSlug == "" {
			platformSlug = "im"
		}
		deliveryID = "test-send-" + platformSlug + "-" + uuid.NewString()
	}
	sendReq := &model.IMSendRequest{
		Platform:   strings.TrimSpace(req.Platform),
		ChannelID:  strings.TrimSpace(req.ChannelID),
		Text:       strings.TrimSpace(req.Text),
		ProjectID:  strings.TrimSpace(req.ProjectID),
		BridgeID:   strings.TrimSpace(req.BridgeID),
		DeliveryID: deliveryID,
		Metadata:   cloneIMStringMap(req.Metadata),
	}
	if sendReq.Metadata == nil {
		sendReq.Metadata = map[string]string{}
	}
	sendReq.Metadata["operator_test"] = "true"

	if err := h.sender.Send(c.Request().Context(), sendReq); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}

	result := model.IMTestSendResponse{
		DeliveryID: deliveryID,
		Status:     model.IMDeliveryStatusPending,
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 1500*time.Millisecond)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		history, err := h.control.ListDeliveryHistory(ctx, &model.IMDeliveryHistoryFilters{DeliveryID: deliveryID})
		if err == nil && len(history) > 0 {
			delivery := history[0]
			result.Status = delivery.Status
			result.FailureReason = delivery.FailureReason
			result.DowngradeReason = delivery.DowngradeReason
			result.ProcessedAt = delivery.ProcessedAt
			result.LatencyMs = delivery.LatencyMs
			if delivery.Status != model.IMDeliveryStatusPending {
				break
			}
		}
		select {
		case <-ctx.Done():
			return c.JSON(http.StatusOK, result)
		case <-ticker.C:
		}
	}
	return c.JSON(http.StatusOK, result)
}
