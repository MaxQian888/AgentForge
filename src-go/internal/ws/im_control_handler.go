package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentforge/server/internal/authutil"
	"github.com/agentforge/server/internal/model"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

type IMControlPlane interface {
	AttachBridgeListener(ctx context.Context, bridgeID string, afterCursor int64, listener IMBridgeListener) ([]*model.IMControlDelivery, error)
	AckDelivery(ctx context.Context, ack *model.IMDeliveryAck) error
	DetachBridgeListener(bridgeID string)
}

type IMBridgeListener interface {
	Send(ctx context.Context, delivery *model.IMControlDelivery) error
	Close() error
}

type IMControlHandler struct {
	control      IMControlPlane
	sharedSecret string
	upgrader     websocket.Upgrader
}

func NewIMControlHandler(control IMControlPlane, sharedSecret string, allowedOrigins []string) *IMControlHandler {
	return &IMControlHandler{
		control:      control,
		sharedSecret: strings.TrimSpace(sharedSecret),
		upgrader:     newUpgrader(allowedOrigins),
	}
}

func (h *IMControlHandler) HandleWS(c echo.Context) error {
	bridgeID := c.QueryParam("bridgeId")
	remoteAddr := c.RealIP()
	if bridgeID == "" {
		log.WithField("remoteAddr", remoteAddr).Warn("IM control ws rejected: missing bridgeId")
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "missing bridgeId"})
	}

	afterCursor := int64(0)
	if raw := c.QueryParam("afterCursor"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			log.WithFields(log.Fields{
				"remoteAddr":  remoteAddr,
				"bridgeId":    bridgeID,
				"afterCursor": raw,
			}).WithError(err).Warn("IM control ws rejected: invalid afterCursor")
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "invalid afterCursor"})
		}
		afterCursor = parsed
	}

	if err := authutil.ValidateBearerSharedSecret(c.Request().Header.Get("Authorization"), h.sharedSecret); err != nil {
		entry := log.WithFields(log.Fields{
			"remoteAddr":  remoteAddr,
			"bridgeId":    bridgeID,
			"afterCursor": afterCursor,
		}).WithError(err)
		if errors.Is(err, authutil.ErrSharedSecretNotConfigured) {
			entry.Warn("IM control ws rejected: auth misconfigured")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "IM control auth unavailable"})
		}
		entry.Warn("IM control ws rejected: unauthorized")
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
	}

	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.WithFields(log.Fields{
			"remoteAddr":  remoteAddr,
			"bridgeId":    bridgeID,
			"afterCursor": afterCursor,
		}).WithError(err).Error("IM control ws upgrade failed")
		return err
	}
	log.WithFields(log.Fields{
		"remoteAddr":  remoteAddr,
		"bridgeId":    bridgeID,
		"afterCursor": afterCursor,
	}).Info("IM control ws connected")

	listener := &imBridgeWSListener{conn: conn}
	replayed, err := h.control.AttachBridgeListener(c.Request().Context(), bridgeID, afterCursor, listener)
	if err != nil {
		conn.Close() //nolint:errcheck
		log.WithFields(log.Fields{
			"remoteAddr":  remoteAddr,
			"bridgeId":    bridgeID,
			"afterCursor": afterCursor,
		}).WithError(err).Warn("IM control ws attach listener failed")
		return c.JSON(http.StatusConflict, map[string]string{"message": err.Error()})
	}
	log.WithFields(log.Fields{
		"remoteAddr":    remoteAddr,
		"bridgeId":      bridgeID,
		"afterCursor":   afterCursor,
		"replayedCount": len(replayed),
	}).Info("IM control ws listener attached")

	for _, delivery := range replayed {
		if err := listener.Send(c.Request().Context(), delivery); err != nil {
			conn.Close() //nolint:errcheck
			h.control.DetachBridgeListener(bridgeID)
			log.WithFields(log.Fields{
				"remoteAddr": remoteAddr,
				"bridgeId":   bridgeID,
				"cursor":     delivery.Cursor,
				"deliveryId": delivery.DeliveryID,
				"kind":       delivery.Kind,
			}).WithError(err).Warn("IM control ws replay send failed")
			return nil
		}
	}

	defer func() {
		h.control.DetachBridgeListener(bridgeID)
		conn.Close() //nolint:errcheck
		log.WithFields(log.Fields{
			"remoteAddr": remoteAddr,
			"bridgeId":   bridgeID,
		}).Info("IM control ws disconnected")
	}()

	conn.SetReadLimit(defaultMaxMessageSize * 4)
	conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.WithFields(log.Fields{
					"remoteAddr": remoteAddr,
					"bridgeId":   bridgeID,
				}).Debug("IM control ws closed")
				return nil
			}
			log.WithFields(log.Fields{
				"remoteAddr": remoteAddr,
				"bridgeId":   bridgeID,
			}).WithError(err).Warn("IM control ws read failed")
			return nil
		}
		var ack model.IMDeliveryAck
		if err := json.Unmarshal(message, &ack); err != nil {
			log.WithFields(log.Fields{
				"remoteAddr": remoteAddr,
				"bridgeId":   bridgeID,
				"sizeBytes":  len(message),
			}).WithError(err).Warn("IM control ws ignored invalid ack payload")
			continue
		}
		if ack.BridgeID == "" {
			ack.BridgeID = bridgeID
		}
		if err := h.control.AckDelivery(c.Request().Context(), &ack); err != nil {
			log.WithFields(log.Fields{
				"remoteAddr": remoteAddr,
				"bridgeId":   ack.BridgeID,
				"cursor":     ack.Cursor,
				"deliveryId": ack.DeliveryID,
			}).WithError(err).Warn("IM control ws ack failed")
			continue
		}
		log.WithFields(log.Fields{
			"remoteAddr": remoteAddr,
			"bridgeId":   ack.BridgeID,
			"cursor":     ack.Cursor,
			"deliveryId": ack.DeliveryID,
		}).Debug("IM control ws ack recorded")
	}
}

type imBridgeWSListener struct {
	conn *websocket.Conn
	mu   chan struct{}
}

func (l *imBridgeWSListener) Send(_ context.Context, delivery *model.IMControlDelivery) error {
	if l.conn == nil {
		return nil
	}
	l.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
	return l.conn.WriteJSON(delivery)
}

func (l *imBridgeWSListener) Close() error {
	if l.conn == nil {
		return nil
	}
	return l.conn.Close()
}
