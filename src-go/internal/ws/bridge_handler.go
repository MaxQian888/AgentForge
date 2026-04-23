package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/agentforge/server/internal/authutil"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// BridgeEventProcessor projects runtime bridge events back into Go orchestration state.
type BridgeEventProcessor interface {
	ProcessBridgeEvent(ctx context.Context, event *BridgeAgentEvent) error
}

// BridgeHandler accepts the internal bridge websocket used by the TS runtime service.
type BridgeHandler struct {
	processor    BridgeEventProcessor
	sharedSecret string
	upgrader     websocket.Upgrader
}

func NewBridgeHandler(processor BridgeEventProcessor, sharedSecret string, allowedOrigins []string) *BridgeHandler {
	return &BridgeHandler{
		processor:    processor,
		sharedSecret: strings.TrimSpace(sharedSecret),
		upgrader:     newUpgrader(allowedOrigins),
	}
}

func (h *BridgeHandler) HandleWS(c echo.Context) error {
	remoteAddr := c.RealIP()

	if h.processor == nil {
		log.WithField("remoteAddr", remoteAddr).Warn("bridge ws rejected: processor unavailable")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "bridge processor unavailable"})
	}

	if err := authutil.ValidateBearerSharedSecret(c.Request().Header.Get("Authorization"), h.sharedSecret); err != nil {
		entry := log.WithField("remoteAddr", remoteAddr).WithError(err)
		if errors.Is(err, authutil.ErrSharedSecretNotConfigured) {
			entry.Warn("bridge ws rejected: auth misconfigured")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "bridge auth unavailable"})
		}
		entry.Warn("bridge ws rejected: unauthorized")
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
	}

	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.WithField("remoteAddr", remoteAddr).WithError(err).Error("bridge ws upgrade failed")
		return err
	}
	defer conn.Close() //nolint:errcheck
	log.WithField("remoteAddr", remoteAddr).Info("bridge ws connected")

	conn.SetReadLimit(defaultMaxMessageSize * 8)
	conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.WithField("remoteAddr", remoteAddr).Debug("bridge ws closed")
				return nil
			}
			log.WithField("remoteAddr", remoteAddr).WithError(err).Warn("bridge ws read failed")
			return nil
		}

		var event BridgeAgentEvent
		if err := json.Unmarshal(message, &event); err != nil {
			log.WithFields(log.Fields{
				"remoteAddr": remoteAddr,
				"sizeBytes":  len(message),
			}).WithError(err).Warn("invalid bridge ws payload")
			continue
		}

		if err := h.processor.ProcessBridgeEvent(c.Request().Context(), &event); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"remoteAddr": remoteAddr,
				"taskId":     event.TaskID,
				"sessionId":  event.SessionID,
				"eventType":  event.Type,
			}).Warn("bridge event processing failed")
		}
	}
}
