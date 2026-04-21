package core

// TestOnly_ResetProviderRegistry clears the global provider registry.
// Intended for tests that register fixtures and need a clean slate. The
// `TestOnly_` prefix ensures calls are grep-able; using this in production
// code is a bug.
func TestOnly_ResetProviderRegistry() {
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	providerRegistry = map[string]ProviderFactory{}
}
