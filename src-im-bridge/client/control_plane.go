package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/agentforge/im-bridge/core"
	"github.com/gorilla/websocket"
)

type ControlDelivery struct {
	Cursor         int64             `json:"cursor"`
	DeliveryID     string            `json:"deliveryId"`
	TargetBridgeID string            `json:"targetBridgeId"`
	Platform       string            `json:"platform"`
	ProjectID      string            `json:"projectId,omitempty"`
	Kind           string            `json:"kind"`
	Content        string            `json:"content"`
	TargetChatID   string            `json:"targetChatId,omitempty"`
	ReplyTarget    *core.ReplyTarget `json:"replyTarget,omitempty"`
	Timestamp      string            `json:"timestamp"`
	Signature      string            `json:"signature,omitempty"`
}

type ControlDeliveryAck struct {
	BridgeID   string `json:"bridgeId"`
	Cursor     int64  `json:"cursor"`
	DeliveryID string `json:"deliveryId,omitempty"`
}

type ControlPlaneConn struct {
	conn     *websocket.Conn
	bridgeID string
	mu       sync.Mutex
}

func DialControlPlane(ctx context.Context, baseURL, bridgeID string, afterCursor int64) (*ControlPlaneConn, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/ws/im-bridge"
	query := u.Query()
	query.Set("bridgeId", strings.TrimSpace(bridgeID))
	query.Set("afterCursor", fmt.Sprintf("%d", afterCursor))
	u.RawQuery = query.Encode()

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), http.Header{})
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial control plane websocket: status=%d err=%w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("dial control plane websocket: %w", err)
	}

	return &ControlPlaneConn{
		conn:     conn,
		bridgeID: strings.TrimSpace(bridgeID),
	}, nil
}

func (c *ControlPlaneConn) ReadDelivery(ctx context.Context) (*ControlDelivery, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("control plane websocket not connected")
	}

	type readResult struct {
		delivery *ControlDelivery
		err      error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		var delivery ControlDelivery
		if err := c.conn.ReadJSON(&delivery); err != nil {
			resultCh <- readResult{err: err}
			return
		}
		resultCh <- readResult{delivery: &delivery}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.delivery, result.err
	}
}

func (c *ControlPlaneConn) Ack(cursor int64, deliveryID string) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("control plane websocket not connected")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(ControlDeliveryAck{
		BridgeID:   c.bridgeID,
		Cursor:     cursor,
		DeliveryID: deliveryID,
	})
}

func (c *ControlPlaneConn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func EncodeReplyTargetHeader(target *core.ReplyTarget) string {
	if target == nil {
		return ""
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		return ""
	}
	return string(encoded)
}
