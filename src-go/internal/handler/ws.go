package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type WSHandler struct {
	jwtSecret string
}

func NewWSHandler(jwtSecret string) *WSHandler {
	return &WSHandler{jwtSecret: jwtSecret}
}

func (h *WSHandler) HandleWS(c echo.Context) error {
	// Accept token from query param or Authorization header
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

	// Validate JWT
	_, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "invalid token"})
	}

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close() //nolint:errcheck // best-effort close in deferred cleanup
		for {
			var msg string
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				break
			}
			reply := fmt.Sprintf(`{"message":%q,"time":%q}`, msg, time.Now().UTC().Format(time.RFC3339))
			if err := websocket.Message.Send(ws, reply); err != nil {
				break
			}
		}
	}).ServeHTTP(c.Response(), c.Request())

	return nil
}
