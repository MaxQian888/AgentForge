package core

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// TenantsConfig is the root shape of the tenants configuration file
// referenced by IM_TENANTS_CONFIG.
//
// Example:
//
//	tenants:
//	  - id: acme
//	    projectId: 4a1e5c6f-...
//	    name: ACME Corp
//	    resolvers:
//	      - kind: chat
//	        platform: feishu
//	        chatIds: [oc_abc123, oc_def456]
//	      - kind: workspace
//	        workspaceIds: [T0ABC]
//	    credentials:
//	      - providerId: feishu
//	        source: env
//	        keyPrefix: FEISHU_ACME_
//	    plugins:
//	      - "@acme/jira-commands"
//	defaultTenant: acme
type TenantsConfig struct {
	Tenants       []tenantEntry `yaml:"tenants"`
	DefaultTenant string        `yaml:"defaultTenant,omitempty"`
}

type tenantEntry struct {
	ID          string            `yaml:"id"`
	ProjectID   string            `yaml:"projectId"`
	Name        string            `yaml:"name,omitempty"`
	Resolvers   []resolverEntry   `yaml:"resolvers,omitempty"`
	Credentials []CredentialRef   `yaml:"credentials,omitempty"`
	Plugins     []string          `yaml:"plugins,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

type resolverEntry struct {
	Kind         string   `yaml:"kind"`
	Platform     string   `yaml:"platform,omitempty"`
	ChatIDs      []string `yaml:"chatIds,omitempty"`
	WorkspaceIDs []string `yaml:"workspaceIds,omitempty"`
	Domains      []string `yaml:"domains,omitempty"`
}

// TenantLoadResult captures everything the runtime needs from a parsed
// tenants file: the registry itself and a composite resolver combining all
// per-tenant resolver bindings.
type TenantLoadResult struct {
	Registry *TenantRegistry
	Resolver TenantResolver
	Default  *Tenant
}

// LoadTenantsConfig reads the YAML file at `path` and builds a registry +
// composite resolver. An empty path returns an empty result (legacy single-
// tenant mode); callers treat that as the tenant feature being disabled.
// The env variable IM_TENANT_DEFAULT, when set, overrides DefaultTenant.
func LoadTenantsConfig(path string) (*TenantLoadResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return &TenantLoadResult{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tenant: read %s: %w", path, err)
	}
	var cfg TenantsConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("tenant: parse %s: %w", path, err)
	}
	if len(cfg.Tenants) == 0 {
		return nil, errors.New("tenant: config contains no tenants")
	}

	tenants := make([]*Tenant, 0, len(cfg.Tenants))
	chatBindings := []ChatIDBinding{}
	workspaceMap := map[string]*Tenant{}
	domainMap := map[string]*Tenant{}

	for idx := range cfg.Tenants {
		entry := cfg.Tenants[idx]
		id := strings.TrimSpace(entry.ID)
		projectID := strings.TrimSpace(entry.ProjectID)
		if id == "" {
			return nil, fmt.Errorf("tenant: entry %d missing id", idx)
		}
		if projectID == "" {
			return nil, fmt.Errorf("tenant %q: projectId is required", id)
		}
		t := &Tenant{
			ID:          id,
			ProjectID:   projectID,
			Name:        strings.TrimSpace(entry.Name),
			Credentials: append([]CredentialRef(nil), entry.Credentials...),
			PluginScope: append([]string(nil), entry.Plugins...),
			Metadata:    map[string]string{},
		}
		for k, v := range entry.Metadata {
			t.Metadata[k] = v
		}
		tenants = append(tenants, t)

		for _, r := range entry.Resolvers {
			kind := strings.ToLower(strings.TrimSpace(r.Kind))
			switch kind {
			case "chat", "chat_id", "chatid":
				if len(r.ChatIDs) == 0 {
					return nil, fmt.Errorf("tenant %q: chat resolver has no chatIds", id)
				}
				chatBindings = append(chatBindings, ChatIDBinding{
					Tenant:   t,
					Platform: strings.TrimSpace(r.Platform),
					ChatIDs:  append([]string(nil), r.ChatIDs...),
				})
			case "workspace", "workspace_id":
				for _, ws := range r.WorkspaceIDs {
					ws = strings.TrimSpace(ws)
					if ws == "" {
						continue
					}
					if prev, ok := workspaceMap[ws]; ok && prev.ID != t.ID {
						return nil, fmt.Errorf("tenant %q: workspace %q already bound to %q", id, ws, prev.ID)
					}
					workspaceMap[ws] = t
				}
			case "domain":
				for _, d := range r.Domains {
					d = strings.ToLower(strings.TrimSpace(d))
					if d == "" {
						continue
					}
					if prev, ok := domainMap[d]; ok && prev.ID != t.ID {
						return nil, fmt.Errorf("tenant %q: domain %q already bound to %q", id, d, prev.ID)
					}
					domainMap[d] = t
				}
			default:
				return nil, fmt.Errorf("tenant %q: unsupported resolver kind %q", id, r.Kind)
			}
		}
	}

	registry, err := NewTenantRegistry(tenants)
	if err != nil {
		return nil, err
	}

	defaultID := strings.TrimSpace(os.Getenv("IM_TENANT_DEFAULT"))
	if defaultID == "" {
		defaultID = strings.TrimSpace(cfg.DefaultTenant)
	}
	var fallback *Tenant
	if defaultID != "" {
		fallback = registry.Get(defaultID)
		if fallback == nil {
			return nil, fmt.Errorf("tenant: default %q not present in tenants list", defaultID)
		}
	}

	sub := []TenantResolver{}
	if len(chatBindings) > 0 {
		sub = append(sub, NewChatIDResolver(chatBindings))
	}
	if len(workspaceMap) > 0 {
		sub = append(sub, NewWorkspaceResolver(workspaceMap))
	}
	if len(domainMap) > 0 {
		sub = append(sub, NewDomainResolver(domainMap))
	}

	var resolver TenantResolver
	switch {
	case len(sub) == 0 && fallback != nil:
		resolver = NewCompositeResolver(nil, fallback)
	case len(sub) > 0:
		resolver = NewCompositeResolver(sub, fallback)
	default:
		// No resolvers declared and no default tenant: every inbound message
		// will be rejected. The runtime can still use the registry for
		// registration payload enumeration.
		resolver = NewCompositeResolver(nil, nil)
	}

	return &TenantLoadResult{
		Registry: registry,
		Resolver: resolver,
		Default:  fallback,
	}, nil
}
