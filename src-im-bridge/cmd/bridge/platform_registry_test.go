package main

import "testing"

func TestNormalizeTransportMode_DefaultsToStub(t *testing.T) {
	if got := normalizeTransportMode("   "); got != transportModeStub {
		t.Fatalf("normalizeTransportMode = %q", got)
	}
	if got := normalizeTransportMode(" LIVE "); got != transportModeLive {
		t.Fatalf("normalizeTransportMode = %q", got)
	}
}

func TestTelegramValidateConfig_RejectsUnsupportedModes(t *testing.T) {
	if err := telegramValidateConfig("webhook", ""); err == nil {
		t.Fatal("expected unsupported update mode to fail")
	}
	if err := telegramValidateConfig("longpoll", "https://example.test/webhook"); err == nil {
		t.Fatal("expected longpoll + webhook url to fail")
	}
	if err := telegramValidateConfig("", ""); err != nil {
		t.Fatalf("telegramValidateConfig default longpoll error: %v", err)
	}
}

func TestLookupPlatformDescriptor_RejectsUnknownPlatform(t *testing.T) {
	if _, err := lookupPlatformDescriptor("unknown-platform"); err == nil {
		t.Fatal("expected unknown platform to fail")
	}
}
