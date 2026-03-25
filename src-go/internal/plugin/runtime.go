package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	pluginsdk "github.com/react-go-quick-starter/server/plugin-sdk-go"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const (
	abiVersionExportName = "agentforge_abi_version"
	runExportName        = "agentforge_run"
)

type WASMRuntimeManager struct {
	mu      sync.Mutex
	plugins map[string]*wasmPluginState
}

type wasmPluginState struct {
	RestartCount       int
	ResolvedSourcePath string
	RuntimeMetadata    *model.PluginRuntimeMetadata
}

type wasmEnvelope struct {
	OK        bool           `json:"ok"`
	Operation string         `json:"operation"`
	Data      map[string]any `json:"data"`
	Error     *struct {
		Code    string         `json:"code,omitempty"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error,omitempty"`
}

func NewWASMRuntimeManager() *WASMRuntimeManager {
	return &WASMRuntimeManager{
		plugins: make(map[string]*wasmPluginState),
	}
}

func (m *WASMRuntimeManager) ActivatePlugin(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	describe, err := m.execute(ctx, record, "describe", nil)
	if err != nil {
		return nil, err
	}
	if err := verifyABIVersion(record, describe.abiVersion); err != nil {
		return nil, err
	}

	if _, err := m.execute(ctx, record, "init", nil); err != nil {
		return degradedStatus(record, describe.modulePath, err, 0, describe.abiVersion), err
	}

	m.mu.Lock()
	state := &wasmPluginState{
		ResolvedSourcePath: describe.modulePath,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: describe.abiVersion,
			Compatible: true,
		},
	}
	if existing, ok := m.plugins[record.Metadata.ID]; ok {
		state.RestartCount = existing.RestartCount
	}
	m.plugins[record.Metadata.ID] = state
	m.mu.Unlock()

	return activeStatus(record, state.ResolvedSourcePath, state.RestartCount, state.RuntimeMetadata), nil
}

func (m *WASMRuntimeManager) CheckPluginHealth(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	result, err := m.execute(ctx, record, "health", nil)
	if err != nil {
		return degradedStatus(record, resolveModulePath(record), err, m.restartCount(record.Metadata.ID), record.Spec.ABIVersion), err
	}
	return activeStatus(record, result.modulePath, m.restartCount(record.Metadata.ID), &model.PluginRuntimeMetadata{
		ABIVersion: result.abiVersion,
		Compatible: true,
	}), nil
}

func (m *WASMRuntimeManager) RestartPlugin(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error) {
	m.mu.Lock()
	state := m.plugins[record.Metadata.ID]
	if state == nil {
		state = &wasmPluginState{}
		m.plugins[record.Metadata.ID] = state
	}
	state.RestartCount++
	restartCount := state.RestartCount
	m.mu.Unlock()

	result, err := m.execute(ctx, record, "init", nil)
	if err != nil {
		return degradedStatus(record, resolveModulePath(record), err, restartCount, record.Spec.ABIVersion), err
	}

	m.mu.Lock()
	state.ResolvedSourcePath = result.modulePath
	state.RuntimeMetadata = &model.PluginRuntimeMetadata{
		ABIVersion: result.abiVersion,
		Compatible: true,
	}
	m.mu.Unlock()

	return activeStatus(record, result.modulePath, restartCount, state.RuntimeMetadata), nil
}

func (m *WASMRuntimeManager) Invoke(ctx context.Context, record model.PluginRecord, operation string, payload map[string]any) (map[string]any, error) {
	if err := ensureDeclaredCapability(record, operation); err != nil {
		return nil, err
	}
	result, err := m.execute(ctx, record, operation, payload)
	if err != nil {
		return nil, err
	}
	return result.data, nil
}

type executionResult struct {
	data       map[string]any
	modulePath string
	abiVersion string
	stdout     string
	stderr     string
	envelope   *wasmEnvelope
}

type DebugExecutionResult struct {
	OK         bool           `json:"ok"`
	Operation  string         `json:"operation"`
	Data       map[string]any `json:"data,omitempty"`
	Error      string         `json:"error,omitempty"`
	ErrorCode  string         `json:"errorCode,omitempty"`
	ErrorInfo  map[string]any `json:"errorInfo,omitempty"`
	Stdout     string         `json:"stdout,omitempty"`
	Stderr     string         `json:"stderr,omitempty"`
	ModulePath string         `json:"modulePath,omitempty"`
	ABIVersion string         `json:"abiVersion,omitempty"`
}

func (m *WASMRuntimeManager) runEnvelope(ctx context.Context, record model.PluginRecord, operation string, payload map[string]any) (*executionResult, error) {
	modulePath := resolveModulePath(record)
	if modulePath == "" {
		return nil, fmt.Errorf("plugin %s is missing a wasm module path", record.Metadata.ID)
	}

	wasmBytes, err := os.ReadFile(modulePath)
	if err != nil {
		return nil, fmt.Errorf("read wasm module %s: %w", modulePath, err)
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx) //nolint:errcheck

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return nil, fmt.Errorf("instantiate wasi: %w", err)
	}

	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm module %s: %w", modulePath, err)
	}
	if err := ensureRequiredExports(compiled.ExportedFunctions(), record.Metadata.ID); err != nil {
		return nil, err
	}

	configJSON, _ := json.Marshal(record.Spec.Config)
	payloadJSON, _ := json.Marshal(payload)
	capabilitiesJSON, _ := json.Marshal(record.Spec.Capabilities)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	moduleConfig := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName(record.Metadata.ID).
		WithEnv("AGENTFORGE_AUTORUN", "true").
		WithEnv("AGENTFORGE_OPERATION", operation).
		WithEnv("AGENTFORGE_CONFIG", string(configJSON)).
		WithEnv("AGENTFORGE_CAPABILITIES", string(capabilitiesJSON)).
		WithEnv("AGENTFORGE_PAYLOAD", string(payloadJSON))

	_, err = runtime.InstantiateModule(ctx, compiled, moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("instantiate wasm module %s: %w: %s", modulePath, err, stderr.String())
	}

	var envelope wasmEnvelope
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &envelope); err != nil {
		return nil, fmt.Errorf("decode plugin %s output: %w (stdout=%q stderr=%q)", record.Metadata.ID, err, stdout.String(), stderr.String())
	}

	abiVersion := record.Spec.ABIVersion
	if operation == "describe" {
		if described, ok := envelope.Data["abiVersion"].(string); ok && described != "" {
			abiVersion = described
		}
	}

	return &executionResult{
		data:       envelope.Data,
		modulePath: modulePath,
		abiVersion: abiVersion,
		stdout:     stdout.String(),
		stderr:     stderr.String(),
		envelope:   &envelope,
	}, nil
}

