package memory_test

import (
	"testing"

	"github.com/react-go-quick-starter/server/internal/memory"
)

func tokenEstimator(text string) int {
	switch text {
	case "":
		return 0
	default:
		count := 1
		for _, ch := range text {
			if ch == ' ' {
				count++
			}
		}
		return count
	}
}

func TestShortTermMemory_StoresWithinBudgetAndRetrievesRecentContext(t *testing.T) {
	t.Parallel()

	store := memory.NewShortTermMemory(memory.Config{
		MaxTokens:            6,
		DefaultContextTokens: 4,
		TokenEstimator:       tokenEstimator,
		EvictionPolicy:       memory.EvictionPolicyLRU,
	})

	for _, input := range []memory.StoreInput{
		{Scope: "session-1", ID: "one", Content: "alpha beta"},
		{Scope: "session-1", ID: "two", Content: "gamma delta"},
		{Scope: "session-1", ID: "three", Content: "epsilon zeta"},
	} {
		if _, err := store.Store(input); err != nil {
			t.Fatalf("Store(%s) error = %v", input.ID, err)
		}
	}

	snapshot, ok := store.Snapshot("session-1")
	if !ok || snapshot.TotalTokens != 6 || len(snapshot.Entries) != 3 {
		t.Fatalf("Snapshot() = %#v, ok=%v, want 3 entries and 6 tokens", snapshot, ok)
	}

	contextEntries, err := store.Context("session-1", 0)
	if err != nil {
		t.Fatalf("Context() error = %v", err)
	}
	if len(contextEntries) != 2 || contextEntries[0].ID != "two" || contextEntries[1].ID != "three" {
		t.Fatalf("Context() = %#v, want [two three]", contextEntries)
	}
}

func TestShortTermMemory_LRUEvictionHonorsBudget(t *testing.T) {
	t.Parallel()

	store := memory.NewShortTermMemory(memory.Config{
		MaxTokens:      5,
		TokenEstimator: tokenEstimator,
		EvictionPolicy: memory.EvictionPolicyLRU,
	})

	for _, input := range []memory.StoreInput{
		{Scope: "session-1", ID: "one", Content: "alpha beta"},
		{Scope: "session-1", ID: "two", Content: "gamma delta"},
		{Scope: "session-1", ID: "three", Content: "epsilon zeta eta"},
	} {
		if _, err := store.Store(input); err != nil {
			t.Fatalf("Store(%s) error = %v", input.ID, err)
		}
	}

	snapshot, ok := store.Snapshot("session-1")
	if !ok {
		t.Fatal("Snapshot() ok = false, want true")
	}
	if snapshot.TotalTokens != 5 {
		t.Fatalf("snapshot.TotalTokens = %d, want 5", snapshot.TotalTokens)
	}
	if len(snapshot.Entries) != 2 || snapshot.Entries[0].ID != "two" || snapshot.Entries[1].ID != "three" {
		t.Fatalf("snapshot.Entries = %#v, want [two three]", snapshot.Entries)
	}
}

func TestShortTermMemory_ImportanceEvictionPrefersHigherPriorityContent(t *testing.T) {
	t.Parallel()

	store := memory.NewShortTermMemory(memory.Config{
		MaxTokens:      5,
		TokenEstimator: tokenEstimator,
		EvictionPolicy: memory.EvictionPolicyImportance,
	})

	for _, input := range []memory.StoreInput{
		{Scope: "session-1", ID: "high", Content: "alpha beta", Importance: 1},
		{Scope: "session-1", ID: "low", Content: "gamma delta", Importance: 0.1},
		{Scope: "session-1", ID: "new", Content: "epsilon zeta eta", Importance: 0.7},
	} {
		if _, err := store.Store(input); err != nil {
			t.Fatalf("Store(%s) error = %v", input.ID, err)
		}
	}

	snapshot, ok := store.Snapshot("session-1")
	if !ok {
		t.Fatal("Snapshot() ok = false, want true")
	}
	if len(snapshot.Entries) != 2 {
		t.Fatalf("len(snapshot.Entries) = %d, want 2", len(snapshot.Entries))
	}
	if snapshot.Entries[0].ID != "high" || snapshot.Entries[1].ID != "new" {
		t.Fatalf("snapshot.Entries = %#v, want [high new]", snapshot.Entries)
	}
}
