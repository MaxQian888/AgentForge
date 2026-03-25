package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

type bridgeRuntimeControl struct {
	cfg      *config
	bridgeID string
	platform core.Platform
	client   *client.AgentForgeClient

	cancel context.CancelFunc
	wg     sync.WaitGroup

	cursorMu   sync.Mutex
	lastCursor int64
}

func newBridgeRuntimeControl(cfg *config, bridgeID string, platform core.Platform, apiClient *client.AgentForgeClient) *bridgeRuntimeControl {
	return &bridgeRuntimeControl{
		cfg:      cfg,
		bridgeID: bridgeID,
		platform: platform,
		client:   apiClient,
	}
}

func (c *bridgeRuntimeControl) Start(ctx context.Context) error {
	if c == nil || c.client == nil || c.platform == nil {
		return nil
	}
	metadata := core.MetadataForPlatform(c.platform)

	registration := client.BridgeRegistration{
		BridgeID:   c.bridgeID,
		Platform:   metadata.Source,
		Transport:  normalizeTransportMode(c.cfg.TransportMode),
		ProjectIDs: []string{strings.TrimSpace(c.cfg.ProjectID)},
		Capabilities: map[string]bool{
			"supports_deferred_reply":  metadata.Capabilities.SupportsDeferredReply,
			"supports_rich_messages":   metadata.Capabilities.SupportsRichMessages,
			"requires_public_callback": metadata.Capabilities.RequiresPublicCallback,
			"supports_mentions":        metadata.Capabilities.SupportsMentions,
			"supports_slash_commands":  metadata.Capabilities.SupportsSlashCommands,
		},
		CapabilityMatrix: metadata.Capabilities.Matrix(),
		CallbackPaths:    []string{"/im/notify", "/im/send"},
		Metadata: map[string]string{
			"platform_name": c.platform.Name(),
		},
	}
	if _, err := c.client.RegisterBridge(ctx, registration); err != nil {
		return fmt.Errorf("register bridge runtime: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		c.heartbeatLoop(runCtx)
	}()
	go func() {
		defer c.wg.Done()
		c.controlPlaneLoop(runCtx)
	}()
	return nil
}

func (c *bridgeRuntimeControl) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	if c.client != nil {
		return c.client.UnregisterBridge(ctx, c.bridgeID)
	}
	return nil
}

func (c *bridgeRuntimeControl) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.client.HeartbeatBridge(ctx, c.bridgeID); err != nil {
				log.Printf("[control-plane] heartbeat failed: %v", err)
			}
		}
	}
}

func (c *bridgeRuntimeControl) controlPlaneLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := client.DialControlPlane(ctx, c.cfg.APIBase, c.bridgeID, c.cursor())
		if err != nil {
			log.Printf("[control-plane] websocket connect failed: %v", err)
			select {
			case <-time.After(c.cfg.ControlReconnectDelay):
				continue
			case <-ctx.Done():
				return
			}
		}

		if err := c.consumeDeliveries(ctx, conn); err != nil && ctx.Err() == nil {
			log.Printf("[control-plane] websocket loop stopped: %v", err)
		}
		_ = conn.Close()

		select {
		case <-time.After(c.cfg.ControlReconnectDelay):
		case <-ctx.Done():
			return
		}
	}
}

func (c *bridgeRuntimeControl) consumeDeliveries(ctx context.Context, conn *client.ControlPlaneConn) error {
	for {
		delivery, err := conn.ReadDelivery(ctx)
		if err != nil {
			return err
		}
		if delivery == nil {
			continue
		}
		if strings.TrimSpace(delivery.TargetBridgeID) != "" && strings.TrimSpace(delivery.TargetBridgeID) != c.bridgeID {
			continue
		}
		if !c.verifyDelivery(delivery) {
			log.Printf("[control-plane] rejected delivery %s due to invalid signature", delivery.DeliveryID)
			continue
		}
		if err := c.applyDelivery(ctx, delivery); err != nil {
			log.Printf("[control-plane] failed to apply delivery %s: %v", delivery.DeliveryID, err)
			continue
		}
		c.setCursor(delivery.Cursor)
		if err := conn.Ack(delivery.Cursor, delivery.DeliveryID); err != nil {
			log.Printf("[control-plane] ack failed for delivery %s: %v", delivery.DeliveryID, err)
		}
	}
}

func (c *bridgeRuntimeControl) applyDelivery(ctx context.Context, delivery *client.ControlDelivery) error {
	if delivery == nil {
		return nil
	}
	targetChatID := strings.TrimSpace(delivery.TargetChatID)
	if _, err := core.DeliverText(ctx, c.platform, core.MetadataForPlatform(c.platform), delivery.ReplyTarget, targetChatID, delivery.Content); err != nil {
		return err
	}
	return nil
}

func (c *bridgeRuntimeControl) verifyDelivery(delivery *client.ControlDelivery) bool {
	if c == nil || c.cfg == nil || strings.TrimSpace(c.cfg.ControlSharedSecret) == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(c.cfg.ControlSharedSecret)))
	_, _ = mac.Write([]byte(strings.Join([]string{
		strings.TrimSpace(delivery.TargetBridgeID),
		fmt.Sprintf("%d", delivery.Cursor),
		strings.TrimSpace(delivery.DeliveryID),
		strings.TrimSpace(delivery.Kind),
		strings.TrimSpace(delivery.Content),
		strings.TrimSpace(delivery.Timestamp),
	}, "|")))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(delivery.Signature)))
}

func (c *bridgeRuntimeControl) cursor() int64 {
	c.cursorMu.Lock()
	defer c.cursorMu.Unlock()
	return c.lastCursor
}

func (c *bridgeRuntimeControl) setCursor(value int64) {
	c.cursorMu.Lock()
	defer c.cursorMu.Unlock()
	if value > c.lastCursor {
		c.lastCursor = value
	}
}

func loadOrCreateBridgeID(path string) (string, error) {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		return "", errors.New("bridge id file path is required")
	}
	if data, err := os.ReadFile(resolved); err == nil {
		if value := strings.TrimSpace(string(data)); value != "" {
			return value, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return "", fmt.Errorf("create bridge id dir: %w", err)
	}
	value := fmt.Sprintf("bridge-%d", time.Now().UTC().UnixNano())
	if err := os.WriteFile(resolved, []byte(value), 0o600); err != nil {
		return "", fmt.Errorf("write bridge id file: %w", err)
	}
	return value, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
