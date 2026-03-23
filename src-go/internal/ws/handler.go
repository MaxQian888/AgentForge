package ws

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
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
	// Accept token from query param or Authorization header.
	token := c.QueryParam("token")
	if token == "" {
		authHeader := c.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
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
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "invalid token"})
	}

	userID, _ := claims.GetSubject()
	projectID := c.QueryParam("projectId")

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err)
		return err
	}

	client := &Client{
		hub:       h.hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		projectID: projectID,
		userID:    userID,
	}
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
	c.conn.SetReadDeadline(time.Now().Add(pongWait))   //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// Client messages are currently ignored; server-push only.
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
