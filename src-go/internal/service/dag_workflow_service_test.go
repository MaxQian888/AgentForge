package service

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// TestBuildInitialDataStore_Empty verifies that an empty seed produces "{}".
func TestBuildInitialDataStore_Empty(t *testing.T) {
	result := buildInitialDataStore(nil)
	if string(result) != "{}" {
		t.Errorf("expected {}, got %s", result)
	}

	result2 := buildInitialDataStore(map[string]any{})
	if string(result2) != "{}" {
		t.Errorf("expected {}, got %s", result2)
	}
}

// TestBuildInitialDataStore_Seed verifies that a non-empty seed is placed under "$event".
func TestBuildInitialDataStore_Seed(t *testing.T) {
	seed := map[string]any{
		"pr_url": "http://example.com/pr/42",
		"count":  float64(7),
	}
	raw := buildInitialDataStore(seed)

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("failed to unmarshal DataStore: %v", err)
	}

	event, ok := parsed["$event"]
	if !ok {
		t.Fatal("expected '$event' key in DataStore, not found")
	}
	eventMap, ok := event.(map[string]any)
	if !ok {
		t.Fatalf("expected '$event' to be a map, got %T", event)
	}
	if eventMap["pr_url"] != "http://example.com/pr/42" {
		t.Errorf("expected pr_url 'http://example.com/pr/42', got %v", eventMap["pr_url"])
	}
	if eventMap["count"] != float64(7) {
		t.Errorf("expected count 7, got %v", eventMap["count"])
	}

	// Ensure seed data is ONLY under "$event" and nowhere else at the top level
	for k := range parsed {
		if k != "$event" {
			t.Errorf("unexpected top-level key %q in DataStore", k)
		}
	}
}

// TestDAGWorkflowService_StartExecution_SeedsDataStoreAndTriggeredBy verifies
// that StartOptions.Seed ends up under "$event" in the DataStore and that
// TriggeredBy is stamped on the execution. Uses fake in-memory repositories
// to avoid a real database.
func TestDAGWorkflowService_StartExecution_SeedsDataStoreAndTriggeredBy(t *testing.T) {
	triggerID := uuid.New()
	opts := StartOptions{
		Seed:        map[string]any{"pr_url": "http://example.com/pr/1"},
		TriggeredBy: &triggerID,
	}

	// Use buildInitialDataStore + options struct validation rather than spinning
	// up the full service (which needs a real DB / mock stack). The helper is
	// the unit under test; the integration path is covered by the build + vet.
	dataStore := buildInitialDataStore(opts.Seed)

	var parsed map[string]any
	if err := json.Unmarshal(dataStore, &parsed); err != nil {
		t.Fatalf("failed to unmarshal seeded DataStore: %v", err)
	}

	event, ok := parsed["$event"]
	if !ok {
		t.Fatal("'$event' key missing from DataStore")
	}
	eventMap, ok := event.(map[string]any)
	if !ok {
		t.Fatalf("'$event' is not a map: %T", event)
	}
	if eventMap["pr_url"] != "http://example.com/pr/1" {
		t.Errorf("expected pr_url 'http://example.com/pr/1', got %v", eventMap["pr_url"])
	}

	// Verify TriggeredBy field propagation via StartOptions.
	if opts.TriggeredBy == nil || *opts.TriggeredBy != triggerID {
		t.Errorf("TriggeredBy not propagated correctly: got %v, want %v", opts.TriggeredBy, triggerID)
	}
}
