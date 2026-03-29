package main

import "testing"

func TestDescribeExposesWorkflowMetadata(t *testing.T) {
  descriptor, err := workflowPlugin{}.Describe(nil)
  if err != nil {
    t.Fatalf("describe plugin: %v", err)
  }
  if descriptor.Kind != "WorkflowPlugin" {
    t.Fatalf("expected workflow kind, got %s", descriptor.Kind)
  }
  if descriptor.ID != "standard-dev-flow" {
    t.Fatalf("expected plugin id standard-dev-flow, got %s", descriptor.ID)
  }
}
