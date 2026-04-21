package core

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Transport mode string constants used by provider factories and the
// cmd/bridge selection code. These live in core so provider packages do
// not need to import cmd/bridge.
const (
	TransportModeStub = "stub"
	TransportModeLive = "live"
)

// ProviderFactory is the self-registering descriptor each IM provider
// package publishes via init(). Each provider id must be registered
// exactly once — duplicate registration panics.
type ProviderFactory struct {
	// ID is the normalized provider identifier (e.g. "feishu", "slack").
	ID string
	// Metadata mirrors what the provider's stub/live adapters report via
	// Platform.Metadata(). Used by inventory reporting.
	Metadata PlatformMetadata
	// SupportedTransportModes enumerates the transport modes this factory
	// can build, e.g. ["stub", "live"].
	SupportedTransportModes []string
	// Features is an opaque, provider-specific capability record. Consumers
	// that care (e.g. Feishu card rendering helpers) type-assert; others
	// ignore the field.
	Features any
	// EnvPrefixes declares the uppercase env-var namespaces this factory is
	// allowed to read through ProviderEnv. Cross-namespace reads return ""
	// to prevent silent coupling between providers.
	EnvPrefixes []string

	ValidateConfig func(env ProviderEnv, mode string) error
	NewStub        func(env ProviderEnv) (Platform, error)
	NewLive        func(env ProviderEnv) (Platform, error)
}

// ProviderEnv is a read-only, namespace-gated view into the Bridge process
// environment. cmd/bridge constructs one per factory invocation, backed by
// the loaded config struct.
type ProviderEnv interface {
	// Get returns the string value for key. The key must begin with one of
	// the factory's declared EnvPrefixes; violations return "".
	Get(key string) string
	BoolOr(key string, fallback bool) bool
	DurationOr(key string, fallback time.Duration) time.Duration
	// TestPort is the shared TEST_PORT value every stub adapter needs.
	// It is intentionally not namespace-gated.
	TestPort() string
}

var (
	providerRegistryMu sync.RWMutex
	providerRegistry   = map[string]ProviderFactory{}
)

// RegisterProvider records a factory. Panics on empty or duplicate ID so
// misconfiguration surfaces at process startup.
func RegisterProvider(f ProviderFactory) {
	id := strings.TrimSpace(f.ID)
	if id == "" {
		panic("core.RegisterProvider: empty provider id")
	}
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	if _, dup := providerRegistry[id]; dup {
		panic(fmt.Sprintf("core.RegisterProvider: duplicate provider id %q", id))
	}
	providerRegistry[id] = f
}

// LookupProvider returns the factory registered for id, if any. The lookup
// normalizes id the same way NormalizePlatformName does so callers can
// pass raw env values.
func LookupProvider(id string) (ProviderFactory, bool) {
	providerRegistryMu.RLock()
	defer providerRegistryMu.RUnlock()
	f, ok := providerRegistry[NormalizePlatformName(id)]
	return f, ok
}

// RegisteredProviders returns a deterministic snapshot sorted by ID.
func RegisteredProviders() []ProviderFactory {
	providerRegistryMu.RLock()
	defer providerRegistryMu.RUnlock()
	out := make([]ProviderFactory, 0, len(providerRegistry))
	for _, f := range providerRegistry {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
