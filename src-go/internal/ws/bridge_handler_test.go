package ws_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/ws"
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
	e.GET("/ws/bridge", ws.NewBridgeHandler(processor).HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/bridge"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
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
