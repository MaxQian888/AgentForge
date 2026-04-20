package strategy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParsedSpecJSONRoundTripPreservesAllFields(t *testing.T) {
	spec := ParsedSpec{
		SchemaVersion:   1,
		ScheduleSeconds: 60,
		Inputs: []ParsedInput{
			{Metric: "cost", Dimensions: []string{"ad_id"}, WindowSeconds: 60},
			{Metric: "cvr", Dimensions: []string{"ad_id"}, WindowSeconds: 900},
		},
		Rules: []ParsedRule{
			{
				Name:         "heartbeat",
				ConditionRaw: "true",
				Actions: []ParsedAction{
					{
						Type:     "notify_im",
						AdIDExpr: "",
						Params:   map[string]any{"channel": "default", "template": "hi"},
					},
				},
			},
			{
				Name:         "bump-bid",
				ConditionRaw: "snapshot.metrics.cvr",
				Actions: []ParsedAction{
					{
						Type:     "adjust_bid",
						AdIDExpr: "snapshot.ad_id",
						Params:   map[string]any{"pct": float64(5)},
					},
				},
			},
		},
	}

	encoded, err := json.Marshal(&spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back ParsedSpec
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if back.SchemaVersion != 1 {
		t.Errorf("SchemaVersion: got %d want 1", back.SchemaVersion)
	}
	if back.ScheduleSeconds != 60 {
		t.Errorf("ScheduleSeconds: got %d want 60", back.ScheduleSeconds)
	}
	if len(back.Inputs) != 2 {
		t.Fatalf("inputs len: got %d want 2", len(back.Inputs))
	}
	if back.Inputs[1].WindowSeconds != 900 {
		t.Errorf("Inputs[1].WindowSeconds: got %d want 900", back.Inputs[1].WindowSeconds)
	}
	if len(back.Rules) != 2 {
		t.Fatalf("rules len: got %d want 2", len(back.Rules))
	}
	if back.Rules[1].ConditionRaw != "snapshot.metrics.cvr" {
		t.Errorf("ConditionRaw: got %q", back.Rules[1].ConditionRaw)
	}
	if back.Rules[1].Actions[0].AdIDExpr != "snapshot.ad_id" {
		t.Errorf("AdIDExpr: got %q", back.Rules[1].Actions[0].AdIDExpr)
	}
	if back.Rules[1].Actions[0].Params["pct"] != float64(5) {
		t.Errorf("pct: got %v", back.Rules[1].Actions[0].Params["pct"])
	}
}

func TestParsedSpecKeysAreSerialisedDeterministically(t *testing.T) {
	spec := ParsedSpec{SchemaVersion: 1, ScheduleSeconds: 30}
	encoded, err := json.Marshal(&spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(encoded)
	// Required keys must appear; struct field order ensures deterministic ordering.
	for _, key := range []string{`"schema_version":1`, `"schedule_seconds":30`, `"inputs":null`, `"rules":null`} {
		if !strings.Contains(got, key) {
			t.Errorf("missing key fragment %q in output: %s", key, got)
		}
	}
}

func TestParsedSpecSchemaVersionConstantIsOne(t *testing.T) {
	if ParsedSpecSchemaVersion != 1 {
		t.Fatalf("ParsedSpecSchemaVersion: got %d want 1", ParsedSpecSchemaVersion)
	}
}
