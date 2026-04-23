package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/ws"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type fakeBridgeProcessor struct {
	events chan *ws.BridgeAgentEvent
}

func (p *fakeBridgeProcessor) ProcessBridgeEvent(_ context.Context, event *ws.BridgeAgentEvent) error {
	p.events <- event
	return nil
}

func TestBridgeHandler_ForwardsParsedBridgeEvents(t *testing.T) {
	processor := &fakeBridgeProcessor{events: make(chan *ws.BridgeAgentEvent, 1)}
	e := echo.New()
	e.GET("/ws/bridge", ws.NewBridgeHandler(processor, "bridge-secret", []string{"http://localhost:3000"}).HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/bridge"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer bridge-secret"},
		"Origin":        []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("dial bridge websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"task_id":      "task-bridge",
		"session_id":   "session-bridge",
		"timestamp_ms": 123,
		"type":         "output",
		"data": map[string]any{
			"content":      "hello from bridge",
			"content_type": "text",
			"turn_number":  1,
		},
	}); err != nil {
		t.Fatalf("write bridge event: %v", err)
	}

	select {
	case event := <-processor.events:
		if event.TaskID != "task-bridge" {
			t.Fatalf("task id = %q, want task-bridge", event.TaskID)
		}
		if event.Type != ws.BridgeEventOutput {
			t.Fatalf("event type = %q, want %q", event.Type, ws.BridgeEventOutput)
		}
		if !strings.Contains(string(event.Data), "hello from bridge") {
			t.Fatalf("event data = %s", string(event.Data))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for processed bridge event")
	}
}

func TestBridgeHandler_RejectsMissingSharedSecret(t *testing.T) {
	e := echo.New()
	e.GET("/ws/bridge", ws.NewBridgeHandler(&fakeBridgeProcessor{events: make(chan *ws.BridgeAgentEvent, 1)}, "bridge-secret", []string{"http://localhost:3000"}).HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/bridge"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err == nil {
		t.Fatal("expected unauthorized dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}
