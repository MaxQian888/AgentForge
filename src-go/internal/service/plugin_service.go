package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
	pluginparser "github.com/react-go-quick-starter/server/internal/plugin"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type PluginRegistry interface {
	Save(ctx context.Context, record *model.PluginRecord) error
	GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error)
	List(ctx context.Context, filter model.PluginFilter) ([]*model.PluginRecord, error)
	Delete(ctx context.Context, pluginID string) error
}

type PluginInstanceStore interface {
	UpsertCurrent(ctx context.Context, snapshot *model.PluginInstanceSnapshot) error
	GetCurrentByPluginID(ctx context.Context, pluginID string) (*model.PluginInstanceSnapshot, error)
	DeleteByPluginID(ctx context.Context, pluginID string) error
}

type PluginEventAuditStore interface {
	Append(ctx context.Context, event *model.PluginEventRecord) error
	ListByPluginID(ctx context.Context, pluginID string, limit int) ([]*model.PluginEventRecord, error)
}

type PluginEventBroadcaster interface {
	BroadcastPluginEvent(event *model.PluginEventRecord)
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

type PluginRoleStore interface {
	Get(id string) (*rolepkg.Manifest, error)
}

type PluginPolicy struct {
	AllowNetwork    bool
	AllowFilesystem bool
}

type PluginListFilter struct {
	Kind           model.PluginKind
	LifecycleState model.PluginLifecycleState
	SourceType     model.PluginSourceType
	TrustState     model.PluginTrustState
}

type PluginService struct {
	repo          PluginRegistry
	instanceStore PluginInstanceStore
	eventStore    PluginEventAuditStore
	broadcaster   PluginEventBroadcaster
	runtimeClient ToolPluginRuntimeClient
	goRuntime     GoPluginRuntime
	roleStore     PluginRoleStore
	builtInsDir   string
	policy        PluginPolicy
}

func NewPluginService(repo PluginRegistry, runtimeClient ToolPluginRuntimeClient, goRuntime GoPluginRuntime, builtInsDir string) *PluginService {
	return &PluginService{
		repo:          repo,
		instanceStore: repository.NewPluginInstanceRepository(),
		eventStore:    repository.NewPluginEventRepository(),
		runtimeClient: runtimeClient,
		goRuntime:     goRuntime,
		builtInsDir:   builtInsDir,
		policy: PluginPolicy{
			AllowNetwork:    true,
			AllowFilesystem: true,
		},
	}
}

func (s *PluginService) WithInstanceStore(store PluginInstanceStore) *PluginService {
	if store != nil {
		s.instanceStore = store
	}
	return s
}

func (s *PluginService) WithEventStore(store PluginEventAuditStore) *PluginService {
	if store != nil {
		s.eventStore = store
	}
	return s
}

func (s *PluginService) WithBroadcaster(broadcaster PluginEventBroadcaster) *PluginService {
	s.broadcaster = broadcaster
	return s
}

func (s *PluginService) WithRoleStore(store PluginRoleStore) *PluginService {
	if store != nil {
		s.roleStore = store
	}
	return s
}

func (s *PluginService) WithPolicy(policy PluginPolicy) *PluginService {
	s.policy = policy
	return s
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
	resolvedPath := path
	if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
		resolvedPath = filepath.Join(path, "manifest.yaml")
	}

	manifest, err := pluginparser.ParseFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	manifest.Source = model.PluginSource{
		Type: sourceType,
		Path: resolvedPath,
	}
	if err := s.validateWorkflowManifest(manifest); err != nil {
		return nil, err
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

	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventInstalled, model.PluginEventSourceControlPlane, "plugin installed", map[string]any{
		"source_type": sourceType,
		"source_path": resolvedPath,
	}); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) List(ctx context.Context, filter PluginListFilter) ([]*model.PluginRecord, error) {
	records, err := s.repo.List(ctx, model.PluginFilter{
		Kind:           filter.Kind,
		LifecycleState: filter.LifecycleState,
		SourceType:     filter.SourceType,
		TrustState:     filter.TrustState,
	})
	if err != nil {
		return nil, err
	}

	hydrated := make([]*model.PluginRecord, 0, len(records))
	for _, record := range records {
		hydrated = append(hydrated, s.hydrateRecord(ctx, record))
	}
	return hydrated, nil
}

