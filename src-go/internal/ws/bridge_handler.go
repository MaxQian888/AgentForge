package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
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
	if h.processor == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "bridge processor unavailable"})
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		slog.Error("bridge ws upgrade failed", "error", err)
		return err
	}
	defer conn.Close() //nolint:errcheck

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
				return nil
			}
			slog.Debug("bridge ws closed", "error", err)
			return nil
		}

		var event BridgeAgentEvent
		if err := json.Unmarshal(message, &event); err != nil {
			slog.Warn("invalid bridge ws payload", "error", err)
			continue
		}

		if err := h.processor.ProcessBridgeEvent(c.Request().Context(), &event); err != nil {
			slog.Warn("bridge event processing failed", "task", event.TaskID, "type", event.Type, "error", err)
		}
	}
}
