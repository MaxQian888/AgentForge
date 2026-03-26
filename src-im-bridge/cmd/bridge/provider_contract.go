package main

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

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
	descriptor, err := lookupProviderDescriptor(cfg.Platform)
	if err != nil {
		return nil, err
	}

	mode := normalizeTransportMode(cfg.TransportMode)
	if mode != transportModeStub && mode != transportModeLive {
		return nil, fmt.Errorf("unsupported IM_TRANSPORT_MODE %q", cfg.TransportMode)
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
