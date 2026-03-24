package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

type IMControlPlane interface {
	AttachBridgeListener(ctx context.Context, bridgeID string, afterCursor int64, listener IMBridgeListener) ([]*model.IMControlDelivery, error)
	AckDelivery(ctx context.Context, bridgeID string, cursor int64, deliveryID string) error
	DetachBridgeListener(bridgeID string)
}

type IMBridgeListener interface {
	Send(ctx context.Context, delivery *model.IMControlDelivery) error
	Close() error
}

type IMControlHandler struct {
	control IMControlPlane
}

func NewIMControlHandler(control IMControlPlane) *IMControlHandler {
	return &IMControlHandler{control: control}
}

func (h *IMControlHandler) HandleWS(c echo.Context) error {
	bridgeID := c.QueryParam("bridgeId")
	if bridgeID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "missing bridgeId"})
	}

	afterCursor := int64(0)
	if raw := c.QueryParam("afterCursor"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "invalid afterCursor"})
		}
		afterCursor = parsed
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	listener := &imBridgeWSListener{conn: conn}
	replayed, err := h.control.AttachBridgeListener(c.Request().Context(), bridgeID, afterCursor, listener)
	if err != nil {
		conn.Close() //nolint:errcheck
		return c.JSON(http.StatusConflict, map[string]string{"message": err.Error()})
	}

	for _, delivery := range replayed {
		if err := listener.Send(c.Request().Context(), delivery); err != nil {
			conn.Close() //nolint:errcheck
			h.control.DetachBridgeListener(bridgeID)
			return nil
		}
	}

	defer func() {
		h.control.DetachBridgeListener(bridgeID)
		conn.Close() //nolint:errcheck
	}()

	conn.SetReadLimit(maxMessageSize * 4)
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
			return nil
		}
		var ack model.IMDeliveryAck
		if err := json.Unmarshal(message, &ack); err != nil {
			continue
		}
		if ack.BridgeID == "" {
			ack.BridgeID = bridgeID
		}
		if err := h.control.AckDelivery(c.Request().Context(), ack.BridgeID, ack.Cursor, ack.DeliveryID); err != nil {
			continue
		}
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
