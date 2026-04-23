package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/ws"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

const wsTestSecret = "test-secret-at-least-32-characters-long"

func makeToken(t *testing.T, secret, userID string) string {
	t.Helper()
	return makeTokenWithJTI(t, secret, userID, uuid.New().String())
}

func makeTokenWithJTI(t *testing.T, secret, userID, jti string) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        jti,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

type wsBlacklistStub struct {
	revoked map[string]bool
	err     error
}

func (s wsBlacklistStub) IsBlacklisted(context.Context, string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return false, nil
}

type revokedBlacklistStub struct {
	revoked map[string]bool
}

func (s revokedBlacklistStub) IsBlacklisted(_ context.Context, jti string) (bool, error) {
	return s.revoked[jti], nil
}

func dialWS(t *testing.T, hub *ws.Hub) (*httptest.Server, *websocket.Conn) {
	t.Helper()

	e := echo.New()
	h := ws.NewHandler(hub, wsTestSecret, wsBlacklistStub{}, []string{"http://localhost:3000"}, 4096)
	e.GET("/ws", h.HandleWS)

	srv := httptest.NewServer(e)

	token := makeToken(t, wsTestSecret, uuid.New().String())
	projectID := uuid.New().String()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token + "&projectId=" + projectID

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial websocket: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != 1 {
		if time.Now().After(deadline) {
			conn.Close()
			srv.Close()
			t.Fatalf("expected websocket client to register, client count = %d", hub.ClientCount())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return srv, conn
}

func TestHandleWS_MissingToken(t *testing.T) {
	e := echo.New()
	hub := ws.NewHub()
	h := ws.NewHandler(hub, wsTestSecret, wsBlacklistStub{}, []string{"http://localhost:3000"}, 4096)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.HandleWS(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleWS_RejectsRevokedToken(t *testing.T) {
	hub := ws.NewHub()
	e := echo.New()
	revokedJTI := uuid.New().String()
	e.GET("/ws", ws.NewHandler(hub, wsTestSecret, revokedBlacklistStub{revoked: map[string]bool{revokedJTI: true}}, []string{"http://localhost:3000"}, 4096).HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	token := makeTokenWithJTI(t, wsTestSecret, uuid.New().String(), revokedJTI)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token + "&projectId=" + uuid.New().String()
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err == nil {
		t.Fatal("expected revoked token dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestHandleWS_RejectsDisallowedOrigin(t *testing.T) {
	hub := ws.NewHub()
	e := echo.New()
	e.GET("/ws", ws.NewHandler(hub, wsTestSecret, wsBlacklistStub{}, []string{"http://localhost:3000"}, 4096).HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	token := makeToken(t, wsTestSecret, uuid.New().String())
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token + "&projectId=" + uuid.New().String()
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"https://evil.example"}})
	if err == nil {
		t.Fatal("expected disallowed origin dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, want 403", resp)
	}
}

func TestHandleWS_BroadcastAllDeliversFrame(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	srv, conn := dialWS(t, hub)
	defer srv.Close()
	defer conn.Close()

	hub.BroadcastAllBytes([]byte(`{"type":"system.notice","payload":{"msg":"hi"}}`))

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pushed event: %v", err)
	}
	if !strings.Contains(string(message), "system.notice") {
		t.Fatalf("expected system.notice in %s", string(message))
	}
}

func TestHandleWS_FanoutRequiresSubscription(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	srv, conn := dialWS(t, hub)
	defer srv.Close()
	defer conn.Close()

	// Subscribe the client to a specific channel.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"subscribe","channels":["channel:task:1"]}`)); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	// Give the read loop a moment to process the frame.
	time.Sleep(50 * time.Millisecond)

	// Frame targeting an unsubscribed channel — should NOT be delivered.
	hub.FanoutBytes([]byte(`{"type":"task.created","channel":"channel:task:2"}`), []string{"channel:task:2"})
	// Frame targeting the subscribed channel — should arrive.
	hub.FanoutBytes([]byte(`{"type":"task.created","channel":"channel:task:1"}`), []string{"channel:task:1"})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pushed event: %v", err)
	}
	body := string(message)
	if !strings.Contains(body, "channel:task:1") {
		t.Fatalf("expected channel:task:1 frame, got %s", body)
	}
	if strings.Contains(body, "channel:task:2") {
		t.Fatalf("unsubscribed channel leaked: %s", body)
	}
}

func TestHandleWS_UnsubscribeStopsDelivery(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	srv, conn := dialWS(t, hub)
	defer srv.Close()
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"subscribe","channels":["channel:task:1"]}`)); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"unsubscribe","channels":["channel:task:1"]}`)); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	hub.FanoutBytes([]byte(`{"type":"task.created","channel":"channel:task:1"}`), []string{"channel:task:1"})

	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected no frame after unsubscribe")
	}
}

func TestHandleWS_MalformedFrameSendsRejected(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	srv, conn := dialWS(t, hub)
	defer srv.Close()
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{not valid json`)); err != nil {
		t.Fatalf("write malformed: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read reject: %v", err)
	}
	var frame struct {
		Type    string `json:"type"`
		Payload struct {
			Reason string `json:"reason"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(message, &frame); err != nil {
		t.Fatalf("decode reject frame: %v: %s", err, message)
	}
	if frame.Type != "event.error.rejected" || frame.Payload.Reason != "bad frame" {
		t.Fatalf("unexpected reject frame: %+v", frame)
	}
}

func TestHandleWS_UnknownOpIsIgnored(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	srv, conn := dialWS(t, hub)
	defer srv.Close()
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"op":"party","channels":["c"]}`)); err != nil {
		t.Fatalf("write unknown op: %v", err)
	}

	// Expect no frame and connection remains open.
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected no frame for unknown op")
	}
	if hub.ClientCount() != 1 {
		t.Fatalf("client dropped unexpectedly, count=%d", hub.ClientCount())
	}
}
