package main

import (
	"strings"
	"testing"
)

func TestParsePlatformListCompatibility(t *testing.T) {
	if got := parsePlatformList("", "feishu"); len(got) != 1 || got[0] != "feishu" {
		t.Fatalf("expected legacy fallback [feishu], got %v", got)
	}
	got := parsePlatformList("feishu, dingtalk ,wecom", "ignored")
	if len(got) != 3 || got[0] != "feishu" || got[1] != "dingtalk" || got[2] != "wecom" {
		t.Fatalf("unexpected parse: %v", got)
	}
	if got := parsePlatformList(" , , ", "fallback"); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestSelectProvidersDuplicateRejected(t *testing.T) {
	cfg := &config{
		Platform:      "feishu",
		TransportMode: transportModeStub,
		FeishuApp:     "app",
		FeishuSec:     "sec",
	}
	_, err := selectProviders(cfg, []string{"feishu", "feishu"})
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestSelectProvidersSingleFallback(t *testing.T) {
	cfg := &config{
		Platform:      "feishu",
		TransportMode: transportModeStub,
	}
	providers, err := selectProviders(cfg, nil)
	if err != nil {
		t.Fatalf("selectProviders: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Descriptor.ID != "feishu" {
		t.Fatalf("expected feishu, got %s", providers[0].Descriptor.ID)
	}
}

func TestOffsetPort(t *testing.T) {
	if got := offsetPort("7779", 0); got != "7779" {
		t.Fatalf("offset 0: got %s", got)
	}
	if got := offsetPort("7779", 2); got != "7781" {
		t.Fatalf("offset 2: got %s", got)
	}
	if got := offsetPort("named", 5); got != "named" {
		t.Fatalf("non-numeric passes through: got %s", got)
	}
}

func TestProviderSpecificSecretPrefersOverride(t *testing.T) {
	t.Setenv("IM_SECRET_FEISHU", "feishu-override")
	if got := providerSpecificSecret("feishu", "shared"); got != "feishu-override" {
		t.Fatalf("expected override, got %s", got)
	}
	if got := providerSpecificSecret("dingtalk", "shared"); got != "shared" {
		t.Fatalf("expected shared, got %s", got)
	}
}
