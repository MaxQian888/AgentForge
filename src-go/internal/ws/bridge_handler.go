package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

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
	processor BridgeEventProcessor
}

func NewBridgeHandler(processor BridgeEventProcessor) *BridgeHandler {
	return &BridgeHandler{processor: processor}
}

func (h *BridgeHandler) HandleWS(c echo.Context) error {
	remoteAddr := c.RealIP()

	if h.processor == nil {
		log.WithField("remoteAddr", remoteAddr).Warn("bridge ws rejected: processor unavailable")
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "bridge processor unavailable"})
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.WithField("remoteAddr", remoteAddr).WithError(err).Error("bridge ws upgrade failed")
		return err
	}
	defer conn.Close() //nolint:errcheck
	log.WithField("remoteAddr", remoteAddr).Info("bridge ws connected")

	conn.SetReadLimit(maxMessageSize * 8)
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
