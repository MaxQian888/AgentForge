package client

import (
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestClientFactoryForReturnsTenantScope(t *testing.T) {
	t.Setenv("ACME_API_KEY", "acme-key")
	acme := &core.Tenant{
		ID:        "acme",
		ProjectID: "acme-project",
		Credentials: []core.CredentialRef{
			{ProviderID: "agentforge", Source: "env", KeyPrefix: "ACME_"},
		},
	}
	reg, err := core.NewTenantRegistry([]*core.Tenant{acme})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	f := NewClientFactory(FactoryOptions{
		BaseURL:        "http://localhost",
		DefaultProject: "default-project",
		DefaultAPIKey:  "default-key",
		Registry:       reg,
	})

	scoped := f.For("acme")
	if got := scoped.ProjectScope(); got != "acme-project" {
		t.Fatalf("expected acme-project scope, got %q", got)
	}
	if got := scoped.apiKey; got != "acme-key" {
		t.Fatalf("expected acme api key, got %q", got)
	}

	// Unknown tenant falls back to defaults.
	fallback := f.For("unknown")
	if got := fallback.ProjectScope(); got != "default-project" {
		t.Fatalf("expected default-project scope, got %q", got)
	}
	if got := fallback.apiKey; got != "default-key" {
		t.Fatalf("expected default api key, got %q", got)
	}
}

func TestAgentForgeClientSatisfiesClientProvider(t *testing.T) {
	c := NewAgentForgeClient("http://localhost", "proj", "key")
	var _ ClientProvider = c
	if got := c.For("whatever"); got != c {
		t.Fatal("legacy For should return receiver")
	}
}
