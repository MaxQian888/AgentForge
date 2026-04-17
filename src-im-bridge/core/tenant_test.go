package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestChatIDResolverHit(t *testing.T) {
	acme := &Tenant{ID: "acme"}
	beta := &Tenant{ID: "beta"}
	r := NewChatIDResolver([]ChatIDBinding{
		{Tenant: acme, Platform: "feishu", ChatIDs: []string{"oc_abc"}},
		{Tenant: beta, ChatIDs: []string{"oc_def"}},
	})
	if got, err := r.Resolve(&Message{Platform: "feishu", ChatID: "oc_abc"}); err != nil || got != acme {
		t.Fatalf("expected acme, got %v err %v", got, err)
	}
	// Any-platform binding matches regardless of platform.
	if got, err := r.Resolve(&Message{Platform: "slack", ChatID: "oc_def"}); err != nil || got != beta {
		t.Fatalf("expected beta, got %v err %v", got, err)
	}
}

func TestChatIDResolverMiss(t *testing.T) {
	r := NewChatIDResolver([]ChatIDBinding{{
		Tenant: &Tenant{ID: "acme"}, Platform: "feishu", ChatIDs: []string{"oc_abc"},
	}})
	_, err := r.Resolve(&Message{Platform: "feishu", ChatID: "oc_unknown"})
	if !errors.Is(err, ErrTenantUnresolved) {
		t.Fatalf("expected ErrTenantUnresolved, got %v", err)
	}
}

func TestCompositeResolverFallback(t *testing.T) {
	acme := &Tenant{ID: "acme"}
	def := &Tenant{ID: "default"}
	comp := NewCompositeResolver([]TenantResolver{
		NewChatIDResolver([]ChatIDBinding{{Tenant: acme, Platform: "feishu", ChatIDs: []string{"oc_abc"}}}),
	}, def)
	// Hit → acme
	if got, _ := comp.Resolve(&Message{Platform: "feishu", ChatID: "oc_abc"}); got != acme {
		t.Fatalf("expected acme, got %v", got)
	}
	// Miss → fallback
	if got, _ := comp.Resolve(&Message{Platform: "feishu", ChatID: "unknown"}); got != def {
		t.Fatalf("expected default fallback, got %v", got)
	}
}

func TestLoadTenantsConfigHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.yaml")
	body := []byte(`tenants:
  - id: acme
    projectId: 4a1e5c6f-0000-0000-0000-000000000001
    name: ACME Corp
    resolvers:
      - kind: chat
        platform: feishu
        chatIds: ["oc_abc"]
      - kind: workspace
        workspaceIds: ["T0ABC"]
      - kind: domain
        domains: ["acme.com"]
    credentials:
      - providerId: feishu
        source: env
        keyPrefix: FEISHU_ACME_
  - id: beta
    projectId: 4a1e5c6f-0000-0000-0000-000000000002
    resolvers:
      - kind: chat
        chatIds: ["oc_def"]
defaultTenant: acme
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	result, err := LoadTenantsConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if result.Registry.Len() != 2 {
		t.Fatalf("expected 2 tenants, got %d", result.Registry.Len())
	}
	// Chat resolver → acme
	got, err := result.Resolver.Resolve(&Message{Platform: "feishu", ChatID: "oc_abc"})
	if err != nil || got == nil || got.ID != "acme" {
		t.Fatalf("chat resolver failed: %v %v", got, err)
	}
	// Workspace resolver → acme
	got, err = result.Resolver.Resolve(&Message{Metadata: map[string]string{"workspaceId": "T0ABC"}})
	if err != nil || got == nil || got.ID != "acme" {
		t.Fatalf("workspace resolver failed: %v %v", got, err)
	}
	// Domain resolver → acme (case-insensitive)
	got, err = result.Resolver.Resolve(&Message{Metadata: map[string]string{"domain": "ACME.com"}})
	if err != nil || got == nil || got.ID != "acme" {
		t.Fatalf("domain resolver failed: %v %v", got, err)
	}
	// Miss → default acme
	got, err = result.Resolver.Resolve(&Message{Platform: "slack", ChatID: "unknown"})
	if err != nil || got == nil || got.ID != "acme" {
		t.Fatalf("default fallback failed: %v %v", got, err)
	}
}

func TestLoadTenantsConfigRejectsDuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.yaml")
	body := []byte(`tenants:
  - id: acme
    projectId: p1
  - id: acme
    projectId: p2
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadTenantsConfig(path); err == nil {
		t.Fatal("expected duplicate id to fail")
	}
}

func TestLoadTenantsConfigRejectsUnknownDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.yaml")
	body := []byte(`tenants:
  - id: acme
    projectId: p1
defaultTenant: gamma
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadTenantsConfig(path); err == nil {
		t.Fatal("expected unknown default to fail")
	}
}