func (s *PluginService) Enable(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	record.LifecycleState = model.PluginStateEnabled
	record.LastError = ""
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventEnabled, model.PluginEventSourceOperator, "plugin enabled", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) Disable(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	record.LifecycleState = model.PluginStateDisabled
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventDisabled, model.PluginEventSourceOperator, "plugin disabled", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) Activate(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.LifecycleState == model.PluginStateDisabled {
		return nil, fmt.Errorf("plugin %s is disabled", pluginID)
	}
	if err := s.validateActivationPermissions(record); err != nil {
		record.LastError = err.Error()
		record.LifecycleState = model.PluginStateDegraded
		_ = s.persistRecord(ctx, record)
		_ = s.appendEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceControlPlane, err.Error(), map[string]any{"operation": "activate"})
		return nil, err
	}

	record.LifecycleState = model.PluginStateActivating
	record.LastError = ""
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventActivating, eventSourceForRecord(record), "plugin activating", nil); err != nil {
		return nil, err
	}

	switch record.Kind {
	case model.PluginKindTool:
		if s.runtimeClient == nil {
			return s.failActivation(ctx, record, fmt.Errorf("tool runtime client is not configured"))
		}
		status, err := s.runtimeClient.ActivateToolPlugin(ctx, pluginID)
		if err != nil {
			return s.failActivation(ctx, record, err)
		}
		applyRuntimeStatus(record, *status)
	case model.PluginKindIntegration:
		if record.Spec.Runtime == model.PluginRuntimeGoPlugin {
			return s.failActivation(ctx, record, fmt.Errorf("legacy go-plugin integration plugin %s is no longer executable; migrate to runtime: wasm with spec.module and spec.abiVersion", pluginID))
		}
		if s.goRuntime == nil {
			return s.failActivation(ctx, record, fmt.Errorf("go plugin runtime is not configured"))
		}
		status, err := s.goRuntime.ActivatePlugin(ctx, *record)
		if err != nil {
			return s.failActivation(ctx, record, err)
		}
		applyRuntimeStatus(record, *status)
	case model.PluginKindWorkflow:
		if record.Spec.Workflow == nil {
			return s.failActivation(ctx, record, fmt.Errorf("workflow plugin %s is missing spec.workflow", pluginID))
		}
		if !isExecutableWorkflowProcess(record.Spec.Workflow.Process) {
			return s.failActivation(ctx, record, unsupportedWorkflowProcessError(record))
		}
		if s.goRuntime == nil {
			return s.failActivation(ctx, record, fmt.Errorf("go plugin runtime is not configured"))
		}
		status, err := s.goRuntime.ActivatePlugin(ctx, *record)
		if err != nil {
			return s.failActivation(ctx, record, err)
		}
		applyRuntimeStatus(record, *status)
	default:
		return s.failActivation(ctx, record, fmt.Errorf("plugin %s is not executable in the current phase", pluginID))
	}

	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventActivated, eventSourceForRecord(record), "plugin activated", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) CheckHealth(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}

	var status *model.PluginRuntimeStatus
	switch {
	case record.Kind == model.PluginKindTool:
		if s.runtimeClient == nil {
			return s.hydrateRecord(ctx, record), nil
		}
		status, err = s.runtimeClient.CheckToolPluginHealth(ctx, pluginID)
	case isGoHostedHealthPlugin(record):
		if s.goRuntime == nil {
			return s.hydrateRecord(ctx, record), nil
		}
		status, err = s.goRuntime.CheckPluginHealth(ctx, *record)
	default:
		return s.hydrateRecord(ctx, record), nil
	}
	if err != nil {
		record.LifecycleState = model.PluginStateDegraded
		record.LastError = err.Error()
		if persistErr := s.persistRecord(ctx, record); persistErr != nil {
			return nil, persistErr
		}
		_ = s.appendEvent(ctx, record, model.PluginEventFailed, eventSourceForRecord(record), err.Error(), map[string]any{"operation": "health"})
		return nil, err
	}

	applyRuntimeStatus(record, *status)
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventHealth, eventSourceForRecord(record), "plugin health updated", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) Restart(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}

	var status *model.PluginRuntimeStatus
	switch {
	case record.Kind == model.PluginKindTool:
		if s.runtimeClient == nil {
			return nil, fmt.Errorf("plugin %s does not support restart through the TS bridge", pluginID)
		}
		status, err = s.runtimeClient.RestartToolPlugin(ctx, pluginID)
	case isGoHostedHealthPlugin(record):
		if s.goRuntime == nil {
			return nil, fmt.Errorf("plugin %s does not support restart because the Go runtime is not configured", pluginID)
		}
		status, err = s.goRuntime.RestartPlugin(ctx, *record)
	default:
		return nil, fmt.Errorf("plugin %s does not support restart through the configured runtimes", pluginID)
	}
	if err != nil {
		record.LifecycleState = model.PluginStateDegraded
		record.LastError = err.Error()
		if persistErr := s.persistRecord(ctx, record); persistErr != nil {
			return nil, persistErr
		}
		_ = s.appendEvent(ctx, record, model.PluginEventFailed, eventSourceForRecord(record), err.Error(), map[string]any{"operation": "restart"})
		return nil, err
	}

	applyRuntimeStatus(record, *status)
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventRestarted, eventSourceForRecord(record), "plugin restarted", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
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
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventRuntimeSync, eventSourceFromHost(status.Host), "runtime state synchronized", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
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
	if err := s.validateInvocation(record, operation); err != nil {
		record.LastError = err.Error()
		if persistErr := s.persistRecord(ctx, record); persistErr != nil {
			return nil, persistErr
		}
		_ = s.appendEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceControlPlane, err.Error(), map[string]any{"operation": operation})
		return nil, err
	}

	result, err := s.goRuntime.Invoke(ctx, *record, operation, payload)
	if err != nil {
		record.LifecycleState = model.PluginStateDegraded
		record.LastError = err.Error()
		if persistErr := s.persistRecord(ctx, record); persistErr != nil {
			return nil, persistErr
		}
		_ = s.appendEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceGoRuntime, err.Error(), map[string]any{"operation": operation})
		return nil, err
	}
	record.LastError = ""
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, record, model.PluginEventInvoked, model.PluginEventSourceGoRuntime, "plugin invoked", map[string]any{"operation": operation}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PluginService) Uninstall(ctx context.Context, id string) error {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}
	if rec.LifecycleState == model.PluginStateActive {
		if _, err := s.Disable(ctx, id); err != nil {
			return err
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete plugin: %w", err)
	}
	if s.instanceStore != nil {
		_ = s.instanceStore.DeleteByPluginID(ctx, id)
	}
	if err := s.appendEvent(ctx, rec, model.PluginEventUninstalled, model.PluginEventSourceOperator, "plugin uninstalled", nil); err != nil {
		return err
	}
	return nil
}

