package service

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

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

type PluginListFilter struct {
	Kind           model.PluginKind
	LifecycleState model.PluginLifecycleState
}

type PluginService struct {
	repo          PluginRegistry
	runtimeClient ToolPluginRuntimeClient
	builtInsDir   string
}

func NewPluginService(repo PluginRegistry, runtimeClient ToolPluginRuntimeClient, builtInsDir string) *PluginService {
	return &PluginService{
		repo:          repo,
		runtimeClient: runtimeClient,
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
		PluginManifest:  *manifest,
		LifecycleState:  model.PluginStateInstalled,
		RuntimeHost:     resolveRuntimeHost(manifest.Kind),
		RestartCount:    0,
		LastHealthAt:    nil,
		LastError:       "",
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
		now := time.Now().UTC()
		record.LifecycleState = model.PluginStateActive
		record.RuntimeHost = model.PluginHostGoOrchestrator
		record.LastHealthAt = &now
		record.LastError = ""
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
	if record.Kind != model.PluginKindTool || s.runtimeClient == nil {
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

func (s *PluginService) Restart(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.Kind != model.PluginKindTool || s.runtimeClient == nil {
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
}
