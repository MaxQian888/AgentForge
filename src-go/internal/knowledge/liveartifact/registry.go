package liveartifact

import (
	"fmt"
	"sync"
)

// Registry is the set of LiveArtifactProjectors registered for this
// process. The server bootstrap populates it once at startup; the
// projection endpoint and the subscription router look projectors up via
// Lookup.
type Registry struct {
	mu         sync.RWMutex
	projectors map[LiveArtifactKind]LiveArtifactProjector
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{projectors: make(map[LiveArtifactKind]LiveArtifactProjector)}
}

// Register adds p to the registry. Registering the same kind twice
// panics — projectors are declared once at startup and duplicate
// registration is always a programmer error.
func (r *Registry) Register(p LiveArtifactProjector) {
	if p == nil {
		panic("liveartifact: nil projector")
	}
	kind := p.Kind()
	if kind == "" {
		panic("liveartifact: projector returned empty kind")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.projectors[kind]; exists {
		panic(fmt.Sprintf("liveartifact: duplicate projector for kind %q", kind))
	}
	r.projectors[kind] = p
}

// Lookup returns the projector for kind if one is registered.
func (r *Registry) Lookup(kind LiveArtifactKind) (LiveArtifactProjector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.projectors[kind]
	return p, ok
}

// Kinds returns the sorted-by-insertion-order set of registered kinds.
// Used by tests and diagnostics.
func (r *Registry) Kinds() []LiveArtifactKind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LiveArtifactKind, 0, len(r.projectors))
	for k := range r.projectors {
		out = append(out, k)
	}
	return out
}
