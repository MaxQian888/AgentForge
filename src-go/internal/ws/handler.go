package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS handled by Echo middleware
	},
}

// Handler handles WebSocket upgrade requests.
type Handler struct {
	hub       *Hub
	jwtSecret string
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, jwtSecret string) *Handler {
	return &Handler{hub: hub, jwtSecret: jwtSecret}
}

// HandleWS upgrades the HTTP connection to a WebSocket and registers the client.
func (h *Handler) HandleWS(c echo.Context) error {
	remoteAddr := c.RealIP()
	projectID := c.QueryParam("projectId")

	// Accept token from query param or Authorization header.
	token := c.QueryParam("token")
	if token == "" {
		authHeader := c.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		log.WithFields(log.Fields{"projectId": projectID, "remoteAddr": remoteAddr}).Warn("ws upgrade rejected: missing token")
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "missing token"})
	}

	// Validate JWT.
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		log.WithFields(log.Fields{"projectId": projectID, "remoteAddr": remoteAddr}).WithError(err).Warn("ws upgrade rejected: invalid token")
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "invalid token"})
	}

	userID, _ := claims.GetSubject()

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.WithFields(log.Fields{"projectId": projectID, "remoteAddr": remoteAddr}).WithError(err).Error("ws upgrade failed")
		return err
	}

	client := &Client{
		hub:        h.hub,
		conn:       conn,
		send:       make(chan []byte, 256),
		projectID:  projectID,
		userID:     userID,
		remoteAddr: remoteAddr,
	}
	log.WithFields(client.logFields()).Info("ws client upgraded")
	h.hub.register <- client

	go client.writePump()
	go client.readPump()
	return nil
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close() //nolint:errcheck
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			entry := log.WithFields(c.logFields())
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				entry.WithError(err).Warn("ws client read failed")
			} else {
				entry.WithError(err).Debug("ws client closed")
			}
			break
		}
		c.handleFrame(message)
	}
}

// clientFrame is the shape of inbound client->server control frames.
// Legacy subscribe/unsubscribe frames use `op` + `channels`. Live-artifact
// asset_open / asset_close frames use `type` + `payload`, handled by the
// subscription router via Hub.DeliverClientMessage.
type clientFrame struct {
	Op       string   `json:"op"`
	Channels []string `json:"channels,omitempty"`
	Type     string   `json:"type,omitempty"`
}

// handleFrame parses a single inbound frame and mutates subscription state.
// Unknown ops are silently ignored. Malformed JSON sends a one-shot
// `event.error.rejected` frame without disconnecting the client.
func (c *Client) handleFrame(raw []byte) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return
	}
	var frame clientFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		c.sendRejected("bad frame")
		return
	}
	// Live-artifact subscription frames route through the router.
	switch frame.Type {
	case "asset_open", "asset_close":
		if err := c.hub.DeliverClientMessage(c.id, raw); err != nil {
			c.sendRejected(err.Error())
		}
		return
	}
	switch frame.Op {
	case "subscribe":
		c.subscribe(frame.Channels)
	case "unsubscribe":
		c.unsubscribe(frame.Channels)
	default:
		// Ignore unknown ops per protocol.
	}
}

func (c *Client) sendRejected(reason string) {
	payload := map[string]any{
		"type":    "event.error.rejected",
		"payload": map[string]string{"reason": reason},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close() //nolint:errcheck
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if !ok {
				log.WithFields(c.logFields()).Debug("ws client send channel closed")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.WithFields(c.logFields()).WithError(err).Warn("ws client write failed")
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.WithFields(c.logFields()).WithError(err).Debug("ws client ping failed")
				return
			}
		}
	}
}
