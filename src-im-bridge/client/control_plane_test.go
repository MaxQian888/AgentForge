package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
	"github.com/gorilla/websocket"
)

func TestDialControlPlane_NormalizesPathAndQuery(t *testing.T) {
	upgrader := websocket.Upgrader{}
	type requestInfo struct {
		path     string
		bridgeID string
		cursor   string
	}
	infoCh := make(chan requestInfo, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade error: %v", err)
		}
		infoCh <- requestInfo{
			path:     r.URL.Path,
			bridgeID: r.URL.Query().Get("bridgeId"),
			cursor:   r.URL.Query().Get("afterCursor"),
		}
		_ = conn.Close()
	}))
	defer server.Close()

	conn, err := DialControlPlane(context.Background(), server.URL+"/ignored?stale=1", " bridge-1 ", 42)
	if err != nil {
		t.Fatalf("DialControlPlane error: %v", err)
	}
	defer conn.Close()

	select {
	case info := <-infoCh:
		if info.path != "/ws/im-bridge" {
			t.Fatalf("path = %q", info.path)
		}
		if info.bridgeID != "bridge-1" {
			t.Fatalf("bridgeId = %q", info.bridgeID)
		}
		if info.cursor != "42" {
			t.Fatalf("afterCursor = %q", info.cursor)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for websocket request")
	}
}

func TestDialControlPlane_RejectsInvalidBaseURL(t *testing.T) {
	if _, err := DialControlPlane(context.Background(), "://bad", "bridge-1", 0); err == nil {
		t.Fatal("expected invalid base url to fail")
	}
}

func TestControlPlaneConn_ReadDeliveryRejectsDisconnectedSocket(t *testing.T) {
	var conn *ControlPlaneConn
	if _, err := conn.ReadDelivery(context.Background()); err == nil {
		t.Fatal("expected disconnected websocket to fail")
	}
}

func TestControlPlaneConn_ReadDeliveryHonorsContextCancellation(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade error: %v", err)
		}
		defer conn.Close()
		<-r.Context().Done()
	}))
	defer server.Close()

	conn, err := DialControlPlane(context.Background(), server.URL, "bridge-1", 0)
	if err != nil {
		t.Fatalf("DialControlPlane error: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := conn.ReadDelivery(ctx); err == nil {
		t.Fatal("expected cancelled context to fail")
	}
}

func TestControlPlaneConn_ReadDeliveryParsesPayload(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade error: %v", err)
		}
		defer conn.Close()

		if err := conn.WriteJSON(ControlDelivery{
			Cursor:         7,
			DeliveryID:     "delivery-1",
			TargetBridgeID: "bridge-1",
			Content:        "hello",
		}); err != nil {
			t.Fatalf("WriteJSON error: %v", err)
		}
	}))
	defer server.Close()

	conn, err := DialControlPlane(context.Background(), server.URL, "bridge-1", 0)
	if err != nil {
		t.Fatalf("DialControlPlane error: %v", err)
	}
	defer conn.Close()

	delivery, err := conn.ReadDelivery(context.Background())
	if err != nil {
		t.Fatalf("ReadDelivery error: %v", err)
	}
	if delivery == nil {
		t.Fatal("expected delivery")
	}
	if delivery.Cursor != 7 || delivery.DeliveryID != "delivery-1" || delivery.Content != "hello" {
		t.Fatalf("delivery = %+v", delivery)
	}
}

func TestControlPlaneConn_AckWritesBridgeCursorPayload(t *testing.T) {
	upgrader := websocket.Upgrader{}
	ackCh := make(chan ControlDeliveryAck, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade error: %v", err)
		}
		defer conn.Close()

		var ack ControlDeliveryAck
		if err := conn.ReadJSON(&ack); err != nil {
			t.Fatalf("ReadJSON error: %v", err)
		}
		ackCh <- ack
	}))
	defer server.Close()

	conn, err := DialControlPlane(context.Background(), server.URL, "bridge-1", 0)
	if err != nil {
		t.Fatalf("DialControlPlane error: %v", err)
	}
	defer conn.Close()

	if err := conn.Ack(9, "delivery-9", "actioncard_send_failed"); err != nil {
		t.Fatalf("Ack error: %v", err)
	}

	select {
	case ack := <-ackCh:
		if ack.BridgeID != "bridge-1" || ack.Cursor != 9 || ack.DeliveryID != "delivery-9" {
			t.Fatalf("ack = %+v", ack)
		}
		if ack.DowngradeReason != "actioncard_send_failed" {
			t.Fatalf("DowngradeReason = %q", ack.DowngradeReason)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ack payload")
	}
}

func TestControlPlaneConn_CloseIsNilSafe(t *testing.T) {
	var conn *ControlPlaneConn
	if err := conn.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestEncodeReplyTargetHeader_EncodesReplyTargetJSON(t *testing.T) {
	header := EncodeReplyTargetHeader(&core.ReplyTarget{
		Platform:  "slack",
		ChannelID: "C123",
		ThreadID:  "thread-1",
	})
	if header == "" {
		t.Fatal("expected encoded reply target header")
	}

	var decoded core.ReplyTarget
	if err := json.Unmarshal([]byte(header), &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.ChannelID != "C123" || decoded.ThreadID != "thread-1" {
		t.Fatalf("decoded = %+v", decoded)
	}

	if got := EncodeReplyTargetHeader(nil); got != "" {
		t.Fatalf("EncodeReplyTargetHeader(nil) = %q, want empty string", got)
	}
}
