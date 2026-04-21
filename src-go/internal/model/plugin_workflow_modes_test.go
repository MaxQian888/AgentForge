package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkflowPluginSpec_HierarchicalFields(t *testing.T) {
	raw := `
process: hierarchical
managerRole: project-assistant
workerRoles: [coding-agent, test-engineer]
maxParallelWorkers: 2
workerFailurePolicy: best_effort
aggregation: manager_summarize
`
	var spec WorkflowPluginSpec
	if err := yaml.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if spec.Process != WorkflowProcessHierarchical {
		t.Errorf("Process = %q, want hierarchical", spec.Process)
	}
	if spec.ManagerRole != "project-assistant" {
		t.Errorf("ManagerRole = %q", spec.ManagerRole)
	}
	if len(spec.WorkerRoles) != 2 {
		t.Errorf("WorkerRoles len = %d, want 2", len(spec.WorkerRoles))
	}
	if spec.MaxParallelWorkers != 2 {
		t.Errorf("MaxParallelWorkers = %d, want 2", spec.MaxParallelWorkers)
	}
	if spec.WorkerFailurePolicy != "best_effort" {
		t.Errorf("WorkerFailurePolicy = %q", spec.WorkerFailurePolicy)
	}
	if spec.Aggregation != "manager_summarize" {
		t.Errorf("Aggregation = %q", spec.Aggregation)
	}
}

func TestPluginWorkflowTrigger_EventDrivenFields(t *testing.T) {
	raw := `
event: integration.im.message_received
filter:
  channel: general
  contains_mention: true
role: project-assistant
action: reply
maxConcurrent: 2
`
	var trigger PluginWorkflowTrigger
	if err := yaml.Unmarshal([]byte(raw), &trigger); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if trigger.Role != "project-assistant" {
		t.Errorf("Role = %q", trigger.Role)
	}
	if trigger.Action != "reply" {
		t.Errorf("Action = %q", trigger.Action)
	}
	if trigger.MaxConcurrent != 2 {
		t.Errorf("MaxConcurrent = %d, want 2", trigger.MaxConcurrent)
	}
	if trigger.Filter["channel"] != "general" {
		t.Errorf("Filter[channel] = %v", trigger.Filter["channel"])
	}
	if trigger.Filter["contains_mention"] != true {
		t.Errorf("Filter[contains_mention] = %v", trigger.Filter["contains_mention"])
	}
}
