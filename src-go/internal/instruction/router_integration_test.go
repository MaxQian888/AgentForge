package instruction_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/agentforge/server/internal/instruction"
)

func TestInstructionRouter_ProcessNextRoutesAcrossTargets(t *testing.T) {
	t.Parallel()

	router := instruction.NewRouter()
	targetCalls := make([]string, 0, 3)

	register := func(instructionType string, target instruction.Target) {
		t.Helper()
		if err := router.Register(instructionType, instruction.Definition{
			Target: target,
			Handler: instruction.HandlerFunc(func(ctx context.Context, req instruction.Request) (map[string]any, error) {
				targetCalls = append(targetCalls, fmt.Sprintf("%s:%s", target, req.ID))
				return map[string]any{
					"id":     req.ID,
					"target": string(target),
				}, nil
			}),
		}); err != nil {
			t.Fatalf("Register(%s) error = %v", instructionType, err)
		}
	}

	register("read", instruction.TargetLocal)
	register("think", instruction.TargetBridge)
	register("plugin.search", instruction.TargetPlugin)

	for _, req := range []instruction.Request{
		{ID: "bridge", Type: "think", Priority: 50},
		{ID: "plugin", Type: "plugin.search", Priority: 20, Dependencies: []string{"bridge"}},
		{ID: "local", Type: "read", Priority: 10},
	} {
		if err := router.Enqueue(req); err != nil {
			t.Fatalf("Enqueue(%s) error = %v", req.ID, err)
		}
	}

	first, err := router.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext(bridge) error = %v", err)
	}
	if first.ID != "bridge" || first.Target != instruction.TargetBridge {
		t.Fatalf("first result = %#v, want bridge target", first)
	}

	second, err := router.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext(plugin) error = %v", err)
	}
	if second.ID != "plugin" || second.Target != instruction.TargetPlugin {
		t.Fatalf("second result = %#v, want plugin target", second)
	}

	third, err := router.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("ProcessNext(local) error = %v", err)
	}
	if third.ID != "local" || third.Target != instruction.TargetLocal {
		t.Fatalf("third result = %#v, want local target", third)
	}

	if got, want := targetCalls, []string{"bridge:bridge", "plugin:plugin", "local:local"}; len(got) != len(want) {
		t.Fatalf("targetCalls len = %d, want %d (%v)", len(got), len(want), got)
	} else {
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("targetCalls[%d] = %q, want %q", index, got[index], want[index])
			}
		}
	}

	history := router.History(10)
	if len(history) != 3 {
		t.Fatalf("History() len = %d, want 3", len(history))
	}
}
