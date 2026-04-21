package main

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

const (
	transportModeStub = core.TransportModeStub
	transportModeLive = core.TransportModeLive
)

type platformFactory func(cfg *config) (core.Platform, error)

func normalizeTransportMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return transportModeStub
	}
	return normalized
}

// platformDescriptors is retained as a name for tests that enumerate
// every built-in provider descriptor. It now sources from the global
// registry instead of a hand-maintained switch.
func platformDescriptors() map[string]platformDescriptor {
	return providerDescriptorsFromRegistry()
}

func providerDescriptors() map[string]providerDescriptor {
	return providerDescriptorsFromRegistry()
}

func providerDescriptorsFromRegistry() map[string]providerDescriptor {
	out := map[string]providerDescriptor{}
	for _, f := range core.RegisteredProviders() {
		out[f.ID] = adaptFactoryToDescriptor(f)
	}
	return out
}

// adaptFactoryToDescriptor converts the opaque core.ProviderFactory produced
// by each provider package's init() into the providerDescriptor shape that
// the multi-provider wiring in main.go and selectProvider consume.
func adaptFactoryToDescriptor(f core.ProviderFactory) providerDescriptor {
	return providerDescriptor{
		ID:                      f.ID,
		Metadata:                f.Metadata,
		SupportedTransportModes: append([]string(nil), f.SupportedTransportModes...),
		Features:                legacyFeaturesForProvider(f.ID),
		ValidateConfig: func(cfg *config, mode string) error {
			if f.ValidateConfig == nil {
				return nil
			}
			return f.ValidateConfig(newCfgProviderEnv(cfg, f.EnvPrefixes), mode)
		},
		NewStub: func(cfg *config) (core.Platform, error) {
			if f.NewStub == nil {
				return nil, fmt.Errorf("provider %s has no stub factory", f.ID)
			}
			return f.NewStub(newCfgProviderEnv(cfg, f.EnvPrefixes))
		},
		NewLive: func(cfg *config) (core.Platform, error) {
			if f.NewLive == nil {
				return nil, fmt.Errorf("provider %s has no live factory", f.ID)
			}
			return f.NewLive(newCfgProviderEnv(cfg, f.EnvPrefixes))
		},
		envPrefixes: append([]string(nil), f.EnvPrefixes...),
	}
}

// legacyFeaturesForProvider returns the provider-specific feature set the
// old providerDescriptors() switch assigned. This is a temporary bridge:
// feishuProviderFeatures lives in cmd/bridge (not reachable from the feishu
// package), so the adapter re-attaches it here. If a second provider ever
// publishes feature data, promote providerFeatureSet to core and have each
// register.go set its own Features field directly.
func legacyFeaturesForProvider(id string) providerFeatureSet {
	switch id {
	case "feishu":
		return providerFeatureSet{
			FeishuCards: &feishuProviderFeatures{
				SupportsJSONCards:      true,
				SupportsTemplateCards:  true,
				SupportsDelayedUpdates: true,
			},
		}
	}
	return providerFeatureSet{}
}

func telegramValidateConfig(updateMode, webhookURL string) error {
	normalized := strings.ToLower(strings.TrimSpace(updateMode))
	if normalized == "" {
		normalized = "longpoll"
	}
	if normalized != "longpoll" {
		return fmt.Errorf("telegram live transport currently supports only longpoll update mode")
	}
	if strings.TrimSpace(webhookURL) != "" {
		return fmt.Errorf("telegram long polling cannot be combined with webhook configuration")
	}
	return nil
}
