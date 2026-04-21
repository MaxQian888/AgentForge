package config

import "testing"

func TestLoadReadsPluginRegistryURL(t *testing.T) {
	t.Setenv("PLUGIN_REGISTRY_URL", "https://registry.agentforge.dev")

	cfg := Load()

	if cfg.PluginRegistryURL != "https://registry.agentforge.dev" {
		t.Fatalf("PluginRegistryURL = %q, want https://registry.agentforge.dev", cfg.PluginRegistryURL)
	}
}

func TestConfig_LogLevel_FromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "warn")
	cfg := Load()
	if cfg.LogLevel != "warn" {
		t.Fatalf("want warn, got %q", cfg.LogLevel)
	}
}

func TestConfig_LogLevel_EmptyWhenUnset(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	cfg := Load()
	if cfg.LogLevel != "" {
		t.Fatalf("want empty, got %q", cfg.LogLevel)
	}
}
