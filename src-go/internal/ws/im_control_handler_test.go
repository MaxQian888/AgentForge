package ws_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/agentforge/server/internal/ws"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

func TestIMControlHandler_ReplaysPendingDeliveryAndAcceptsAck(t *testing.T) {
	control := service.NewIMControlPlane(service.IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})
	if _, err := control.RegisterBridge(context.Background(), &model.IMBridgeRegisterRequest{
		BridgeID:   "bridge-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	delivery, err := control.QueueDelivery(context.Background(), service.IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-1",
		Platform:       "slack",
		ProjectID:      "project-1",
		Kind:           service.IMDeliveryKindProgress,
		Content:        "agent running",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "1700000000.1",
		},
	})
	if err != nil {
		t.Fatalf("QueueDelivery error: %v", err)
	}

	e := echo.New()
	e.GET("/ws/im-bridge", ws.NewIMControlHandler(control, "shared-secret", []string{"http://localhost:3000"}).HandleWS)
	srv := httptest.NewServer(e)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/im-bridge?bridgeId=bridge-1&afterCursor=0"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer shared-secret"},
		"Origin":        []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	var replayed model.IMControlDelivery
	if err := conn.ReadJSON(&replayed); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if replayed.DeliveryID != delivery.DeliveryID {
		t.Fatalf("delivery id = %q, want %q", replayed.DeliveryID, delivery.DeliveryID)
	}

	if err := conn.WriteJSON(model.IMDeliveryAck{
		BridgeID:   "bridge-1",
		Cursor:     delivery.Cursor,
		DeliveryID: delivery.DeliveryID,
	}); err != nil {
		t.Fatalf("WriteJSON ack error: %v", err)
	}

	control.DetachBridgeListener("bridge-1")
	replayedList, err := control.AttachBridgeListener(context.Background(), "bridge-1", delivery.Cursor, &fakeBridgeAckListener{})
	if err != nil {
		t.Fatalf("AttachBridgeListener replay error: %v", err)
	}
	if len(replayedList) != 0 {
		t.Fatalf("replayed deliveries = %d, want 0", len(replayedList))
	}
}

func TestIMControlHandler_RejectsMissingSharedSecret(t *testing.T) {
	control := service.NewIMControlPlane(service.IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})
	e := echo.New()
	e.GET("/ws/im-bridge", ws.NewIMControlHandler(control, "shared-secret", []string{"http://localhost:3000"}).HandleWS)

	srv := httptest.NewServer(e)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/im-bridge?bridgeId=bridge-1"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err == nil {
		t.Fatal("expected unauthorized dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

type fakeBridgeAckListener struct{}

func (l *fakeBridgeAckListener) Send(context.Context, *model.IMControlDelivery) error { return nil }
func (l *fakeBridgeAckListener) Close() error                                         { return nil }
