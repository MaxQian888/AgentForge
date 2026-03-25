package pluginsdk

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

type testPlugin struct{}

func (testPlugin) Describe(ctx *Context) (*Descriptor, error) {
	return &Descriptor{
		ID:          "sample-plugin",
		Name:        "Sample Plugin",
		ABIVersion:  ABIVersion,
		Runtime:     "wasm",
		Description: "sample plugin for sdk tests",
		Capabilities: []Capability{
			{
				Name:        "send_message",
				Description: "Send a message to a chat target",
			},
		},
	}, nil
}

func (testPlugin) Init(ctx *Context) error {
	return nil
}

func (testPlugin) Health(ctx *Context) (*Result, error) {
	return Success(map[string]any{
		"status":    "ok",
		"operation": ctx.Operation(),
	}), nil
}

func (testPlugin) Invoke(ctx *Context, invocation Invocation) (*Result, error) {
	if !ctx.CapabilityAllowed(invocation.Operation) {
		return nil, NewRuntimeError("capability_denied", "operation not allowed")
	}
	return Success(map[string]any{
		"status":             "sent",
		"operation":          invocation.Operation,
		"payload":            invocation.Payload,
		"context_operation":  ctx.Operation(),
		"allowed_operations": ctx.AllowedCapabilities(),
	}), nil
}

func TestRuntime_DescribeEncodesTypedDescriptor(t *testing.T) {
	runtime := NewRuntime(testPlugin{})

	stdout, _, exitCode := runRuntimeWithEnv(t, runtime, map[string]string{
		"AGENTFORGE_OPERATION": "describe",
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var envelope struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected ok envelope, got %+v", envelope)
	}
	if envelope.Data["id"] != "sample-plugin" {
		t.Fatalf("expected descriptor id, got %+v", envelope.Data)
	}
	if envelope.Data["abiVersion"] != ABIVersion {
		t.Fatalf("expected abi version %q, got %+v", ABIVersion, envelope.Data)
	}

	capabilities, ok := envelope.Data["capabilities"].([]any)
	if !ok || len(capabilities) != 1 {
		t.Fatalf("expected typed capabilities, got %+v", envelope.Data["capabilities"])
	}

	capability, ok := capabilities[0].(map[string]any)
	if !ok {
		t.Fatalf("expected capability object, got %+v", capabilities[0])
	}
	if capability["name"] != "send_message" {
		t.Fatalf("expected capability name, got %+v", capability)
	}
}

func TestRuntime_InvokeExposesBoundedExecutionContext(t *testing.T) {
	runtime := NewRuntime(testPlugin{})

	stdout, _, exitCode := runRuntimeWithEnv(t, runtime, map[string]string{
		"AGENTFORGE_OPERATION":    "send_message",
		"AGENTFORGE_CAPABILITIES": `["send_message","health"]`,
		"AGENTFORGE_PAYLOAD":      `{"chat_id":"chat-1","content":"hello"}`,
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	var envelope struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	if envelope.Data["context_operation"] != "send_message" {
		t.Fatalf("expected context operation, got %+v", envelope.Data)
	}
	allowed, ok := envelope.Data["allowed_operations"].([]any)
	if !ok || len(allowed) != 2 {
		t.Fatalf("expected allowed operations, got %+v", envelope.Data["allowed_operations"])
	}
}

func TestExportHelpersDelegateToRuntime(t *testing.T) {
	runtime := NewRuntime(testPlugin{})

	if got := ExportABIVersion(runtime); got != ABIVersionNumber {
		t.Fatalf("expected abi version number %d, got %d", ABIVersionNumber, got)
	}
}

func TestRuntime_InvokeEncodesStructuredRuntimeError(t *testing.T) {
	runtime := NewRuntime(testPlugin{})

	stdout, _, exitCode := runRuntimeWithEnv(t, runtime, map[string]string{
		"AGENTFORGE_OPERATION": "send_message",
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	if envelope.OK {
		t.Fatalf("expected error envelope, got %+v", envelope)
	}
	if envelope.Error.Code != "capability_denied" {
		t.Fatalf("expected structured error code, got %+v", envelope.Error)
	}
}

func runRuntimeWithEnv(t *testing.T, runtime *Runtime, env map[string]string) ([]byte, []byte, uint32) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	oldEnviron := make(map[string]string, len(env))
	for key, value := range env {
		oldEnviron[key], _ = os.LookupEnv(key)
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("set env %s: %v", key, err)
		}
	}

	exitCode := runtime.Run()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	for key, value := range oldEnviron {
		if value == "" {
			_ = os.Unsetenv(key)
			continue
		}
		_ = os.Setenv(key, value)
	}

	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return stdout, stderr, exitCode
}
