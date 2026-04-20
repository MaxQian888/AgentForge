package nodetypes

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// fakeConditionTaskRepo is a stub ConditionTaskResolver used to verify that
// RegisterBuiltins wires deps.TaskRepo into the registered ConditionHandler.
type fakeConditionTaskRepo struct{}

func (f *fakeConditionTaskRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Task, error) {
	return nil, nil
}

// fakeBootstrapLoopRepo is a stub LoopDefResolver used to verify that
// RegisterBuiltins wires deps.DefRepo into the registered LoopHandler.
type fakeBootstrapLoopRepo struct{}

func (f *fakeBootstrapLoopRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.WorkflowDefinition, error) {
	return nil, nil
}

// builtinNames is the canonical list of names RegisterBuiltins must register.
var builtinNames = []string{
	"trigger",
	"condition",
	"agent_dispatch",
	"notification",
	"status_transition",
	"gate",
	"parallel_split",
	"parallel_join",
	"llm_agent",
	"function",
	"human_review",
	"wait_event",
	"loop",
	"sub_workflow",
	"http_call",
	"im_send",
	"qianchuan_metrics_fetcher",
	"qianchuan_strategy_runner",
	"qianchuan_action_executor",
}

func TestRegisterBuiltins_RegistersAllNineteen(t *testing.T) {
	reg := NewRegistry(nil)

	if err := RegisterBuiltins(reg, BuiltinDeps{}); err != nil {
		t.Fatalf("RegisterBuiltins returned error: %v", err)
	}

	projectID := uuid.New()
	for _, name := range builtinNames {
		entry, err := reg.Resolve(projectID, name)
		if err != nil {
			t.Errorf("Resolve(%q) returned error: %v", name, err)
			continue
		}
		if entry.Name != name {
			t.Errorf("Resolve(%q): entry.Name = %q, want %q", name, entry.Name, name)
		}
		if entry.Source != SourceBuiltin {
			t.Errorf("Resolve(%q): entry.Source = %q, want %q", name, entry.Source, SourceBuiltin)
		}
	}
}

func TestRegisterBuiltins_ReturnsErrorOnDuplicate(t *testing.T) {
	reg := NewRegistry(nil)

	if err := RegisterBuiltins(reg, BuiltinDeps{}); err != nil {
		t.Fatalf("first RegisterBuiltins returned error: %v", err)
	}

	err := RegisterBuiltins(reg, BuiltinDeps{})
	if err == nil {
		t.Fatal("second RegisterBuiltins returned nil; expected duplicate-registration error")
	}
}

func TestRegisterBuiltins_WiresConditionTaskRepo(t *testing.T) {
	reg := NewRegistry(nil)
	taskRepo := &fakeConditionTaskRepo{}

	if err := RegisterBuiltins(reg, BuiltinDeps{TaskRepo: taskRepo}); err != nil {
		t.Fatalf("RegisterBuiltins returned error: %v", err)
	}

	entry, err := reg.Resolve(uuid.New(), "condition")
	if err != nil {
		t.Fatalf("Resolve(\"condition\") failed: %v", err)
	}
	h, ok := entry.Handler.(ConditionHandler)
	if !ok {
		t.Fatalf("Handler is not ConditionHandler; got %T", entry.Handler)
	}
	if h.TaskRepo != taskRepo {
		t.Errorf("ConditionHandler.TaskRepo = %v, want %v", h.TaskRepo, taskRepo)
	}
}

func TestRegisterBuiltins_WiresLoopDefRepo(t *testing.T) {
	reg := NewRegistry(nil)
	defRepo := &fakeBootstrapLoopRepo{}

	if err := RegisterBuiltins(reg, BuiltinDeps{DefRepo: defRepo}); err != nil {
		t.Fatalf("RegisterBuiltins returned error: %v", err)
	}

	entry, err := reg.Resolve(uuid.New(), "loop")
	if err != nil {
		t.Fatalf("Resolve(\"loop\") failed: %v", err)
	}
	h, ok := entry.Handler.(LoopHandler)
	if !ok {
		t.Fatalf("Handler is not LoopHandler; got %T", entry.Handler)
	}
	if h.DefRepo != defRepo {
		t.Errorf("LoopHandler.DefRepo = %v, want %v", h.DefRepo, defRepo)
	}
}
