package nodetypes

import (
	"context"
	"testing"
)

// assertCapsCoverEffects fails the test if any effect in `effects` has a Kind
// that is not declared in the handler's Capabilities().
func assertCapsCoverEffects(t *testing.T, h NodeTypeHandler, effects []Effect) {
	t.Helper()
	caps := make(map[EffectKind]bool)
	for _, k := range h.Capabilities() {
		caps[k] = true
	}
	for _, e := range effects {
		if !caps[e.Kind] {
			t.Fatalf("handler emitted undeclared capability: %s", e.Kind)
		}
	}
}

func TestStructuralHandlers_NoOp(t *testing.T) {
	cases := []struct {
		name    string
		handler NodeTypeHandler
	}{
		{"trigger", TriggerHandler{}},
		{"gate", GateHandler{}},
		{"parallel_split", ParallelSplitHandler{}},
		{"parallel_join", ParallelJoinHandler{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			req := &NodeExecRequest{}

			result, err := tc.handler.Execute(ctx, req)
			if err != nil {
				t.Fatalf("Execute() returned error: %v", err)
			}
			if result == nil {
				t.Fatal("Execute() returned nil result")
			}
			if result.Result != nil {
				t.Errorf("Execute() result.Result = %v, want nil", result.Result)
			}
			if len(result.Effects) != 0 {
				t.Errorf("Execute() produced %d effects, want 0", len(result.Effects))
			}

			if schema := tc.handler.ConfigSchema(); schema != nil {
				t.Errorf("ConfigSchema() = %s, want nil", schema)
			}

			if caps := tc.handler.Capabilities(); len(caps) != 0 {
				t.Errorf("Capabilities() = %v, want nil/empty", caps)
			}

			assertCapsCoverEffects(t, tc.handler, result.Effects)
		})
	}
}
