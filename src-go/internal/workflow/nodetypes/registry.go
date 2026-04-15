package nodetypes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// ErrNodeTypeNotFound is returned when Resolve cannot find a matching node type.
var ErrNodeTypeNotFound = errors.New("node type not found")

// EntrySource distinguishes built-in from plugin-contributed node types.
type EntrySource string

const (
	SourceBuiltin EntrySource = "builtin"
	SourcePlugin  EntrySource = "plugin"
)

// NodeTypeEntry is a single registered node type with its metadata.
type NodeTypeEntry struct {
	Name          string
	Handler       NodeTypeHandler
	Source        EntrySource
	PluginID      string              // empty for builtins
	PluginVersion string              // empty for builtins
	DeclaredCaps  map[EffectKind]bool // cached from Capabilities()
}

// PluginEventSink is the audit trail interface. May be nil in tests.
type PluginEventSink interface {
	RecordEvent(ctx context.Context, eventType string, payload map[string]any) error
}

// NodeTypeRegistry is a two-layer (global built-in + per-project plugin) registry
// for workflow node types.
type NodeTypeRegistry struct {
	mu           sync.RWMutex
	global       map[string]NodeTypeEntry               // name -> entry (built-in)
	project      map[uuid.UUID]map[string]NodeTypeEntry  // projectID -> {name -> entry}
	reserved     map[string]bool                         // set of reserved built-in names
	events       PluginEventSink                         // for audit writes (may be nil)
	lockedGlobal bool                                    // after bootstrap: no more built-in registration
}

// NewRegistry creates an empty registry with an optional event sink.
func NewRegistry(events PluginEventSink) *NodeTypeRegistry {
	return &NodeTypeRegistry{
		global:   make(map[string]NodeTypeEntry),
		project:  make(map[uuid.UUID]map[string]NodeTypeEntry),
		reserved: make(map[string]bool),
		events:   events,
	}
}

// RegisterBuiltin registers a built-in node type in the global scope.
// Returns an error if the global scope is locked or the name already exists.
func (r *NodeTypeRegistry) RegisterBuiltin(name string, h NodeTypeHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lockedGlobal {
		return fmt.Errorf("global registry is locked; cannot register builtin %q", name)
	}
	if _, exists := r.global[name]; exists {
		return fmt.Errorf("builtin %q already registered", name)
	}

	entry := NodeTypeEntry{
		Name:         name,
		Handler:      h,
		Source:       SourceBuiltin,
		DeclaredCaps: capsMap(h.Capabilities()),
	}
	r.global[name] = entry
	r.reserved[name] = true

	r.emitLocked("registry_entry_added", map[string]any{
		"name":   name,
		"source": string(SourceBuiltin),
	})
	return nil
}

// LockGlobal prevents further built-in registrations. Typically called after bootstrap.
func (r *NodeTypeRegistry) LockGlobal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lockedGlobal = true
}

// RegisterPluginNode registers a plugin-contributed node type in a project scope.
func (r *NodeTypeRegistry) RegisterPluginNode(projectID uuid.UUID, pluginID, pluginVersion, name string, h NodeTypeHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate: name must contain exactly one "/".
	parts := strings.SplitN(name, "/", 3)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		r.emitLocked("registry_rejected", map[string]any{
			"name":   name,
			"reason": "name must contain exactly one '/' separator",
		})
		return fmt.Errorf("plugin node name %q must contain exactly one '/' separator", name)
	}

	// Validate: prefix before "/" must equal pluginID.
	if parts[0] != pluginID {
		r.emitLocked("registry_rejected", map[string]any{
			"name":   name,
			"reason": "prefix must equal pluginID",
		})
		return fmt.Errorf("plugin node name prefix %q does not match pluginID %q", parts[0], pluginID)
	}

	// Validate: full name and suffix must not collide with reserved names.
	if r.reserved[name] {
		r.emitLocked("registry_rejected", map[string]any{
			"name":   name,
			"reason": "name is reserved",
		})
		return fmt.Errorf("plugin node name %q collides with a reserved name", name)
	}
	if r.reserved[parts[1]] {
		r.emitLocked("registry_rejected", map[string]any{
			"name":   name,
			"reason": "suffix collides with reserved name",
		})
		return fmt.Errorf("plugin node name suffix %q collides with reserved name %q", parts[1], parts[1])
	}

	// Validate: not already present in the same project scope.
	projMap := r.project[projectID]
	if projMap != nil {
		if _, exists := projMap[name]; exists {
			r.emitLocked("registry_rejected", map[string]any{
				"name":   name,
				"reason": "duplicate in project scope",
			})
			return fmt.Errorf("plugin node %q already registered in project %s", name, projectID)
		}
	}

	// Register.
	if projMap == nil {
		projMap = make(map[string]NodeTypeEntry)
		r.project[projectID] = projMap
	}
	projMap[name] = NodeTypeEntry{
		Name:          name,
		Handler:       h,
		Source:        SourcePlugin,
		PluginID:      pluginID,
		PluginVersion: pluginVersion,
		DeclaredCaps:  capsMap(h.Capabilities()),
	}

	r.emitLocked("registry_entry_added", map[string]any{
		"name":      name,
		"source":    string(SourcePlugin),
		"pluginID":  pluginID,
		"projectID": projectID.String(),
	})
	return nil
}

// UnregisterPlugin removes all entries for the given pluginID from the given project.
// Returns the count of entries removed.
func (r *NodeTypeRegistry) UnregisterPlugin(projectID uuid.UUID, pluginID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	projMap := r.project[projectID]
	if projMap == nil {
		return 0
	}

	var toDelete []string
	for name, entry := range projMap {
		if entry.PluginID == pluginID {
			toDelete = append(toDelete, name)
		}
	}

	for _, name := range toDelete {
		delete(projMap, name)
		r.emitLocked("registry_entry_removed", map[string]any{
			"name":      name,
			"pluginID":  pluginID,
			"projectID": projectID.String(),
		})
	}

	if len(projMap) == 0 {
		delete(r.project, projectID)
	}

	return len(toDelete)
}

// Resolve looks up a node type by name, checking the project scope first, then global.
func (r *NodeTypeRegistry) Resolve(projectID uuid.UUID, name string) (NodeTypeEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check project scope first.
	if projMap := r.project[projectID]; projMap != nil {
		if entry, ok := projMap[name]; ok {
			return entry, nil
		}
	}

	// Fall back to global.
	if entry, ok := r.global[name]; ok {
		return entry, nil
	}

	return NodeTypeEntry{}, ErrNodeTypeNotFound
}

// ListForProject returns a merged view of project-scoped entries and all built-ins.
func (r *NodeTypeRegistry) ListForProject(projectID uuid.UUID) []NodeTypeEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]NodeTypeEntry, 0, len(r.global))

	// Add all built-ins.
	for _, entry := range r.global {
		result = append(result, entry)
	}

	// Add project entries.
	if projMap := r.project[projectID]; projMap != nil {
		for _, entry := range projMap {
			result = append(result, entry)
		}
	}

	return result
}

// capsMap converts a slice of EffectKind into a lookup map.
func capsMap(kinds []EffectKind) map[EffectKind]bool {
	m := make(map[EffectKind]bool, len(kinds))
	for _, k := range kinds {
		m[k] = true
	}
	return m
}

// emitLocked emits an event if the sink is non-nil. Must be called with mu held.
func (r *NodeTypeRegistry) emitLocked(eventType string, payload map[string]any) {
	if r.events != nil {
		// Use background context since this is an internal audit write.
		_ = r.events.RecordEvent(context.Background(), eventType, payload)
	}
}