func (s *PluginService) UpdateConfig(ctx context.Context, id string, config map[string]interface{}) (*model.PluginRecord, error) {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}
	rec.Spec.Config = config
	if err := s.persistRecord(ctx, rec); err != nil {
		return nil, fmt.Errorf("update plugin config: %w", err)
	}
	return s.hydrateRecord(ctx, rec), nil
}

func (s *PluginService) ListMarketplace(ctx context.Context) ([]model.MarketplacePluginDTO, error) {
	installed, err := s.repo.List(ctx, model.PluginFilter{})
	if err != nil {
		return nil, err
	}
	installedByID := make(map[string]*model.PluginRecord, len(installed))
	for _, record := range installed {
		installedByID[record.Metadata.ID] = record
	}

	catalog := make(map[string]model.MarketplacePluginDTO)
	err = filepath.WalkDir(s.builtInsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Base(path) != "manifest.yaml" {
			return nil
		}
		manifest, parseErr := pluginparser.ParseFile(path)
		if parseErr != nil {
			return nil
		}
		manifest.Source = model.PluginSource{
			Type: model.PluginSourceBuiltin,
			Path: path,
		}
		catalog[manifest.Metadata.ID] = marketplaceFromManifest(*manifest, installedByID[manifest.Metadata.ID])
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list plugin catalog: %w", err)
	}

	for _, record := range installed {
		if _, ok := catalog[record.Metadata.ID]; ok {
			continue
		}
		catalog[record.Metadata.ID] = marketplaceFromManifest(record.PluginManifest, record)
	}

	items := make([]model.MarketplacePluginDTO, 0, len(catalog))
	for _, item := range catalog {
		items = append(items, item)
	}
	return items, nil
}

func (s *PluginService) ListEvents(ctx context.Context, pluginID string, limit int) ([]*model.PluginEventRecord, error) {
	if s.eventStore == nil {
		return []*model.PluginEventRecord{}, nil
	}
	return s.eventStore.ListByPluginID(ctx, pluginID, limit)
}

