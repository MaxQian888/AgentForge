package main

import (
	"context"
	"fmt"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core/plugin"
)

// RegistrationInventory is the result of scanning the Bridge's active
// providers and loaded command plugins into the shapes expected by
// client.BridgeRegistration.Providers / CommandPlugins.
type RegistrationInventory struct {
	Providers      []client.BridgeProvider
	CommandPlugins []client.BridgeCommandPlugin
}

// buildRegistrationInventory assembles the multi-provider + command-plugin
// snapshot the orchestrator displays. The activeProvider slice is the full
// list of providers this Bridge process is hosting; pluginReg may be nil
// when IM_BRIDGE_PLUGIN_DIR is unset.
func buildRegistrationInventory(providers []*activeProvider, pluginReg *plugin.Registry) RegistrationInventory {
	inv := RegistrationInventory{}
	for _, p := range providers {
		if p == nil || p.Platform == nil {
			continue
		}
		md := p.Metadata()
		inv.Providers = append(inv.Providers, client.BridgeProvider{
			ID:               md.Source,
			Transport:        p.TransportMode,
			ReadinessTier:    string(md.Capabilities.ReadinessTier),
			CapabilityMatrix: md.Capabilities.Matrix(),
			CallbackPaths:    collectCallbackPaths(p),
			Tenants:          append([]string(nil), p.Tenants...),
			MetadataSource:   "builtin",
		})
	}
	if pluginReg != nil {
		for _, p := range pluginReg.Snapshot() {
			inv.CommandPlugins = append(inv.CommandPlugins, client.BridgeCommandPlugin{
				ID:         p.ID,
				Version:    p.Version,
				Commands:   append([]string(nil), p.Commands...),
				Tenants:    append([]string(nil), p.Tenants...),
				SourcePath: p.SourcePath,
			})
		}
	}
	return inv
}

// collectCallbackPaths reuses the Platform's optional callbackPathProvider
// interface (already used by bridgeRuntimeControl.Start) so inventory
// reporting and register payload stay in sync.
func collectCallbackPaths(p *activeProvider) []string {
	out := []string{"/im/notify", "/im/send"}
	if prov, ok := p.Platform.(callbackPathProvider); ok {
		for _, path := range prov.CallbackPaths() {
			if path != "" {
				out = append(out, path)
			}
		}
	}
	return out
}

// registerBridgeInventory calls /im/bridge/register once for this Bridge
// process with the full provider and command-plugin inventory. It replaces
// the per-RuntimeControl Start() registration that overwrote itself in
// multi-provider deployments.
func registerBridgeInventory(ctx context.Context, cl *client.AgentForgeClient, bridgeID string, cfg *config, providers []*activeProvider, pluginReg *plugin.Registry) error {
	if cl == nil || bridgeID == "" || len(providers) == 0 {
		return nil
	}
	primary := providers[0]
	md := primary.Metadata()
	inv := buildRegistrationInventory(providers, pluginReg)

	tenantIDs := append([]string(nil), primary.Tenants...)
	tenantManifest := buildTenantManifestFromProviders(providers)

	registration := client.BridgeRegistration{
		BridgeID:       bridgeID,
		Platform:       md.Source,
		Transport:      primary.TransportMode,
		ProjectIDs:     []string{cfg.ProjectID},
		Tenants:        tenantIDs,
		TenantManifest: tenantManifest,
		Capabilities: map[string]bool{
			"supports_deferred_reply":  md.Capabilities.SupportsDeferredReply,
			"supports_rich_messages":   md.Capabilities.SupportsRichMessages,
			"requires_public_callback": md.Capabilities.RequiresPublicCallback,
			"supports_mentions":        md.Capabilities.SupportsMentions,
			"supports_slash_commands":  md.Capabilities.SupportsSlashCommands,
		},
		CapabilityMatrix: md.Capabilities.Matrix(),
		CallbackPaths:    collectCallbackPaths(primary),
		Metadata: map[string]string{
			"platform_name":  primary.Platform.Name(),
			"provider_id":    primary.Descriptor.ID,
			"readiness_tier": string(md.Capabilities.ReadinessTier),
		},
		Providers:      inv.Providers,
		CommandPlugins: inv.CommandPlugins,
	}
	if preferredMode := string(md.Capabilities.PreferredAsyncUpdateMode); preferredMode != "" {
		registration.Metadata["preferred_async_update_mode"] = preferredMode
	}
	if fallbackMode := string(md.Capabilities.FallbackAsyncUpdateMode); fallbackMode != "" {
		registration.Metadata["fallback_async_update_mode"] = fallbackMode
	}

	if _, err := cl.RegisterBridge(ctx, registration); err != nil {
		return fmt.Errorf("register bridge inventory: %w", err)
	}
	return nil
}

// buildTenantManifestFromProviders consolidates the per-provider tenant
// slices into a single []client.TenantBinding for the top-level payload.
// Duplicate (id, projectId) pairs are suppressed.
func buildTenantManifestFromProviders(providers []*activeProvider) []client.TenantBinding {
	seen := map[string]struct{}{}
	var out []client.TenantBinding
	for _, p := range providers {
		for _, tid := range p.Tenants {
			key := tid
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			// ProjectID is not carried on activeProvider per-tenant; leave
			// empty. The orchestrator resolves projectId separately.
			out = append(out, client.TenantBinding{ID: tid})
		}
	}
	return out
}
