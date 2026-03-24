package service

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/react-go-quick-starter/server/internal/model"
	pluginparser "github.com/react-go-quick-starter/server/internal/plugin"
)

type PluginRegistry interface {
	Save(ctx context.Context, record *model.PluginRecord) error
	GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error)
	List(ctx context.Context, filter model.PluginFilter) ([]*model.PluginRecord, error)
}

type ToolPluginRuntimeClient interface {
	RegisterToolPlugin(ctx context.Context, manifest model.PluginManifest) (*model.PluginRuntimeStatus, error)
	ActivateToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error)
	CheckToolPluginHealth(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error)
	RestartToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error)
}

type GoPluginRuntime interface {
	ActivatePlugin(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error)
	CheckPluginHealth(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error)
	RestartPlugin(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error)
	Invoke(ctx context.Context, record model.PluginRecord, operation string, payload map[string]any) (map[string]any, error)
}

type PluginListFilter struct {
	Kind           model.PluginKind
	LifecycleState model.PluginLifecycleState
}

type PluginService struct {
	repo          PluginRegistry
	runtimeClient ToolPluginRuntimeClient
	goRuntime     GoPluginRuntime
	builtInsDir   string
}

func NewPluginService(repo PluginRegistry, runtimeClient ToolPluginRuntimeClient, goRuntime GoPluginRuntime, builtInsDir string) *PluginService {
	return &PluginService{
		repo:          repo,
		runtimeClient: runtimeClient,
		goRuntime:     goRuntime,
		builtInsDir:   builtInsDir,
	}
}

