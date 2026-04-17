package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestIMControlPlane_TenantScopedRoutingFiltersByTenants(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
	})

	// Bridge A hosts acme + beta; bridge B hosts only gamma.
	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-a",
		Platform:   "feishu",
		Transport:  "live",
		ProjectIDs: []string{"project-a"},
		Tenants:    []string{"acme", "beta"},
		TenantManifest: []model.IMTenantBinding{
			{ID: "acme", ProjectID: "project-acme"},
			{ID: "beta", ProjectID: "project-beta"},
		},
	}); err != nil {
		t.Fatalf("RegisterBridge A: %v", err)
	}
	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-b",
		Platform:   "feishu",
		Transport:  "live",
		ProjectIDs: []string{"project-b"},
		Tenants:    []string{"gamma"},
	}); err != nil {
		t.Fatalf("RegisterBridge B: %v", err)
	}

	// Unscoped queue → any live feishu bridge is acceptable; the selector
	// prefers the lexicographically-first bridge id.
	delivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		Platform: "feishu",
		Kind:     IMDeliveryKindProgress,
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("unscoped QueueDelivery: %v", err)
	}
	if delivery.TargetBridgeID != "bridge-a" {
		t.Fatalf("unscoped routing picked %q, want bridge-a", delivery.TargetBridgeID)
	}

	// Tenant-scoped queue for gamma MUST skip bridge-a (which only has
	// acme+beta) and pick bridge-b.
	delivery, err = control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		Platform: "feishu",
		TenantID: "gamma",
		Kind:     IMDeliveryKindProgress,
		Content:  "hi gamma",
	})
	if err != nil {
		t.Fatalf("tenant=gamma QueueDelivery: %v", err)
	}
	if delivery.TargetBridgeID != "bridge-b" {
		t.Fatalf("gamma routing picked %q, want bridge-b", delivery.TargetBridgeID)
	}
	if delivery.TenantID != "gamma" {
		t.Fatalf("delivery tenant id = %q, want gamma", delivery.TenantID)
	}

	// Explicitly targeting bridge-a with tenant=gamma MUST surface the
	// mismatch error rather than silently routing or falling back.
	_, err = control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-a",
		Platform:       "feishu",
		TenantID:       "gamma",
		Kind:           IMDeliveryKindProgress,
		Content:        "cross-tenant",
	})
	if !errors.Is(err, ErrIMTenantProviderMismatch) {
		t.Fatalf("expected ErrIMTenantProviderMismatch, got %v", err)
	}
}

func TestIMControlPlane_LegacyBridgeWithoutTenantsStillServesAnyTenant(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
	})

	// A legacy registration that doesn't declare Tenants[] is accepted by
	// every tenant-scoped delivery, preserving back-compat.
	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-legacy",
		Platform:   "feishu",
		Transport:  "live",
		ProjectIDs: []string{"project-legacy"},
	}); err != nil {
		t.Fatalf("RegisterBridge: %v", err)
	}

	delivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		Platform: "feishu",
		TenantID: "acme",
		Kind:     IMDeliveryKindProgress,
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("legacy routing: %v", err)
	}
	if delivery.TargetBridgeID != "bridge-legacy" {
		t.Fatalf("expected legacy bridge to handle acme, got %q", delivery.TargetBridgeID)
	}
}
