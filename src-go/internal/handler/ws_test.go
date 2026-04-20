package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

func TestNewWSHandler(t *testing.T) {
	h := handler.NewWSHandler("secret")
	if h == nil {
		t.Fatal("expected non-nil WSHandler")
	}
}

func TestWSHandler_MissingToken(t *testing.T) {
	e := echo.New()
	h := handler.NewWSHandler(testSecret)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.HandleWS(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWSHandler_InvalidTokenQueryParam(t *testing.T) {
	e := echo.New()
	h := handler.NewWSHandler(testSecret)

	req := httptest.NewRequest(http.MethodGet, "/ws?token=invalid-jwt", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.HandleWS(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWSHandler_InvalidTokenFromHeader(t *testing.T) {
	e := echo.New()
	h := handler.NewWSHandler(testSecret)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.HandleWS(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWSHandler_MissingBearerPrefix(t *testing.T) {
	e := echo.New()
	h := handler.NewWSHandler(testSecret)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Authorization", "Token some-value")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.HandleWS(c)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func makeWSToken(secret string) string {
	claims := &service.Claims{
		UserID: uuid.New().String(),
		Email:  "ws@example.com",
		JTI:    uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return token
}

func TestWSHandler_ValidToken_WebSocketEcho(t *testing.T) {
	e := echo.New()
	h := handler.NewWSHandler(testSecret)
	e.GET("/ws", h.HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	token := makeWSToken(testSecret)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token

	ws, err := websocket.Dial(wsURL, "", srv.URL)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer func() {
		_ = ws.Close()
	}()

	// Send a message
	if err := websocket.Message.Send(ws, "hello"); err != nil {
		t.Fatalf("websocket send: %v", err)
	}

	// Receive the echo
	var reply string
	if err := websocket.Message.Receive(ws, &reply); err != nil {
		t.Fatalf("websocket receive: %v", err)
	}

	if !strings.Contains(reply, "hello") {
		t.Errorf("expected reply to contain 'hello', got %q", reply)
	}
	if !strings.Contains(reply, "time") {
		t.Errorf("expected reply to contain 'time', got %q", reply)
	}
}

func TestWSHandler_ValidToken_HeaderAuth_WebSocket(t *testing.T) {
	e := echo.New()
	h := handler.NewWSHandler(testSecret)
	e.GET("/ws", h.HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	token := makeWSToken(testSecret)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	config, err := websocket.NewConfig(wsURL, srv.URL)
	if err != nil {
		t.Fatalf("websocket config: %v", err)
	}
	config.Header.Set("Authorization", "Bearer "+token)

	ws, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer func() {
		_ = ws.Close()
	}()

	if err := websocket.Message.Send(ws, "world"); err != nil {
		t.Fatalf("websocket send: %v", err)
	}

	var reply string
	if err := websocket.Message.Receive(ws, &reply); err != nil {
		t.Fatalf("websocket receive: %v", err)
	}

	if !strings.Contains(reply, "world") {
		t.Errorf("expected reply to contain 'world', got %q", reply)
	}
}
