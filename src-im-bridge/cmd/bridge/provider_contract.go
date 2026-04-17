package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

// osGetenv indirects os.Getenv so tests can stub provider override lookups.
var osGetenv = os.Getenv

type providerFeatureSet struct {
	FeishuCards *feishuProviderFeatures
}

type feishuProviderFeatures struct {
	SupportsJSONCards      bool
	SupportsTemplateCards  bool
	SupportsDelayedUpdates bool
}

type providerDescriptor struct {
	ID                      string
	Metadata                core.PlatformMetadata
	SupportedTransportModes []string
	Features                providerFeatureSet
	PlannedReason           string
	ValidateConfig          func(cfg *config, mode string) error
	NewStub                 platformFactory
	NewLive                 platformFactory
}

// Keep the older name as an alias while callers and tests migrate to the
// provider terminology.
type platformDescriptor = providerDescriptor

type activeProvider struct {
	Descriptor    providerDescriptor
	Platform      core.Platform
	TransportMode string
	// Tenants this provider binding serves (subset of the bridge's tenant
	// registry). Empty when no tenant registry is configured.
	Tenants []string
}

func (d providerDescriptor) normalizedMetadata() core.PlatformMetadata {
	return core.NormalizeMetadata(d.Metadata, d.ID)
}

func (d providerDescriptor) supportsTransport(mode string) bool {
	for _, supported := range d.SupportedTransportModes {
		if strings.EqualFold(strings.TrimSpace(supported), strings.TrimSpace(mode)) {
			return true
		}
	}
	return false
}

func (d providerDescriptor) factoryForMode(mode string) platformFactory {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case transportModeStub:
		return d.NewStub
	case transportModeLive:
		return d.NewLive
	default:
		return nil
	}
}

func (d providerDescriptor) unavailableError() error {
	if strings.TrimSpace(d.PlannedReason) != "" {
		return fmt.Errorf("selected platform %s is planned but not yet runnable; %s", d.normalizedMetadata().Source, strings.TrimSpace(d.PlannedReason))
	}
	return fmt.Errorf("selected platform %s is not runnable", d.normalizedMetadata().Source)
}

func (p *activeProvider) Metadata() core.PlatformMetadata {
	if p == nil {
		return core.PlatformMetadata{}
	}
	return p.Descriptor.normalizedMetadata()
}

func (p *activeProvider) Source() string {
	return p.Metadata().Source
}

func lookupProviderDescriptor(name string) (providerDescriptor, error) {
	normalized := core.NormalizePlatformName(name)
	descriptor, ok := providerDescriptors()[normalized]
	if !ok {
		return providerDescriptor{}, fmt.Errorf("unsupported IM_PLATFORM %q", name)
	}
	return descriptor, nil
}

func lookupPlatformDescriptor(name string) (platformDescriptor, error) {
	return lookupProviderDescriptor(name)
}

func selectProvider(cfg *config) (*activeProvider, error) {
	return selectProviderForPlatform(cfg, cfg.Platform, cfg.TransportMode)
}

func selectProviderForPlatform(cfg *config, platformID, transportMode string) (*activeProvider, error) {
	descriptor, err := lookupProviderDescriptor(platformID)
	if err != nil {
		return nil, err
	}

	mode := normalizeTransportMode(transportMode)
	if mode != transportModeStub && mode != transportModeLive {
		return nil, fmt.Errorf("unsupported IM_TRANSPORT_MODE for %s: %q", platformID, transportMode)
	}
	if !descriptor.supportsTransport(mode) {
		return nil, fmt.Errorf("selected platform %s does not support %s transport", descriptor.normalizedMetadata().Source, mode)
	}
	factory := descriptor.factoryForMode(mode)
	if factory == nil {
		return nil, descriptor.unavailableError()
	}
	if descriptor.ValidateConfig != nil {
		if err := descriptor.ValidateConfig(cfg, mode); err != nil {
			return nil, err
		}
	}

	platform, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	return &activeProvider{
		Descriptor:    descriptor,
		Platform:      platform,
		TransportMode: mode,
	}, nil
}

// selectProviders builds an activeProvider for every entry in IM_PLATFORMS
// (comma-separated). It preserves legacy IM_PLATFORM behavior by falling
// back to a single-element list when IM_PLATFORMS is empty. All listed
// providers MUST be valid; a failure in any of them fails the whole
// selection so the bridge does not come up partially.
func selectProviders(cfg *config, platforms []string) ([]*activeProvider, error) {
	if len(platforms) == 0 {
		platforms = []string{cfg.Platform}
	}
	out := make([]*activeProvider, 0, len(platforms))
	seen := map[string]struct{}{}
	for _, p := range platforms {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		normalized := core.NormalizePlatformName(name)
		if _, dup := seen[normalized]; dup {
			return nil, fmt.Errorf("provider %q declared twice in IM_PLATFORMS", name)
		}
		seen[normalized] = struct{}{}
		// Per-provider transport override: IM_TRANSPORT_MODE_<PROVIDER> with
		// the normalized id (UPPERCASE) overrides the shared IM_TRANSPORT_MODE.
		mode := cfg.TransportMode
		if override := strings.TrimSpace(providerTransportOverride(normalized)); override != "" {
			mode = override
		}
		provider, err := selectProviderForPlatform(cfg, name, mode)
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", name, err)
		}
		out = append(out, provider)
	}
	return out, nil
}

// providerTransportOverride reads IM_TRANSPORT_MODE_<PROVIDER> (e.g.
// IM_TRANSPORT_MODE_FEISHU). Exposed as a hook so tests can stub the env.
var providerTransportOverride = func(normalized string) string {
	return osGetenv("IM_TRANSPORT_MODE_" + strings.ToUpper(normalized))
}