func (s *PluginService) failActivation(ctx context.Context, record *model.PluginRecord, cause error) (*model.PluginRecord, error) {
	record.LifecycleState = model.PluginStateDegraded
	record.LastError = cause.Error()
	if err := s.persistRecord(ctx, record); err != nil {
		return nil, err
	}
	_ = s.appendEvent(ctx, record, model.PluginEventFailed, eventSourceForRecord(record), cause.Error(), map[string]any{"operation": "activate"})
	return nil, cause
}

func (s *PluginService) persistRecord(ctx context.Context, record *model.PluginRecord) error {
	if err := s.repo.Save(ctx, record); err != nil {
		return err
	}
	if s.instanceStore == nil {
		return nil
	}
	snapshot := &model.PluginInstanceSnapshot{
		PluginID:           record.Metadata.ID,
		RuntimeHost:        record.RuntimeHost,
		LifecycleState:     record.LifecycleState,
		ResolvedSourcePath: record.ResolvedSourcePath,
		RestartCount:       record.RestartCount,
		LastHealthAt:       record.LastHealthAt,
		LastError:          record.LastError,
	}
	if record.RuntimeMetadata != nil {
		metadata := *record.RuntimeMetadata
		snapshot.RuntimeMetadata = &metadata
	}
	if err := s.instanceStore.UpsertCurrent(ctx, snapshot); err != nil {
		return err
	}
	return nil
}

func (s *PluginService) hydrateRecord(ctx context.Context, record *model.PluginRecord) *model.PluginRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	if record.RuntimeMetadata != nil {
		metadata := *record.RuntimeMetadata
		cloned.RuntimeMetadata = &metadata
	}
	if s.instanceStore != nil {
		if snapshot, err := s.instanceStore.GetCurrentByPluginID(ctx, record.Metadata.ID); err == nil {
			cloned.CurrentInstance = snapshot
		}
	}
	return &cloned
}

func (s *PluginService) appendEvent(ctx context.Context, record *model.PluginRecord, eventType model.PluginEventType, source model.PluginEventSource, summary string, payload map[string]any) error {
	if record == nil || s.eventStore == nil {
		return nil
	}
	event := &model.PluginEventRecord{
		PluginID:       record.Metadata.ID,
		EventType:      eventType,
		EventSource:    source,
		LifecycleState: record.LifecycleState,
		Summary:        summary,
		Payload:        payload,
	}
	if err := s.eventStore.Append(ctx, event); err != nil {
		return err
	}
	if s.broadcaster != nil {
		s.broadcaster.BroadcastPluginEvent(event)
	}
	return nil
}

func (s *PluginService) validateActivationPermissions(record *model.PluginRecord) error {
	if record == nil {
		return fmt.Errorf("plugin record is required")
	}
	if record.Permissions.Network != nil && record.Permissions.Network.Required {
		if !s.policy.AllowNetwork {
			return fmt.Errorf("plugin %s requires network permission but the current server policy disallows it", record.Metadata.ID)
		}
		if len(record.Permissions.Network.Domains) == 0 {
			return fmt.Errorf("plugin %s requires network permission declarations with at least one allowed domain", record.Metadata.ID)
		}
	}
	if record.Permissions.Filesystem != nil && record.Permissions.Filesystem.Required {
		if !s.policy.AllowFilesystem {
			return fmt.Errorf("plugin %s requires filesystem permission but the current server policy disallows it", record.Metadata.ID)
		}
		if len(record.Permissions.Filesystem.AllowedPaths) == 0 {
			return fmt.Errorf("plugin %s requires filesystem permission declarations with at least one allowed path", record.Metadata.ID)
		}
	}
	return nil
}