func (m *WASMRuntimeManager) execute(ctx context.Context, record model.PluginRecord, operation string, payload map[string]any) (*executionResult, error) {
	result, err := m.runEnvelope(ctx, record, operation, payload)
	if err != nil {
		return nil, err
	}
	if result.envelope != nil && !result.envelope.OK {
		if result.envelope.Error != nil && result.envelope.Error.Message != "" {
			if result.envelope.Error.Code != "" {
				return result, fmt.Errorf("%s: %s", result.envelope.Error.Code, result.envelope.Error.Message)
			}
			return result, errors.New(result.envelope.Error.Message)
		}
		return result, fmt.Errorf("plugin %s operation %s returned a non-success envelope", record.Metadata.ID, operation)
	}
	return result, nil
}

func (m *WASMRuntimeManager) DebugExecute(ctx context.Context, record model.PluginRecord, operation string, payload map[string]any) (*DebugExecutionResult, error) {
	if operation != "describe" && operation != "init" && operation != "health" {
		if err := ensureDeclaredCapability(record, operation); err != nil {
			return &DebugExecutionResult{
				OK:        false,
				Operation: operation,
				Error:     err.Error(),
			}, nil
		}
	}

	result, err := m.runEnvelope(ctx, record, operation, payload)
	if err != nil {
		return nil, err
	}

	debugResult := &DebugExecutionResult{
		OK:         result.envelope != nil && result.envelope.OK,
		Operation:  operation,
		Data:       result.data,
		Stdout:     result.stdout,
		Stderr:     result.stderr,
		ModulePath: result.modulePath,
		ABIVersion: result.abiVersion,
	}

	if result.envelope != nil && result.envelope.Error != nil {
		debugResult.Error = result.envelope.Error.Message
		debugResult.ErrorCode = result.envelope.Error.Code
		debugResult.ErrorInfo = result.envelope.Error.Details
	}

	return debugResult, nil
}

func verifyABIVersion(record model.PluginRecord, actual string) error {
	expected := record.Spec.ABIVersion
	if expected == "" {
		expected = pluginsdk.ABIVersion
	}
	if actual != expected {
		return fmt.Errorf("plugin %s ABI mismatch: expected %s, got %s", record.Metadata.ID, expected, actual)
	}
	return nil
}

func activeStatus(record model.PluginRecord, modulePath string, restartCount int, metadata *model.PluginRuntimeMetadata) *model.PluginRuntimeStatus {
	now := time.Now().UTC()
	return &model.PluginRuntimeStatus{
		PluginID:           record.Metadata.ID,
		Host:               model.PluginHostGoOrchestrator,
		LifecycleState:     model.PluginStateActive,
		LastHealthAt:       &now,
		RestartCount:       restartCount,
		ResolvedSourcePath: modulePath,
		RuntimeMetadata:    metadata,
	}
}

func degradedStatus(record model.PluginRecord, modulePath string, cause error, restartCount int, abiVersion string) *model.PluginRuntimeStatus {
	now := time.Now().UTC()
	return &model.PluginRuntimeStatus{
		PluginID:           record.Metadata.ID,
		Host:               model.PluginHostGoOrchestrator,
		LifecycleState:     model.PluginStateDegraded,
		LastHealthAt:       &now,
		LastError:          cause.Error(),
		RestartCount:       restartCount,
		ResolvedSourcePath: modulePath,
		RuntimeMetadata: &model.PluginRuntimeMetadata{
			ABIVersion: abiVersion,
			Compatible: false,
		},
	}
}

func resolveModulePath(record model.PluginRecord) string {
	modulePath := record.Spec.Module
	if filepath.IsAbs(modulePath) {
		return modulePath
	}
	base := filepath.Dir(record.Source.Path)
	if base == "." || base == "" {
		return modulePath
	}
	return filepath.Clean(filepath.Join(base, modulePath))
}

func (m *WASMRuntimeManager) restartCount(pluginID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.plugins[pluginID]; ok {
		return state.RestartCount
	}
	return 0
}

func ensureRequiredExports(exports map[string]api.FunctionDefinition, pluginID string) error {
	for _, name := range []string{abiVersionExportName, runExportName} {
		if _, ok := exports[name]; !ok {
			return fmt.Errorf("plugin %s is missing required export %s", pluginID, name)
		}
	}
	return nil
}

func ensureDeclaredCapability(record model.PluginRecord, operation string) error {
	if len(record.Spec.Capabilities) == 0 {
		return nil
	}
	for _, capability := range record.Spec.Capabilities {
		if capability == operation {
			return nil
		}
	}
	return fmt.Errorf("plugin %s operation %s is not declared in spec.capabilities", record.Metadata.ID, operation)
}