func (s *PluginService) DiscoverBuiltIns(ctx context.Context) ([]*model.PluginRecord, error) {
	records := make([]*model.PluginRecord, 0)
	err := filepath.WalkDir(s.builtInsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "manifest.yaml" {
			return nil
		}

		record, regErr := s.registerPath(ctx, path, model.PluginSourceBuiltin)
		if regErr != nil {
			return regErr
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover built-ins: %w", err)
	}
	return records, nil
}

func (s *PluginService) RegisterLocalPath(ctx context.Context, path string) (*model.PluginRecord, error) {
	return s.registerPath(ctx, path, model.PluginSourceLocal)
}

func (s *PluginService) registerPath(ctx context.Context, path string, sourceType model.PluginSourceType) (*model.PluginRecord, error) {
	manifest, err := pluginparser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	manifest.Source = model.PluginSource{
		Type: sourceType,
		Path: path,
	}

	record := &model.PluginRecord{
		PluginManifest:     *manifest,
		LifecycleState:     model.PluginStateInstalled,
		RuntimeHost:        resolveRuntimeHost(manifest.Kind),
		RestartCount:       0,
		LastHealthAt:       nil,
		LastError:          "",
		ResolvedSourcePath: resolveSourcePath(*manifest),
		RuntimeMetadata:    initialRuntimeMetadata(*manifest),
	}

	if manifest.Kind == model.PluginKindTool && s.runtimeClient != nil {
		status, err := s.runtimeClient.RegisterToolPlugin(ctx, *manifest)
		if err != nil {
			return nil, fmt.Errorf("register tool plugin in bridge: %w", err)
		}
		applyRuntimeStatus(record, *status)
	}

	if err := s.repo.Save(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *PluginService) List(ctx context.Context, filter PluginListFilter) ([]*model.PluginRecord, error) {
	return s.repo.List(ctx, model.PluginFilter{
		Kind:           filter.Kind,
		LifecycleState: filter.LifecycleState,
	})
}

func (s *PluginService) Enable(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	record.LifecycleState = model.PluginStateEnabled
	record.LastError = ""
	if err := s.repo.Save(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *PluginService) Disable(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	record.LifecycleState = model.PluginStateDisabled
	if err := s.repo.Save(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *PluginService) Activate(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.LifecycleState == model.PluginStateDisabled {
		return nil, fmt.Errorf("plugin %s is disabled", pluginID)
	}

	switch record.Kind {
	case model.PluginKindTool:
		if s.runtimeClient == nil {
			return nil, fmt.Errorf("tool runtime client is not configured")
		}
		status, err := s.runtimeClient.ActivateToolPlugin(ctx, pluginID)
		if err != nil {
			return nil, err
		}
		applyRuntimeStatus(record, *status)
	case model.PluginKindIntegration:
		if record.Spec.Runtime == model.PluginRuntimeGoPlugin {
			return nil, fmt.Errorf("legacy go-plugin integration plugin %s is no longer executable; migrate to runtime: wasm with spec.module and spec.abiVersion", pluginID)
		}
		if s.goRuntime == nil {
			return nil, fmt.Errorf("go plugin runtime is not configured")
		}
		status, err := s.goRuntime.ActivatePlugin(ctx, *record)
		if err != nil {
			return nil, err
		}
		applyRuntimeStatus(record, *status)
	default:
		return nil, fmt.Errorf("plugin %s is not executable in the current phase", pluginID)
	}

	if err := s.repo.Save(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *PluginService) CheckHealth(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.Kind == model.PluginKindTool {
		if s.runtimeClient == nil {
			return record, nil
		}
		status, err := s.runtimeClient.CheckToolPluginHealth(ctx, pluginID)
		if err != nil {
			return nil, err
		}
		applyRuntimeStatus(record, *status)
		if err := s.repo.Save(ctx, record); err != nil {
			return nil, err
		}
		return record, nil
	}
	if record.Kind == model.PluginKindIntegration && record.Spec.Runtime == model.PluginRuntimeWASM {
		if s.goRuntime == nil {
			return record, nil
		}
		status, err := s.goRuntime.CheckPluginHealth(ctx, *record)
		if err != nil {
			return nil, err
		}
		applyRuntimeStatus(record, *status)
		if err := s.repo.Save(ctx, record); err != nil {
			return nil, err
		}
		return record, nil
	}
	if record.Kind != model.PluginKindTool {
		return record, nil
	}
	return record, nil
}

func (s *PluginService) Restart(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.Kind == model.PluginKindTool {
		if s.runtimeClient == nil {
			return nil, fmt.Errorf("plugin %s does not support restart through the TS bridge", pluginID)
		}
		status, err := s.runtimeClient.RestartToolPlugin(ctx, pluginID)
		if err != nil {
			return nil, err
		}
		applyRuntimeStatus(record, *status)
		if err := s.repo.Save(ctx, record); err != nil {
			return nil, err
		}
		return record, nil
	}
	if record.Kind == model.PluginKindIntegration && record.Spec.Runtime == model.PluginRuntimeWASM {
		if s.goRuntime == nil {
			return nil, fmt.Errorf("plugin %s does not support restart because the Go runtime is not configured", pluginID)
		}
		status, err := s.goRuntime.RestartPlugin(ctx, *record)
		if err != nil {
			return nil, err
		}
		applyRuntimeStatus(record, *status)
		if err := s.repo.Save(ctx, record); err != nil {
			return nil, err
		}
		return record, nil
	}
	return nil, fmt.Errorf("plugin %s does not support restart through the configured runtimes", pluginID)
}

func (s *PluginService) ReportRuntimeState(ctx context.Context, pluginID string, status model.PluginRuntimeStatus) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if status.PluginID != "" && status.PluginID != pluginID {
		return nil, fmt.Errorf("runtime update plugin id mismatch: %s != %s", status.PluginID, pluginID)
	}
	applyRuntimeStatus(record, status)
	if err := s.repo.Save(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *PluginService) Invoke(ctx context.Context, pluginID, operation string, payload map[string]any) (map[string]any, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.LifecycleState == model.PluginStateDisabled {
		return nil, fmt.Errorf("plugin %s is disabled", pluginID)
	}
	if record.Kind != model.PluginKindIntegration || record.Spec.Runtime != model.PluginRuntimeWASM {
		return nil, fmt.Errorf("plugin %s does not support Go runtime invocation", pluginID)
	}
	if record.LifecycleState != model.PluginStateActive {
		return nil, fmt.Errorf("plugin %s must be active before invocation", pluginID)
	}
	if s.goRuntime == nil {
		return nil, fmt.Errorf("go plugin runtime is not configured")
	}

	result, err := s.goRuntime.Invoke(ctx, *record, operation, payload)
	if err != nil {
		return nil, err
	}
	record.LastError = ""
	if err := s.repo.Save(ctx, record); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveRuntimeHost(kind model.PluginKind) model.PluginRuntimeHost {
	switch kind {
	case model.PluginKindTool, model.PluginKindReview:
		return model.PluginHostTSBridge
	default:
		return model.PluginHostGoOrchestrator
	}
}

func applyRuntimeStatus(record *model.PluginRecord, status model.PluginRuntimeStatus) {
	record.RuntimeHost = status.Host
	if status.LifecycleState != "" {
		record.LifecycleState = status.LifecycleState
	}
	record.LastError = status.LastError
	record.RestartCount = status.RestartCount
	if status.LastHealthAt != nil {
		record.LastHealthAt = status.LastHealthAt
	}
	if status.ResolvedSourcePath != "" {
		record.ResolvedSourcePath = status.ResolvedSourcePath
	}
	if status.RuntimeMetadata != nil {
		metadata := *status.RuntimeMetadata
		record.RuntimeMetadata = &metadata
	}
}

func resolveSourcePath(manifest model.PluginManifest) string {
	switch manifest.Spec.Runtime {
	case model.PluginRuntimeWASM:
		return manifest.Spec.Module
	case model.PluginRuntimeGoPlugin:
		return manifest.Spec.Binary
	default:
		return manifest.Source.Path
	}
}

func initialRuntimeMetadata(manifest model.PluginManifest) *model.PluginRuntimeMetadata {
	if manifest.Spec.ABIVersion == "" {
		return nil
	}
	return &model.PluginRuntimeMetadata{
		ABIVersion: manifest.Spec.ABIVersion,
		Compatible: true,
	}
}
