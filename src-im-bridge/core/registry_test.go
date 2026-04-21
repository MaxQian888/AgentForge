package core

import (
	"testing"
)

func resetProviderRegistry() {
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	providerRegistry = map[string]ProviderFactory{}
}

func resetRegistry(t *testing.T) {
	t.Helper()
	resetProviderRegistry()
}

func TestRegisterAndLookupProvider(t *testing.T) {
	resetRegistry(t)

	f := ProviderFactory{
		ID:                      "unittest-platform",
		SupportedTransportModes: []string{TransportModeStub},
		EnvPrefixes:             []string{"UNITTEST_"},
		NewStub: func(env ProviderEnv) (Platform, error) {
			return nil, nil
		},
	}
	RegisterProvider(f)

	got, ok := LookupProvider("unittest-platform")
	if !ok {
		t.Fatalf("LookupProvider unittest-platform: not found")
	}
	if got.ID != "unittest-platform" {
		t.Errorf("ID = %q, want unittest-platform", got.ID)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	resetRegistry(t)
	RegisterProvider(ProviderFactory{
		ID:                      "dup",
		SupportedTransportModes: []string{TransportModeStub},
		NewStub:                 func(env ProviderEnv) (Platform, error) { return nil, nil },
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	RegisterProvider(ProviderFactory{
		ID:                      "dup",
		SupportedTransportModes: []string{TransportModeStub},
		NewStub:                 func(env ProviderEnv) (Platform, error) { return nil, nil },
	})
}

func TestRegisterEmptyIDPanics(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty id")
		}
	}()
	RegisterProvider(ProviderFactory{ID: "   "})
}

func TestLookupUnknownReturnsFalse(t *testing.T) {
	resetRegistry(t)
	if _, ok := LookupProvider("nope"); ok {
		t.Error("LookupProvider nope = ok, want false")
	}
}

func TestRegisteredProvidersStableOrder(t *testing.T) {
	resetRegistry(t)
	for _, id := range []string{"c-platform", "a-platform", "b-platform"} {
		RegisterProvider(ProviderFactory{
			ID:                      id,
			SupportedTransportModes: []string{TransportModeStub},
			NewStub:                 func(env ProviderEnv) (Platform, error) { return nil, nil },
		})
	}
	list := RegisteredProviders()
	if len(list) != 3 {
		t.Fatalf("got %d providers, want 3", len(list))
	}
	want := []string{"a-platform", "b-platform", "c-platform"}
	for i, f := range list {
		if f.ID != want[i] {
			t.Errorf("index %d = %q, want %q", i, f.ID, want[i])
		}
	}
}

func TestRegisterNormalizesID(t *testing.T) {
	resetRegistry(t)
	RegisterProvider(ProviderFactory{
		ID:                      "Feishu-live",
		SupportedTransportModes: []string{TransportModeStub},
		NewStub:                 func(env ProviderEnv) (Platform, error) { return nil, nil },
	})
	got, ok := LookupProvider("feishu")
	if !ok {
		t.Fatalf("LookupProvider feishu after registering %q: not found", "Feishu-live")
	}
	if got.ID != "feishu" {
		t.Errorf("got.ID = %q, want %q (canonical form)", got.ID, "feishu")
	}
}
