package config

import "testing"

func TestLoadReadsPluginRegistryURL(t *testing.T) {
	t.Setenv("PLUGIN_REGISTRY_URL", "https://registry.agentforge.dev")

	cfg := Load()

	if cfg.PluginRegistryURL != "https://registry.agentforge.dev" {
		t.Fatalf("PluginRegistryURL = %q, want https://registry.agentforge.dev", cfg.PluginRegistryURL)
	}
}