func (s *PluginService) validateInvocation(record *model.PluginRecord, operation string) error {
	if operation == "" {
		return fmt.Errorf("operation is required")
	}
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

func (s *PluginService) validateWorkflowManifest(manifest *model.PluginManifest) error {
	if manifest == nil || manifest.Kind != model.PluginKindWorkflow || manifest.Spec.Workflow == nil {
		return nil
	}
	if s.roleStore == nil {
		return fmt.Errorf("workflow role store is not configured")
	}

	workflow := manifest.Spec.Workflow
	roleIDs := make(map[string]struct{}, len(workflow.Roles))
	for _, binding := range workflow.Roles {
		roleID := strings.TrimSpace(binding.ID)
		if roleID == "" {
			return fmt.Errorf("workflow role id is required")
		}
		if _, exists := roleIDs[roleID]; exists {
			return fmt.Errorf("duplicate workflow role reference: %s", roleID)
		}
		if _, err := s.roleStore.Get(roleID); err != nil {
			return fmt.Errorf("unknown workflow role reference: %s", roleID)
		}
		roleIDs[roleID] = struct{}{}
	}

	stepIDs := make(map[string]struct{}, len(workflow.Steps))
	for _, step := range workflow.Steps {
		stepID := strings.TrimSpace(step.ID)
		if stepID == "" {
			return fmt.Errorf("workflow step id is required")
		}
		if _, exists := stepIDs[stepID]; exists {
			return fmt.Errorf("duplicate workflow step id: %s", stepID)
		}
		if !isSupportedWorkflowAction(step.Action) {
			return fmt.Errorf("unsupported workflow action: %s", step.Action)
		}

		roleID := strings.TrimSpace(step.Role)
		if roleID == "" {
			return fmt.Errorf("workflow step %s must declare a role", stepID)
		}
		if _, exists := roleIDs[roleID]; !exists {
			if _, err := s.roleStore.Get(roleID); err != nil {
				return fmt.Errorf("unknown workflow role reference: %s", roleID)
			}
			roleIDs[roleID] = struct{}{}
		}
		stepIDs[stepID] = struct{}{}
	}

	for _, step := range workflow.Steps {
		for _, nextID := range step.Next {
			normalizedNextID := strings.TrimSpace(nextID)
			if normalizedNextID == "" {
				return fmt.Errorf("workflow step %s declares an empty next transition", step.ID)
			}
			if _, exists := stepIDs[normalizedNextID]; !exists {
				return fmt.Errorf("unknown workflow step transition: %s -> %s", step.ID, normalizedNextID)
			}
		}
	}
	return nil
}

func isSupportedWorkflowAction(action model.WorkflowActionType) bool {
	switch action {
	case model.WorkflowActionAgent, model.WorkflowActionReview, model.WorkflowActionTask:
		return true
	default:
		return false
	}
}

func isExecutableWorkflowProcess(process model.WorkflowProcessMode) bool {
	return process == model.WorkflowProcessSequential
}

func unsupportedWorkflowProcessError(record *model.PluginRecord) error {
	if record == nil || record.Spec.Workflow == nil {
		return fmt.Errorf("unsupported workflow process")
	}
	return fmt.Errorf(
		"unsupported workflow process %q for plugin %s; only sequential workflows are executable in the current phase",
		record.Spec.Workflow.Process,
		record.Metadata.ID,
	)
}

func isGoHostedHealthPlugin(record *model.PluginRecord) bool {
	if record == nil {
		return false
	}
	if record.Kind == model.PluginKindIntegration && record.Spec.Runtime == model.PluginRuntimeWASM {
		return true
	}
	return record.Kind == model.PluginKindWorkflow &&
		record.Spec.Runtime == model.PluginRuntimeWASM &&
		record.Spec.Workflow != nil &&
		isExecutableWorkflowProcess(record.Spec.Workflow.Process)
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

func eventSourceForRecord(record *model.PluginRecord) model.PluginEventSource {
	if record == nil {
		return model.PluginEventSourceControlPlane
	}
	return eventSourceFromHost(record.RuntimeHost)
}

func eventSourceFromHost(host model.PluginRuntimeHost) model.PluginEventSource {
	switch host {
	case model.PluginHostTSBridge:
		return model.PluginEventSourceTSBridge
	case model.PluginHostGoOrchestrator:
		return model.PluginEventSourceGoRuntime
	default:
		return model.PluginEventSourceControlPlane
	}
}

func marketplaceFromManifest(manifest model.PluginManifest, record *model.PluginRecord) model.MarketplacePluginDTO {
	item := model.MarketplacePluginDTO{
		ID:          manifest.Metadata.ID,
		Name:        manifest.Metadata.Name,
		Description: manifest.Metadata.Description,
		Version:     manifest.Metadata.Version,
		Kind:        string(manifest.Kind),
		InstallURL:  manifest.Source.Path,
		SourceType:  string(manifest.Source.Type),
		Runtime:     string(manifest.Spec.Runtime),
		Release:     manifest.Source.Release,
	}
	if manifest.Source.Trust != nil {
		item.TrustStatus = manifest.Source.Trust.Status
		item.ApprovalState = manifest.Source.Trust.ApprovalState
	}
	if record != nil {
		item.Installed = true
	}
	return item
}
