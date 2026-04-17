package client

import (
	"os"
	"strings"
	"sync"

	"github.com/agentforge/im-bridge/core"
)

// ClientProvider is the minimal interface Register* functions accept for
// producing per-tenant AgentForgeClient instances. Both *ClientFactory
// and *AgentForgeClient satisfy it — the concrete *AgentForgeClient
// returns itself from For(_) so legacy call sites and tests that construct
// a single client directly continue to work.
type ClientProvider interface {
	For(tenantID string) *AgentForgeClient
}

// For implements ClientProvider on the legacy single-client path. It
// ignores the tenant id and returns the receiver unchanged, preserving
// today's single-tenant semantics.
func (c *AgentForgeClient) For(_ string) *AgentForgeClient { return c }

// ClientFactory produces AgentForgeClient instances scoped to a specific
// tenant. It replaces the single-process, single-project "one client
// instance for the whole bridge" model with an explicit factory so the
// runtime can route inbound messages through the right project + api key.
//
// The factory is concurrency-safe and idempotent: For(tenantID) with the
// same id returns a fresh clone every time so callers may further specialize
// it (WithSource, WithBridgeContext) without mutating shared state.
type ClientFactory struct {
	mu             sync.RWMutex
	baseURL        string
	defaultProject string
	defaultAPIKey  string
	defaultSource  string
	registry       *core.TenantRegistry
}

// FactoryOptions configures a new ClientFactory.
type FactoryOptions struct {
	BaseURL        string
	DefaultProject string
	DefaultAPIKey  string
	DefaultSource  string
	Registry       *core.TenantRegistry
}

// NewClientFactory builds a factory bound to the given backend + default
// project / api key. `registry` may be nil when no tenants are configured
// (legacy single-tenant mode).
func NewClientFactory(opts FactoryOptions) *ClientFactory {
	return &ClientFactory{
		baseURL:        opts.BaseURL,
		defaultProject: strings.TrimSpace(opts.DefaultProject),
		defaultAPIKey:  opts.DefaultAPIKey,
		defaultSource:  strings.TrimSpace(opts.DefaultSource),
		registry:       opts.Registry,
	}
}

// SetTenantRegistry swaps the backing registry (for SIGHUP reload).
func (f *ClientFactory) SetTenantRegistry(r *core.TenantRegistry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registry = r
}

// Registry returns the current registry (may be nil).
func (f *ClientFactory) Registry() *core.TenantRegistry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.registry
}

// Default returns a client bound to the factory's default project + api
// key, tagged with the default source if one was configured. It is the
// "no tenant resolved" fallback client and is equivalent to For("").
func (f *ClientFactory) Default() *AgentForgeClient {
	return f.For("")
}

// For returns a fresh AgentForgeClient scoped to the given tenant id. When
// the id matches a tenant in the registry, the client is scoped to that
// tenant's projectId and the tenant-specific API key (resolved from the
// tenant's credentials[] or falling back to the default key when absent).
// An empty or unknown id falls back to the factory defaults.
func (f *ClientFactory) For(tenantID string) *AgentForgeClient {
	f.mu.RLock()
	defer f.mu.RUnlock()
	project := f.defaultProject
	apiKey := f.defaultAPIKey
	if f.registry != nil {
		if tenant := f.registry.Get(strings.TrimSpace(tenantID)); tenant != nil {
			if scoped := strings.TrimSpace(tenant.ProjectID); scoped != "" {
				project = scoped
			}
			if key := resolveTenantAPIKey(tenant); key != "" {
				apiKey = key
			}
		}
	}
	client := NewAgentForgeClient(f.baseURL, project, apiKey)
	if f.defaultSource != "" {
		client = client.WithSource(f.defaultSource)
	}
	return client
}

// ForTenant is a convenience helper returning a client scoped to the tenant
// named in a resolved *core.Tenant.
func (f *ClientFactory) ForTenant(tenant *core.Tenant) *AgentForgeClient {
	if tenant == nil {
		return f.Default()
	}
	return f.For(tenant.ID)
}

// resolveTenantAPIKey resolves a tenant-specific API key from the tenant's
// declared credentials. We look for a credential entry whose ProviderID is
// either empty (meaning "applies to all backends") or matches the special
// marker "agentforge" for the backend API key itself. The credential's
// source=env with KeyPrefix=ACME_ means we read ACME_API_KEY.
func resolveTenantAPIKey(tenant *core.Tenant) string {
	if tenant == nil {
		return ""
	}
	for _, cred := range tenant.Credentials {
		kind := strings.ToLower(strings.TrimSpace(cred.ProviderID))
		if kind != "" && kind != "agentforge" && kind != "api" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(cred.Source)) {
		case "env":
			if prefix := strings.TrimSpace(cred.KeyPrefix); prefix != "" {
				if v := strings.TrimSpace(os.Getenv(prefix + "API_KEY")); v != "" {
					return v
				}
			}
		case "file":
			if path := strings.TrimSpace(cred.Path); path != "" {
				if raw, err := os.ReadFile(path); err == nil {
					return strings.TrimSpace(string(raw))
				}
			}
		}
	}
	return ""
}
