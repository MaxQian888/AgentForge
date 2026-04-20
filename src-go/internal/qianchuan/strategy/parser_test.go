package strategy

import (
	"errors"
	"strings"
	"testing"
)

const wellFormedYAML = `
name: my-strategy
description: a test
triggers:
  schedule: 1m
inputs:
  - metric: cost
    dimensions: [ad_id]
    window: 1m
rules:
  - name: heartbeat
    condition: "true"
    actions:
      - type: notify_im
        target: {}
        params:
          channel: default
          template: "tick"
`

func TestParseWellFormed(t *testing.T) {
	strat, parsed, err := Parse(wellFormedYAML)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if strat.Name != "my-strategy" {
		t.Errorf("name: got %q", strat.Name)
	}
	if parsed.ScheduleSeconds != 60 {
		t.Errorf("schedule seconds: got %d want 60", parsed.ScheduleSeconds)
	}
	if parsed.SchemaVersion != ParsedSpecSchemaVersion {
		t.Errorf("schema version: got %d", parsed.SchemaVersion)
	}
	if len(parsed.Rules) != 1 {
		t.Fatalf("rules len: got %d", len(parsed.Rules))
	}
	if parsed.Rules[0].ConditionRaw != "true" {
		t.Errorf("condition: got %q", parsed.Rules[0].ConditionRaw)
	}
	if len(parsed.Inputs) != 1 || parsed.Inputs[0].WindowSeconds != 60 {
		t.Errorf("input window seconds: got %+v", parsed.Inputs)
	}
}

func TestParseEmptyRulesRejected(t *testing.T) {
	yamlSrc := `
name: x
triggers:
  schedule: 1m
inputs: []
rules: []
`
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "at least one rule") {
		t.Errorf("error %q does not mention 'at least one rule'", err.Error())
	}
}

func TestParseScheduleBelowFloorRejected(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, "schedule: 1m", "schedule: 5s", 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "schedule") {
		t.Errorf("expected schedule-related error: %v", err)
	}
}

func TestParseScheduleAboveCeilingRejected(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, "schedule: 1m", "schedule: 2h", 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "schedule") {
		t.Errorf("expected schedule-related error: %v", err)
	}
}

func TestParseUnknownActionRejected(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, "type: notify_im", "type: do_evil", 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	var spe *StrategyParseError
	if errors.As(err, &spe) {
		if spe.Line == 0 {
			t.Errorf("expected parse error to carry a non-zero line, got %+v", spe)
		}
	} else {
		// non-structured errors are tolerated as long as they mention the bad type
	}
	if !strings.Contains(err.Error(), "do_evil") {
		t.Errorf("error should mention bad type: %v", err)
	}
}

func TestParseMissingRequiredActionParamRejected(t *testing.T) {
	yamlSrc := `
name: x
triggers:
  schedule: 1m
inputs: []
rules:
  - name: r1
    condition: "true"
    actions:
      - type: notify_im
        target: {}
        params: {}
`
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "r1") {
		t.Errorf("error should reference rule r1: %v", err)
	}
	if !strings.Contains(err.Error(), "notify_im") {
		t.Errorf("error should reference action notify_im: %v", err)
	}
}

func TestParseEmptyConditionRejected(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, `condition: "true"`, `condition: ""`, 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "condition") {
		t.Errorf("error should reference condition: %v", err)
	}
}

func TestParseWhitespaceConditionRejected(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, `condition: "true"`, `condition: "   "`, 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "condition") {
		t.Errorf("error should reference condition: %v", err)
	}
}

func TestParseDuplicateRuleNamesRejected(t *testing.T) {
	yamlSrc := `
name: x
triggers:
  schedule: 1m
inputs: []
rules:
  - name: dup
    condition: "true"
    actions:
      - type: pause_ad
        target: {}
        params: {}
  - name: dup
    condition: "true"
    actions:
      - type: pause_ad
        target: {}
        params: {}
`
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate-related error: %v", err)
	}
}

func TestParseYAMLSyntaxErrorCarriesLine(t *testing.T) {
	yamlSrc := `
name: "broken
triggers:
  schedule: 1m
`
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected yaml syntax error")
	}
	var spe *StrategyParseError
	if errors.As(err, &spe) {
		if spe.Line == 0 {
			t.Errorf("expected line info on parse error: %+v", spe)
		}
	}
}

func TestParseNameTooLongRejected(t *testing.T) {
	long := strings.Repeat("a", 200)
	yamlSrc := strings.Replace(wellFormedYAML, "name: my-strategy", "name: "+long, 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected name-length error")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("expected name-related error: %v", err)
	}
}

func TestParseInputWindowDurations(t *testing.T) {
	cases := []struct {
		window string
		want   int
		ok     bool
	}{
		{"1m", 60, true},
		{"15m", 900, true},
		{"1h", 3600, true},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.window, func(t *testing.T) {
			yamlSrc := strings.Replace(wellFormedYAML, "window: 1m", "window: "+tc.window, 1)
			_, parsed, err := Parse(yamlSrc)
			if !tc.ok {
				if err == nil {
					t.Fatalf("expected error for window=%q", tc.window)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if parsed.Inputs[0].WindowSeconds != tc.want {
				t.Errorf("window seconds: got %d want %d", parsed.Inputs[0].WindowSeconds, tc.want)
			}
		})
	}
}

func TestParseScheduleAtFloorAccepted(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, "schedule: 1m", "schedule: 10s", 1)
	_, parsed, err := Parse(yamlSrc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.ScheduleSeconds != 10 {
		t.Errorf("schedule seconds: got %d want 10", parsed.ScheduleSeconds)
	}
}

func TestParseScheduleAtCeilingAccepted(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, "schedule: 1m", "schedule: 1h", 1)
	_, parsed, err := Parse(yamlSrc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.ScheduleSeconds != 3600 {
		t.Errorf("schedule seconds: got %d want 3600", parsed.ScheduleSeconds)
	}
}

func TestParseEmptyNameRejected(t *testing.T) {
	yamlSrc := strings.Replace(wellFormedYAML, "name: my-strategy", `name: ""`, 1)
	_, _, err := Parse(yamlSrc)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should reference name: %v", err)
	}
}
