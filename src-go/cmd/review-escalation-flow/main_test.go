package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

func TestDescribeExposesWorkflowMetadata(t *testing.T) {
	descriptor, err := workflowPlugin{}.Describe(&pluginsdk.Context{})
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if descriptor.Kind != "WorkflowPlugin" {
		t.Fatalf("descriptor.Kind = %q, want %q", descriptor.Kind, "WorkflowPlugin")
	}
	if descriptor.ID != "review-escalation-flow" {
		t.Fatalf("descriptor.ID = %q, want %q", descriptor.ID, "review-escalation-flow")
	}
	if descriptor.Runtime != "wasm" {
		t.Fatalf("descriptor.Runtime = %q, want %q", descriptor.Runtime, "wasm")
	}
	if descriptor.ABIVersion != pluginsdk.ABIVersion {
		t.Fatalf("descriptor.ABIVersion = %q, want %q", descriptor.ABIVersion, pluginsdk.ABIVersion)
	}
}

func TestInitHealthAndInvokeCoverHappyPathAndUnsupportedOperation(t *testing.T) {
	plugin := workflowPlugin{}
	ctx := &pluginsdk.Context{}

	if err := plugin.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	health, err := plugin.Health(ctx)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.Data["workflow"] != "review-escalation-flow" {
		t.Fatalf("Health().Data[workflow] = %v, want review-escalation-flow", health.Data["workflow"])
	}

	result, err := plugin.Invoke(ctx, pluginsdk.Invocation{Operation: "run_workflow"})
	if err != nil {
		t.Fatalf("Invoke(run_workflow) error = %v", err)
	}
	if result.Data["workflow"] != "review-escalation-flow" {
		t.Fatalf("Invoke(run_workflow).Data[workflow] = %v, want review-escalation-flow", result.Data["workflow"])
	}

	_, err = plugin.Invoke(ctx, pluginsdk.Invocation{Operation: "unknown"})
	runtimeErr, ok := err.(*pluginsdk.RuntimeError)
	if !ok {
		t.Fatalf("Invoke(unknown) error type = %T, want *pluginsdk.RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_operation" {
		t.Fatalf("runtimeErr.Code = %q, want unsupported_operation", runtimeErr.Code)
	}
}

func TestAgentforgeExportsUseSDKRuntime(t *testing.T) {
	if got := agentforgeABIVersion(); got != pluginsdk.ABIVersionNumber {
		t.Fatalf("agentforgeABIVersion() = %d, want %d", got, pluginsdk.ABIVersionNumber)
	}

	t.Setenv("AGENTFORGE_OPERATION", "health")
	t.Setenv("AGENTFORGE_CONFIG", "")
	t.Setenv("AGENTFORGE_PAYLOAD", "")
	t.Setenv("AGENTFORGE_CAPABILITIES", "")

	stdout := captureWorkflowStdout(t, func() {
		if exitCode := agentforgeRun(); exitCode != 0 {
			t.Fatalf("agentforgeRun() exit code = %d, want 0", exitCode)
		}
	})

	var envelope struct {
		OK        bool           `json:"ok"`
		Operation string         `json:"operation"`
		Data      map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode stdout envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatal("expected successful runtime envelope")
	}
	if envelope.Data["workflow"] != "review-escalation-flow" {
		t.Fatalf("envelope.Data[workflow] = %v, want review-escalation-flow", envelope.Data["workflow"])
	}
}

func captureWorkflowStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer

	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close() error = %v", err)
	}
	return string(data)
}
