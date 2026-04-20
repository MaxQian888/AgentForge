package vcs

import (
	"fmt"
	"sync"
)

// Constructor builds a Provider for a given (host, token). token is
// the resolved-at-call-time PAT plaintext from the 1B secrets store
// — it MUST NOT be cached on the returned Provider value beyond the
// single outbound request being constructed.
type Constructor func(host, token string) (Provider, error)

// Registry maps provider names ("github", "gitlab", ...) to their
// constructors. Wired once at server bootstrap. Tests build a fresh
// Registry per case to avoid global mutation.
type Registry struct {
	mu    sync.RWMutex
	ctors map[string]Constructor
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{ctors: map[string]Constructor{}}
}

// Register installs ctor under name. Panics on duplicate to surface
// wiring bugs at boot rather than silently shadowing.
func (r *Registry) Register(name string, ctor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.ctors[name]; exists {
		panic(fmt.Sprintf("vcs: provider %q already registered", name))
	}
	r.ctors[name] = ctor
}

// Resolve constructs a Provider for the given configuration. Returns
// ErrProviderUnsupported if no constructor is registered for name.
func (r *Registry) Resolve(name, host, token string) (Provider, error) {
	r.mu.RLock()
	ctor, ok := r.ctors[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderUnsupported, name)
	}
	return ctor(host, token)
}

// Names returns registered provider names. Order is not guaranteed;
// callers that need deterministic ordering should sort.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.ctors))
	for n := range r.ctors {
		out = append(out, n)
	}
	return out
}
