package strategy

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStrategyYAMLRoundTrip(t *testing.T) {
	original := Strategy{
		Name:        "test:rt",
		Description: "round-trip",
		Triggers:    StrategyTriggers{Schedule: "1m"},
		Inputs: []StrategyInput{
			{Metric: "cost", Dimensions: []string{"ad_id"}, Window: "1m"},
		},
		Rules: []StrategyRule{
			{
				Name:      "rule-a",
				Condition: "true",
				Actions: []StrategyAction{
					{
						Type:   "notify_im",
						Target: StrategyTarget{},
						Params: map[string]any{"channel": "default", "template": "hi"},
					},
				},
			},
		},
	}

	bytes, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	var decoded Strategy
	if err := yaml.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("name mismatch: got %q want %q", decoded.Name, original.Name)
	}
	if decoded.Triggers.Schedule != original.Triggers.Schedule {
		t.Errorf("schedule mismatch: got %q want %q", decoded.Triggers.Schedule, original.Triggers.Schedule)
	}
	if len(decoded.Rules) != 1 {
		t.Fatalf("rules len: got %d want 1", len(decoded.Rules))
	}
	if len(decoded.Rules[0].Actions) != 1 {
		t.Fatalf("actions len: got %d want 1", len(decoded.Rules[0].Actions))
	}
	if decoded.Rules[0].Actions[0].Type != "notify_im" {
		t.Errorf("action type: got %q want notify_im", decoded.Rules[0].Actions[0].Type)
	}
	if decoded.Rules[0].Actions[0].Params["channel"] != "default" {
		t.Errorf("action params channel: got %v want default", decoded.Rules[0].Actions[0].Params["channel"])
	}
}

func TestActionTypesAllowlistMatchesContract(t *testing.T) {
	// This list is the contract Plan 3D's runtime ships against.
	// Adding/removing types here is a CROSS-PLAN change.
	expected := []string{
		"adjust_bid",
		"adjust_budget",
		"pause_ad",
		"resume_ad",
		"apply_material",
		"notify_im",
		"record_event",
	}
	if len(ActionTypes) != len(expected) {
		t.Fatalf("ActionTypes len: got %d want %d", len(ActionTypes), len(expected))
	}
	for i, want := range expected {
		if ActionTypes[i] != want {
			t.Errorf("ActionTypes[%d]: got %q want %q", i, ActionTypes[i], want)
		}
	}
}

func TestStrategyZeroValueRoundTripsThroughYAML(t *testing.T) {
	var zero Strategy
	bytes, err := yaml.Marshal(&zero)
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	var back Strategy
	if err := yaml.Unmarshal(bytes, &back); err != nil {
		t.Fatalf("unmarshal zero: %v", err)
	}
	if back.Name != "" {
		t.Errorf("zero round-trip name: got %q want empty", back.Name)
	}
}
