package core_test

import (
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func resetRegistry(t *testing.T) {
	t.Helper()
	// TestOnly_ResetProviderRegistry is only exposed to _test.go via a
	// companion file; provided to let tests run in any order.
	core.TestOnly_ResetProviderRegistry()
}

func TestRegisterAndLookupProvider(t *testing.T) {
	resetRegistry(t)

	f := core.ProviderFactory{
		ID:                      "unittest-platform",
		SupportedTransportModes: []string{core.TransportModeStub},
		EnvPrefixes:             []string{"UNITTEST_"},
		NewStub: func(env core.ProviderEnv) (core.Platform, error) {
			return nil, nil
		},
	}
	core.RegisterProvider(f)

	got, ok := core.LookupProvider("unittest-platform")
	if !ok {
		t.Fatalf("LookupProvider unittest-platform: not found")
	}
	if got.ID != "unittest-platform" {
		t.Errorf("ID = %q, want unittest-platform", got.ID)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	resetRegistry(t)
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "dup",
		SupportedTransportModes: []string{core.TransportModeStub},
		NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "dup",
		SupportedTransportModes: []string{core.TransportModeStub},
		NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
	})
}

func TestRegisterEmptyIDPanics(t *testing.T) {
	resetRegistry(t)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty id")
		}
	}()
	core.RegisterProvider(core.ProviderFactory{ID: "   "})
}

func TestLookupUnknownReturnsFalse(t *testing.T) {
	resetRegistry(t)
	if _, ok := core.LookupProvider("nope"); ok {
		t.Error("LookupProvider nope = ok, want false")
	}
}

func TestRegisteredProvidersStableOrder(t *testing.T) {
	resetRegistry(t)
	for _, id := range []string{"c-platform", "a-platform", "b-platform"} {
		core.RegisterProvider(core.ProviderFactory{
			ID:                      id,
			SupportedTransportModes: []string{core.TransportModeStub},
			NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
		})
	}
	list := core.RegisteredProviders()
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
	core.RegisterProvider(core.ProviderFactory{
		ID:                      "Feishu-live",
		SupportedTransportModes: []string{core.TransportModeStub},
		NewStub:                 func(env core.ProviderEnv) (core.Platform, error) { return nil, nil },
	})
	got, ok := core.LookupProvider("feishu")
	if !ok {
		t.Fatalf("LookupProvider feishu after registering %q: not found", "Feishu-live")
	}
	if got.ID != "feishu" {
		t.Errorf("got.ID = %q, want %q (canonical form)", got.ID, "feishu")
	}
}
