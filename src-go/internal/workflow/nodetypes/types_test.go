package nodetypes

import (
	"encoding/json"
	"testing"
)

func TestEffectKind_IsPark(t *testing.T) {
	cases := map[EffectKind]bool{
		EffectSpawnAgent:        true,
		EffectRequestReview:     true,
		EffectWaitEvent:         true,
		EffectInvokeSubWorkflow: true,
		EffectBroadcastEvent:    false,
		EffectUpdateTaskStatus:  false,
		EffectResetNodes:        false,
	}
	for k, want := range cases {
		if got := k.IsPark(); got != want {
			t.Errorf("EffectKind(%q).IsPark() = %v, want %v", k, got, want)
		}
	}
}

func TestNodeExecResult_ParkCount(t *testing.T) {
	r := &NodeExecResult{
		Effects: []Effect{
			{Kind: EffectBroadcastEvent, Payload: json.RawMessage(`{}`)},
			{Kind: EffectSpawnAgent, Payload: json.RawMessage(`{}`)},
		},
	}
	if got := r.ParkCount(); got != 1 {
		t.Errorf("ParkCount() = %d, want 1", got)
	}
}
