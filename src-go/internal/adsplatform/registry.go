package adsplatform

import (
	"fmt"
	"sync"
)

// Constructor builds a Provider on demand. Idempotent: registry caches
// nothing; callers may invoke Resolve repeatedly.
type Constructor func() Provider

// Registry maps provider name → constructor.
type Registry struct {
	mu    sync.RWMutex
	ctors map[string]Constructor
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{ctors: map[string]Constructor{}} }

// Register installs ctor under name. Panics on duplicate registration —
// duplicate names are a programmer error, not a runtime condition.
func (r *Registry) Register(name string, ctor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.ctors[name]; exists {
		panic(fmt.Sprintf("adsplatform: duplicate registration for %q", name))
	}
	r.ctors[name] = ctor
}

// Resolve constructs the provider for name, or returns ErrProviderNotFound.
func (r *Registry) Resolve(name string) (Provider, error) {
	r.mu.RLock()
	ctor, ok := r.ctors[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrProviderNotFound, name)
	}
	return ctor(), nil
}
