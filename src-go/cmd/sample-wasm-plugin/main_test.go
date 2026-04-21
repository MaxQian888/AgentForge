package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

func TestDescribeExposesIntegrationMetadata(t *testing.T) {
	descriptor, err := samplePlugin{}.Describe(&pluginsdk.Context{})
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}

	if descriptor.Kind != "IntegrationPlugin" {
		t.Fatalf("descriptor.Kind = %q, want %q", descriptor.Kind, "IntegrationPlugin")
	}
	if descriptor.ID != "sample-integration-plugin" {
		t.Fatalf("descriptor.ID = %q, want %q", descriptor.ID, "sample-integration-plugin")
	}
	if descriptor.Runtime != "wasm" {
		t.Fatalf("descriptor.Runtime = %q, want %q", descriptor.Runtime, "wasm")
	}
	if descriptor.ABIVersion != pluginsdk.ABIVersion {
		t.Fatalf("descriptor.ABIVersion = %q, want %q", descriptor.ABIVersion, pluginsdk.ABIVersion)
	}
	if len(descriptor.Capabilities) != 2 {
		t.Fatalf("len(descriptor.Capabilities) = %d, want 2", len(descriptor.Capabilities))
	}
}

func TestInitHealthAndInvokeCoverHappyPathAndUnsupportedOperation(t *testing.T) {
	plugin := samplePlugin{}
	ctx := &pluginsdk.Context{}

	if err := plugin.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	health, err := plugin.Health(ctx)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.Data["status"] != "ok" {
		t.Fatalf("Health().Data[status] = %v, want ok", health.Data["status"])
	}
	if health.Data["mode"] != "" {
		t.Fatalf("Health().Data[mode] = %v, want empty string", health.Data["mode"])
	}

	result, err := plugin.Invoke(ctx, pluginsdk.Invocation{
		Operation: "echo",
		Payload: map[string]any{
			"key": "value",
		},
	})
	if err != nil {
		t.Fatalf("Invoke(echo) error = %v", err)
	}
	if result.Data["key"] != "value" {
		t.Fatalf("Invoke(echo).Data[key] = %v, want value", result.Data["key"])
	}

	_, err = plugin.Invoke(ctx, pluginsdk.Invocation{Operation: "unsupported"})
	runtimeErr, ok := err.(*pluginsdk.RuntimeError)
	if !ok {
		t.Fatalf("Invoke(unsupported) error type = %T, want *pluginsdk.RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_operation" {
		t.Fatalf("runtimeErr.Code = %q, want unsupported_operation", runtimeErr.Code)
	}
	if runtimeErr.Details["operation"] != "unsupported" {
		t.Fatalf("runtimeErr.Details[operation] = %v, want unsupported", runtimeErr.Details["operation"])
	}
}

func TestAgentforgeExportsUseSDKRuntime(t *testing.T) {
	if got := agentforgeABIVersion(); got != pluginsdk.ABIVersionNumber {
		t.Fatalf("agentforgeABIVersion() = %d, want %d", got, pluginsdk.ABIVersionNumber)
	}

	t.Setenv("AGENTFORGE_OPERATION", "health")
	t.Setenv("AGENTFORGE_CONFIG", `{"mode":"dry-run"}`)
	t.Setenv("AGENTFORGE_PAYLOAD", "")
	t.Setenv("AGENTFORGE_CAPABILITIES", "")

	stdout := captureStdout(t, func() {
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
	if envelope.Operation != "health" {
		t.Fatalf("envelope.Operation = %q, want %q", envelope.Operation, "health")
	}
	if envelope.Data["status"] != "ok" {
		t.Fatalf("envelope.Data[status] = %v, want ok", envelope.Data["status"])
	}
	if envelope.Data["mode"] != "dry-run" {
		t.Fatalf("envelope.Data[mode] = %v, want dry-run", envelope.Data["mode"])
	}
}

func captureStdout(t *testing.T, fn func()) string {
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
