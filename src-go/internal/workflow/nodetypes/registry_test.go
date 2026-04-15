package nodetypes

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// fakeHandler implements NodeTypeHandler with configurable capabilities.
type fakeHandler struct {
	caps []EffectKind
}

func (f *fakeHandler) Execute(_ context.Context, _ *NodeExecRequest) (*NodeExecResult, error) {
	return &NodeExecResult{}, nil
}

func (f *fakeHandler) ConfigSchema() json.RawMessage { return nil }

func (f *fakeHandler) Capabilities() []EffectKind { return f.caps }

// fakeEventSink records audit events for assertions.
type fakeEventSink struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	eventType string
	payload   map[string]any
}

func (s *fakeEventSink) RecordEvent(_ context.Context, eventType string, payload map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, recordedEvent{eventType: eventType, payload: payload})
	return nil
}

func (s *fakeEventSink) eventTypes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.events))
	for i, e := range s.events {
		out[i] = e.eventType
	}
	return out
}

func newFakeHandler(caps ...EffectKind) *fakeHandler {
	return &fakeHandler{caps: caps}
}

func TestRegistry_RegisterBuiltin_ThenResolve(t *testing.T) {
	reg := NewRegistry(nil)
	h := newFakeHandler(EffectSpawnAgent)

	if err := reg.RegisterBuiltin("llm_agent", h); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Resolve from a random project should find the built-in.
	entry, err := reg.Resolve(uuid.New(), "llm_agent")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if entry.Name != "llm_agent" {
		t.Errorf("expected name llm_agent, got %s", entry.Name)
	}
	if entry.Source != SourceBuiltin {
		t.Errorf("expected source builtin, got %s", entry.Source)
	}
	if !entry.DeclaredCaps[EffectSpawnAgent] {
		t.Error("expected spawn_agent capability")
	}
}

func TestRegistry_LockGlobal_PreventsFurtherBuiltinRegistration(t *testing.T) {
	reg := NewRegistry(nil)

	if err := reg.RegisterBuiltin("llm_agent", newFakeHandler()); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	reg.LockGlobal()

	err := reg.RegisterBuiltin("another", newFakeHandler())
	if err == nil {
		t.Fatal("expected error after LockGlobal, got nil")
	}
}

func TestRegistry_RegisterPluginNode_RejectsReservedName(t *testing.T) {
	reg := NewRegistry(nil)
	projID := uuid.New()

	// Register a built-in to reserve the name "llm_agent".
	if err := reg.RegisterBuiltin("llm_agent", newFakeHandler()); err != nil {
		t.Fatalf("RegisterBuiltin failed: %v", err)
	}

	// Plugin tries to use the reserved suffix "llm_agent".
	err := reg.RegisterPluginNode(projID, "acme", "1.0.0", "acme/llm_agent", newFakeHandler())
	if err == nil {
		t.Fatal("expected error for reserved name, got nil")
	}
}

func TestRegistry_RegisterPluginNode_RequiresPluginIDPrefix(t *testing.T) {
	reg := NewRegistry(nil)
	projID := uuid.New()

	// No slash → fail.
	err := reg.RegisterPluginNode(projID, "acme", "1.0.0", "noslash", newFakeHandler())
	if err == nil {
		t.Fatal("expected error for missing slash")
	}

	// Wrong prefix → fail.
	err = reg.RegisterPluginNode(projID, "acme", "1.0.0", "wrong/mynode", newFakeHandler())
	if err == nil {
		t.Fatal("expected error for wrong prefix")
	}

	// Correct prefix → succeed.
	err = reg.RegisterPluginNode(projID, "acme", "1.0.0", "acme/mynode", newFakeHandler())
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestRegistry_ResolveOrder_ProjectBeforeGlobal(t *testing.T) {
	sink := &fakeEventSink{}
	reg := NewRegistry(sink)
	projA := uuid.New()
	projB := uuid.New()

	// Register a built-in.
	if err := reg.RegisterBuiltin("llm_agent", newFakeHandler()); err != nil {
		t.Fatal(err)
	}
	reg.LockGlobal()

	// Register a plugin node for projA.
	if err := reg.RegisterPluginNode(projA, "acme", "1.0.0", "acme/custom", newFakeHandler(EffectRequestReview)); err != nil {
		t.Fatal(err)
	}

	// Built-in resolves for any project.
	if _, err := reg.Resolve(projA, "llm_agent"); err != nil {
		t.Errorf("expected builtin resolve for projA: %v", err)
	}
	if _, err := reg.Resolve(projB, "llm_agent"); err != nil {
		t.Errorf("expected builtin resolve for projB: %v", err)
	}

	// Plugin name resolves only for its project.
	entry, err := reg.Resolve(projA, "acme/custom")
	if err != nil {
		t.Fatalf("expected plugin resolve for projA: %v", err)
	}
	if entry.Source != SourcePlugin {
		t.Errorf("expected plugin source, got %s", entry.Source)
	}

	// Plugin name does NOT resolve for another project.
	_, err = reg.Resolve(projB, "acme/custom")
	if err == nil {
		t.Fatal("expected ErrNodeTypeNotFound for projB")
	}
	if err != ErrNodeTypeNotFound {
		t.Errorf("expected ErrNodeTypeNotFound, got %v", err)
	}
}

func TestRegistry_UnregisterPlugin_RemovesAllEntries(t *testing.T) {
	sink := &fakeEventSink{}
	reg := NewRegistry(sink)
	projID := uuid.New()

	if err := reg.RegisterPluginNode(projID, "acme", "1.0.0", "acme/node1", newFakeHandler()); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterPluginNode(projID, "acme", "1.0.0", "acme/node2", newFakeHandler()); err != nil {
		t.Fatal(err)
	}

	count := reg.UnregisterPlugin(projID, "acme")
	if count != 2 {
		t.Errorf("expected 2 removed, got %d", count)
	}

	// Both should be gone.
	if _, err := reg.Resolve(projID, "acme/node1"); err != ErrNodeTypeNotFound {
		t.Error("expected node1 gone")
	}
	if _, err := reg.Resolve(projID, "acme/node2"); err != ErrNodeTypeNotFound {
		t.Error("expected node2 gone")
	}

	// Check audit events.
	types := sink.eventTypes()
	removedCount := 0
	for _, et := range types {
		if et == "registry_entry_removed" {
			removedCount++
		}
	}
	if removedCount != 2 {
		t.Errorf("expected 2 removal events, got %d", removedCount)
	}
}

func TestRegistry_ListForProject_MergesBuiltinAndPlugin(t *testing.T) {
	reg := NewRegistry(nil)
	projID := uuid.New()

	if err := reg.RegisterBuiltin("llm_agent", newFakeHandler()); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterBuiltin("http_call", newFakeHandler()); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterPluginNode(projID, "acme", "1.0.0", "acme/custom", newFakeHandler()); err != nil {
		t.Fatal(err)
	}

	entries := reg.ListForProject(projID)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	for _, expected := range []string{"llm_agent", "http_call", "acme/custom"} {
		if !names[expected] {
			t.Errorf("missing expected entry %s", expected)
		}
	}
}

func TestRegistry_RegisterPluginNode_RejectsDuplicateInSameProject(t *testing.T) {
	reg := NewRegistry(nil)
	projID := uuid.New()

	if err := reg.RegisterPluginNode(projID, "acme", "1.0.0", "acme/mynode", newFakeHandler()); err != nil {
		t.Fatal(err)
	}

	err := reg.RegisterPluginNode(projID, "acme", "2.0.0", "acme/mynode", newFakeHandler())
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}
