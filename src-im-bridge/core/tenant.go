package core

import (
	"errors"
	"fmt"
	"strings"
)

// CredentialRef points the runtime at a tenant-scoped credential source.
// Only the `env` and `file` kinds are implemented in this change; future
// kinds (vault, kms, etc.) plug in here without callers changing.
type CredentialRef struct {
	// ProviderID is the normalized IM provider id this credential is for.
	ProviderID string `yaml:"providerId" json:"providerId"`
	// Source selects the resolution strategy ("env", "file").
	Source string `yaml:"source" json:"source"`
	// KeyPrefix is used by the "env" source. Individual callers append the
	// tail (e.g. "APP_ID", "APP_SECRET") to this prefix.
	KeyPrefix string `yaml:"keyPrefix,omitempty" json:"keyPrefix,omitempty"`
	// Path is used by the "file" source.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}

// Tenant is the in-memory representation of a routed tenant. Tenants are
// not business entities owned by the bridge — the authoritative source is
// the backend. The bridge just needs enough to (a) resolve inbound messages
// to a tenant and (b) dispatch outbound calls with that tenant's project
// scope and credentials.
type Tenant struct {
	ID          string            `yaml:"id" json:"id"`
	ProjectID   string            `yaml:"projectId" json:"projectId"`
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	Credentials []CredentialRef   `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	PluginScope []string          `yaml:"plugins,omitempty" json:"plugins,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// CredentialFor returns the tenant's credential reference for the given
// provider id or nil when the tenant does not declare one.
func (t *Tenant) CredentialFor(providerID string) *CredentialRef {
	if t == nil {
		return nil
	}
	providerID = NormalizePlatformName(providerID)
	for i := range t.Credentials {
		if NormalizePlatformName(t.Credentials[i].ProviderID) == providerID {
			return &t.Credentials[i]
		}
	}
	return nil
}

// TenantResolver maps an inbound message to the tenant that owns it. A
// resolver MUST be deterministic: calling Resolve twice with the same
// message MUST return the same tenant (or the same miss). Resolvers MUST
// be safe for concurrent use.
type TenantResolver interface {
	Resolve(msg *Message) (*Tenant, error)
}

// ErrTenantUnresolved signals that no resolver could map a message to a
// tenant. Callers may choose to fall back to a default tenant or reject the
// message outright; this sentinel lets them distinguish resolver misses
// from configuration errors.
var ErrTenantUnresolved = errors.New("tenant unresolved")

// ChatIDResolver maps platform + chat id to a tenant. The key format is
// "<platform>:<chatId>" (both lowercased); entries that omit platform match
// any platform for that chat id.
type ChatIDResolver struct {
	mapping map[string]*Tenant
}

// NewChatIDResolver builds a resolver from a list of bindings. Each binding
// declares a tenant and one or more (platform, chatId) pairs it owns.
func NewChatIDResolver(bindings []ChatIDBinding) *ChatIDResolver {
	r := &ChatIDResolver{mapping: map[string]*Tenant{}}
	for i := range bindings {
		b := bindings[i]
		if b.Tenant == nil {
			continue
		}
		platform := strings.ToLower(strings.TrimSpace(b.Platform))
		for _, chatID := range b.ChatIDs {
			chatID = strings.TrimSpace(chatID)
			if chatID == "" {
				continue
			}
			if platform != "" {
				r.mapping[platform+":"+chatID] = b.Tenant
			}
			r.mapping[":"+chatID] = b.Tenant
		}
	}
	return r
}

// ChatIDBinding binds a tenant to a set of chat ids on an optional platform.
type ChatIDBinding struct {
	Tenant   *Tenant
	Platform string // optional; empty binds on any platform
	ChatIDs  []string
}

// Resolve implements TenantResolver.
func (r *ChatIDResolver) Resolve(msg *Message) (*Tenant, error) {
	if r == nil || msg == nil {
		return nil, ErrTenantUnresolved
	}
	chat := strings.TrimSpace(msg.ChatID)
	if chat == "" {
		return nil, ErrTenantUnresolved
	}
	platform := strings.ToLower(strings.TrimSpace(msg.Platform))
	if t, ok := r.mapping[platform+":"+chat]; ok {
		return t, nil
	}
	if t, ok := r.mapping[":"+chat]; ok {
		return t, nil
	}
	return nil, ErrTenantUnresolved
}

// WorkspaceResolver maps a workspace id (Slack team id, Feishu tenant_key,
// Discord guild id, etc.) to a tenant. The workspace id is read from
// msg.Metadata["workspaceId"]; adapters populate that field during message
// normalization.
type WorkspaceResolver struct {
	mapping map[string]*Tenant
}

// NewWorkspaceResolver builds a resolver from a flat (workspaceId → tenant)
// mapping.
func NewWorkspaceResolver(mapping map[string]*Tenant) *WorkspaceResolver {
	r := &WorkspaceResolver{mapping: map[string]*Tenant{}}
	for k, v := range mapping {
		key := strings.TrimSpace(k)
		if key == "" || v == nil {
			continue
		}
		r.mapping[key] = v
	}
	return r
}

// Resolve implements TenantResolver.
func (r *WorkspaceResolver) Resolve(msg *Message) (*Tenant, error) {
	if r == nil || msg == nil || msg.Metadata == nil {
		return nil, ErrTenantUnresolved
	}
	ws := strings.TrimSpace(msg.Metadata["workspaceId"])
	if ws == "" {
		return nil, ErrTenantUnresolved
	}
	if t, ok := r.mapping[ws]; ok {
		return t, nil
	}
	return nil, ErrTenantUnresolved
}

// DomainResolver maps an email / workspace domain to a tenant. The domain
// is read from msg.Metadata["domain"].
type DomainResolver struct {
	mapping map[string]*Tenant
}

// NewDomainResolver builds a resolver from a (domain → tenant) mapping.
func NewDomainResolver(mapping map[string]*Tenant) *DomainResolver {
	r := &DomainResolver{mapping: map[string]*Tenant{}}
	for k, v := range mapping {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" || v == nil {
			continue
		}
		r.mapping[key] = v
	}
	return r
}

// Resolve implements TenantResolver.
func (r *DomainResolver) Resolve(msg *Message) (*Tenant, error) {
	if r == nil || msg == nil || msg.Metadata == nil {
		return nil, ErrTenantUnresolved
	}
	dom := strings.ToLower(strings.TrimSpace(msg.Metadata["domain"]))
	if dom == "" {
		return nil, ErrTenantUnresolved
	}
	if t, ok := r.mapping[dom]; ok {
		return t, nil
	}
	return nil, ErrTenantUnresolved
}

// CompositeResolver walks a list of resolvers and returns the first
// successful match. It exists so the runtime can combine chat-id, workspace,
// and domain strategies behind a single entry point.
type CompositeResolver struct {
	resolvers []TenantResolver
	fallback  *Tenant
}

// NewCompositeResolver builds a resolver that tries each underlying resolver
// in order and, on miss, returns `fallback` when it is non-nil.
func NewCompositeResolver(resolvers []TenantResolver, fallback *Tenant) *CompositeResolver {
	return &CompositeResolver{resolvers: resolvers, fallback: fallback}
}

// Resolve implements TenantResolver.
func (r *CompositeResolver) Resolve(msg *Message) (*Tenant, error) {
	if r == nil || msg == nil {
		return nil, ErrTenantUnresolved
	}
	for _, sub := range r.resolvers {
		if sub == nil {
			continue
		}
		if t, err := sub.Resolve(msg); err == nil && t != nil {
			return t, nil
		}
	}
	if r.fallback != nil {
		return r.fallback, nil
	}
	return nil, ErrTenantUnresolved
}

// TenantRegistry is the authoritative in-process collection of configured
// tenants. It provides O(1) lookup by id and exposes a read-only snapshot
// so downstream consumers (client factory, control plane registration)
// can iterate without racing against reloads.
type TenantRegistry struct {
	byID  map[string]*Tenant
	order []string
}

// NewTenantRegistry builds a registry from a slice of tenants. It returns
// an error when duplicate ids are present.
func NewTenantRegistry(tenants []*Tenant) (*TenantRegistry, error) {
	r := &TenantRegistry{
		byID:  map[string]*Tenant{},
		order: make([]string, 0, len(tenants)),
	}
	for _, t := range tenants {
		if t == nil {
			continue
		}
		id := strings.TrimSpace(t.ID)
		if id == "" {
			return nil, errors.New("tenant: empty id")
		}
		if _, exists := r.byID[id]; exists {
			return nil, fmt.Errorf("tenant: duplicate id %q", id)
		}
		r.byID[id] = t
		r.order = append(r.order, id)
	}
	return r, nil
}

// Get returns the tenant with the given id or nil when absent.
func (r *TenantRegistry) Get(id string) *Tenant {
	if r == nil {
		return nil
	}
	return r.byID[strings.TrimSpace(id)]
}

// All returns the tenants in insertion order.
func (r *TenantRegistry) All() []*Tenant {
	if r == nil {
		return nil
	}
	out := make([]*Tenant, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}

// IDs returns the tenant ids in insertion order.
func (r *TenantRegistry) IDs() []string {
	if r == nil {
		return nil
	}
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Len returns the number of tenants.
func (r *TenantRegistry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.order)
}
