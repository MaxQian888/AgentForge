package ws_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
	"github.com/gorilla/websocket"
)

const wsTestSecret = "test-secret-at-least-32-characters-long"

func makeToken(t *testing.T, secret, userID string) string {
	t.Helper()
	claims := &service.Claims{
		UserID: userID,
		Email:  "ws@example.com",
		JTI:    uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestHandleWS_MissingToken(t *testing.T) {
	e := echo.New()
	hub := ws.NewHub()
	h := ws.NewHandler(hub, wsTestSecret)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.HandleWS(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWS_DeliversServerPushEvent(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	e := echo.New()
	h := ws.NewHandler(hub, wsTestSecret)
	e.GET("/ws", h.HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	projectID := uuid.New().String()
	token := makeToken(t, wsTestSecret, uuid.New().String())
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token + "&projectId=" + projectID

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventAgentStarted,
		ProjectID: projectID,
		Payload: map[string]any{
			"status": "running",
		},
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pushed event: %v", err)
	}

	body := string(message)
	if !strings.Contains(body, ws.EventAgentStarted) {
		t.Fatalf("expected event type %q in %s", ws.EventAgentStarted, body)
	}
	if !strings.Contains(body, projectID) {
		t.Fatalf("expected project ID %s in %s", projectID, body)
	}
}
