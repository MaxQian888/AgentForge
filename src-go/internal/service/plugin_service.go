package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/react-go-quick-starter/server/internal/model"
	pluginparser "github.com/react-go-quick-starter/server/internal/plugin"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/pkg/database"
	"gorm.io/gorm"
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
	// DisableToolPlugin signals the tool-plugin host (TS bridge) to
	// disconnect the MCP transport and release the child process. It is
	// invoked from Disable/Deactivate/Uninstall to prevent orphan
	// processes. Implementations MAY return the resulting runtime
	// status. Nil returns (or unimplemented mocks) are tolerated by the
	// caller so older test doubles keep working.
	DisableToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error)
	CheckToolPluginHealth(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error)
	RestartToolPlugin(ctx context.Context, pluginID string) (*model.PluginRuntimeStatus, error)
	RefreshToolPluginMCPSurface(ctx context.Context, pluginID string) (*model.PluginMCPRefreshResult, error)
	InvokeToolPluginMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error)
	ReadToolPluginMCPResource(ctx context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error)
	GetToolPluginMCPPrompt(ctx context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error)
}

type GoPluginRuntime interface {
	ActivatePlugin(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error)
	// DeactivatePlugin releases any cached resources (compiled WASM
	// modules, runtime handles) for the plugin. Called during
	// Disable/Deactivate/Uninstall so memory doesn't leak across the
	// plugin's lifecycle. Implementations that hold no state MAY return
	// nil with no effect.
	DeactivatePlugin(ctx context.Context, pluginID string) error
	CheckPluginHealth(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error)
	RestartPlugin(ctx context.Context, record model.PluginRecord) (*model.PluginRuntimeStatus, error)
	Invoke(ctx context.Context, record model.PluginRecord, operation string, payload map[string]any) (map[string]any, error)
}

type PluginRoleStore interface {
	Get(id string) (*rolepkg.Manifest, error)
}

// RemoteRegistryClient fetches plugin catalogs and downloads from a remote registry.
type RemoteRegistryClient interface {
	FetchCatalog(ctx context.Context, registryURL string) ([]RemotePluginEntry, error)
	Download(ctx context.Context, pluginID, version, registryURL string) (io.ReadCloser, error)
}

// RemotePluginEntry represents a plugin available in a remote registry.
type RemotePluginEntry struct {
	PluginID    string   `json:"pluginId"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Kind        string   `json:"kind,omitempty"`
	Runtime     string   `json:"runtime,omitempty"`
	Tags        []string `json:"tags"`
}

type RemoteRegistryFailure struct {
	Code       model.RemoteRegistryErrorCode
	Message    string
	StatusCode int
	cause      error
}

func (f *RemoteRegistryFailure) Error() string {
	if f == nil {
		return ""
	}
	return f.Message
}

func (f *RemoteRegistryFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.cause
}

func RemoteRegistryFailureFromError(err error) (*RemoteRegistryFailure, bool) {
	var failure *RemoteRegistryFailure
	if !errors.As(err, &failure) || failure == nil {
		return nil, false
	}
	return failure, true
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

type PluginInstallRequest struct {
	Path   string
	Source *model.PluginSource
}

type pluginWriteStores struct {
	repo             PluginRegistry
	instanceStore    PluginInstanceStore
	eventStore       PluginEventAuditStore
	pendingBroadcast []*model.PluginEventRecord
}

type pluginTransactionalStores struct {
	db            *gorm.DB
	repo          repository.PluginRegistryDBBinder
	instanceStore repository.PluginInstanceDBBinder
	eventStore    repository.PluginEventDBBinder
}

type PluginService struct {
	repo           PluginRegistry
	instanceStore  PluginInstanceStore
	eventStore     PluginEventAuditStore
	broadcaster    PluginEventBroadcaster
	runtimeClient  ToolPluginRuntimeClient
	goRuntime      GoPluginRuntime
	roleStore      PluginRoleStore
	builtInsDir    string
	policy         PluginPolicy
	remoteRegistry RemoteRegistryClient
	registryURL    string
}

func NewPluginService(repo PluginRegistry, runtimeClient ToolPluginRuntimeClient, goRuntime GoPluginRuntime, builtInsDir string) *PluginService {
	instanceStore, eventStore := defaultPluginPersistenceStores(repo)
	return &PluginService{
		repo:          repo,
		instanceStore: instanceStore,
		eventStore:    eventStore,
		runtimeClient: runtimeClient,
		goRuntime:     goRuntime,
		builtInsDir:   builtInsDir,
		policy: PluginPolicy{
			AllowNetwork:    true,
			AllowFilesystem: true,
		},
	}
}

func defaultPluginPersistenceStores(repo PluginRegistry) (PluginInstanceStore, PluginEventAuditStore) {
	if binder, ok := repo.(repository.PluginRegistryDBBinder); ok && binder.DB() != nil {
		db := binder.DB()
		return repository.NewPluginInstanceRepository(db), repository.NewPluginEventRepository(db)
	}
	return repository.NewPluginInstanceRepository(), repository.NewPluginEventRepository()
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

func (s *PluginService) transactionalStores() (*pluginTransactionalStores, bool) {
	repoBinder, ok := s.repo.(repository.PluginRegistryDBBinder)
	if !ok || repoBinder.DB() == nil {
		return nil, false
	}

	txStores := &pluginTransactionalStores{
		db:   repoBinder.DB(),
		repo: repoBinder,
	}

	if s.instanceStore != nil {
		instanceBinder, ok := s.instanceStore.(repository.PluginInstanceDBBinder)
		if !ok {
			return nil, false
		}
		txStores.instanceStore = instanceBinder
	}
	if s.eventStore != nil {
		eventBinder, ok := s.eventStore.(repository.PluginEventDBBinder)
		if !ok {
			return nil, false
		}
		txStores.eventStore = eventBinder
	}
	return txStores, true
}

func (s *PluginService) withWriteStores(ctx context.Context, fn func(stores *pluginWriteStores) error) error {
	if txStores, ok := s.transactionalStores(); ok {
		var stores *pluginWriteStores
		err := database.WithTx(ctx, txStores.db, func(tx *gorm.DB) error {
			stores = &pluginWriteStores{
				repo: txStores.repo.WithDB(tx),
			}
			if txStores.instanceStore != nil {
				stores.instanceStore = txStores.instanceStore.WithDB(tx)
			}
			if txStores.eventStore != nil {
				stores.eventStore = txStores.eventStore.WithDB(tx)
			}
			return fn(stores)
		})
		if err != nil {
			return err
		}
		stores.flushBroadcasts(s.broadcaster)
		return nil
	}

	stores := &pluginWriteStores{
		repo:          s.repo,
		instanceStore: s.instanceStore,
		eventStore:    s.eventStore,
	}
	if err := fn(stores); err != nil {
		return err
	}
	stores.flushBroadcasts(s.broadcaster)
	return nil
}

func (stores *pluginWriteStores) persistRecord(ctx context.Context, record *model.PluginRecord) error {
	if err := stores.repo.Save(ctx, record); err != nil {
		return err
	}
	if stores.instanceStore == nil {
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
	return stores.instanceStore.UpsertCurrent(ctx, snapshot)
}

func (stores *pluginWriteStores) deletePlugin(ctx context.Context, pluginID string) error {
	if stores.instanceStore != nil {
		if err := stores.instanceStore.DeleteByPluginID(ctx, pluginID); err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
	}
	return stores.repo.Delete(ctx, pluginID)
}

func (stores *pluginWriteStores) appendEvent(ctx context.Context, record *model.PluginRecord, eventType model.PluginEventType, source model.PluginEventSource, summary string, payload map[string]any) error {
	if record == nil || stores.eventStore == nil {
		return nil
	}
	event := newPluginEventRecord(record, eventType, source, summary, payload)
	if err := stores.eventStore.Append(ctx, event); err != nil {
		return err
	}
	stores.pendingBroadcast = append(stores.pendingBroadcast, event)
	return nil
}

func (stores *pluginWriteStores) flushBroadcasts(broadcaster PluginEventBroadcaster) {
	if broadcaster == nil {
		return
	}
	for _, event := range stores.pendingBroadcast {
		broadcaster.BroadcastPluginEvent(event)
	}
}

func newPluginEventRecord(record *model.PluginRecord, eventType model.PluginEventType, source model.PluginEventSource, summary string, payload map[string]any) *model.PluginEventRecord {
	return &model.PluginEventRecord{
		PluginID:       record.Metadata.ID,
		EventType:      eventType,
		EventSource:    source,
		LifecycleState: record.LifecycleState,
		Summary:        summary,
		Payload:        payload,
	}
}

func (s *PluginService) persistRecordWithEvent(
	ctx context.Context,
	record *model.PluginRecord,
	eventType model.PluginEventType,
	source model.PluginEventSource,
	summary string,
	payload map[string]any,
) error {
	return s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		return stores.appendEvent(ctx, record, eventType, source, summary, payload)
	})
}

func (s *PluginService) broadcastOnly(record *model.PluginRecord, eventType model.PluginEventType, source model.PluginEventSource, summary string, payload map[string]any) {
	if record == nil || s.broadcaster == nil {
		return
	}
	s.broadcaster.BroadcastPluginEvent(newPluginEventRecord(record, eventType, source, summary, payload))
}

func (s *PluginService) DiscoverBuiltIns(ctx context.Context) ([]*model.PluginRecord, error) {
	records := make([]*model.PluginRecord, 0)
	bundle, err := loadBuiltInBundle(s.builtInsDir)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(s.builtInsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "manifest.yaml" {
			return nil
		}

		manifest, parseErr := pluginparser.ParseFile(path)
		if parseErr != nil {
			return parseErr
		}
		builtInMeta, include, bundleErr := s.resolveBuiltInMetadata(path, manifest, bundle)
		if bundleErr != nil {
			return bundleErr
		}
		if !include {
			return nil
		}
		manifest.Source = normalizePluginSource(
			manifest.Metadata.ID,
			manifest.Source,
			&model.PluginSource{
				Type: model.PluginSourceBuiltin,
				Path: path,
			},
			path,
		)
		if err := s.validateWorkflowManifest(manifest); err != nil {
			return err
		}

		record := buildPluginRecordFromManifest(*manifest)
		record.BuiltIn = builtInMeta
		records = append(records, s.hydrateRecord(ctx, record))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover built-ins: %w", err)
	}
	return records, nil
}

func (s *PluginService) RegisterLocalPath(ctx context.Context, path string) (*model.PluginRecord, error) {
	return s.Install(ctx, PluginInstallRequest{
		Path: path,
		Source: &model.PluginSource{
			Type: model.PluginSourceLocal,
			Path: path,
		},
	})
}

// SyncBuiltIns walks the built-ins directory and upserts a PluginRecord
// for every manifest.yaml it finds, so the control-plane registry
// reflects on-disk state as soon as the server boots — callers no
// longer need to hit GET /api/v1/plugins/discover before the list is
// populated.
//
// Existing records are preserved: if an operator has already installed
// a plugin with the same id, we leave its lifecycle_state alone. New
// manifests land in the "installed" state so activation remains an
// explicit operator action. No runtime (Tool/bridge/WASM) calls are
// made — activation runs later through the normal lifecycle endpoints.
//
// Returns the list of NEW records persisted this pass (empty slice on
// no-op). Errors bubble up; per-manifest parse failures short-circuit.
func (s *PluginService) SyncBuiltIns(ctx context.Context) ([]*model.PluginRecord, error) {
	discovered, err := s.DiscoverBuiltIns(ctx)
	if err != nil {
		return nil, err
	}
	created := make([]*model.PluginRecord, 0, len(discovered))
	for _, record := range discovered {
		if record == nil {
			continue
		}
		if existing, getErr := s.repo.GetByID(ctx, record.Metadata.ID); getErr == nil && existing != nil {
			continue
		}
		fresh := &model.PluginRecord{
			PluginManifest:     record.PluginManifest,
			LifecycleState:     model.PluginStateInstalled,
			RuntimeHost:        record.RuntimeHost,
			RestartCount:       0,
			ResolvedSourcePath: record.ResolvedSourcePath,
			RuntimeMetadata:    record.RuntimeMetadata,
			BuiltIn:            record.BuiltIn,
		}
		if err := s.persistRecordWithEvent(ctx, fresh, model.PluginEventInstalled, model.PluginEventSourceControlPlane, "builtin plugin synced from disk", map[string]any{
			"source_type": string(fresh.Source.Type),
			"source_path": fresh.Source.Path,
			"runtime":     string(fresh.Spec.Runtime),
		}); err != nil {
			return created, fmt.Errorf("sync builtin %s: %w", fresh.Metadata.ID, err)
		}
		created = append(created, s.hydrateRecord(ctx, fresh))
	}
	return created, nil
}

// RegisterFirstPartyInProc upserts a plugin record for a first-party,
// in-proc integration whose implementation ships inside the Go binary
// (runtime: firstparty-inproc). Unlike Install, this method skips all
// external runtime interactions and lands the record directly in the
// active lifecycle state so it mirrors how the feature actually runs:
// wired into the server at boot, with no separate activation step.
//
// Callers are expected to invoke this from their registration hook
// (e.g. installQianchuanPlugin) right after the plugin's routes and
// background loops have been attached, so FE/API consumers see a
// coherent picture of "plugin is active ⇔ feature is enabled".
func (s *PluginService) RegisterFirstPartyInProc(
	ctx context.Context,
	manifestPath string,
) (*model.PluginRecord, error) {
	resolvedPath, err := resolveInstallManifestPath(manifestPath)
	if err != nil {
		return nil, err
	}
	manifest, err := pluginparser.ParseFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	if manifest.Spec.Runtime != model.PluginRuntimeFirstpartyInproc {
		return nil, fmt.Errorf(
			"plugin %s: RegisterFirstPartyInProc requires spec.runtime=firstparty-inproc (got %q)",
			manifest.Metadata.ID, manifest.Spec.Runtime,
		)
	}
	manifest.Source = normalizePluginSource(
		manifest.Metadata.ID,
		manifest.Source,
		&model.PluginSource{Type: model.PluginSourceBuiltin, Path: resolvedPath},
		resolvedPath,
	)

	record := buildPluginRecordFromManifest(*manifest)
	record.LifecycleState = model.PluginStateActive
	record.RuntimeHost = model.PluginHostGoOrchestrator
	record.LastError = ""

	eventType := model.PluginEventActivated
	summary := "first-party in-proc plugin registered"
	if existing, getErr := s.repo.GetByID(ctx, record.Metadata.ID); getErr == nil && existing != nil {
		eventType = model.PluginEventUpdated
		summary = "first-party in-proc plugin re-registered on boot"
	}

	if err := s.persistRecordWithEvent(ctx, record, eventType, model.PluginEventSourceControlPlane, summary, map[string]any{
		"source_type": manifest.Source.Type,
		"source_path": resolvedPath,
		"runtime":     string(manifest.Spec.Runtime),
	}); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) Install(ctx context.Context, req PluginInstallRequest) (*model.PluginRecord, error) {
	resolvedPath, err := resolveInstallManifestPath(req.Path)
	if err != nil {
		return nil, err
	}

	manifest, err := pluginparser.ParseFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	manifest.Source = normalizePluginSource(manifest.Metadata.ID, manifest.Source, req.Source, resolvedPath)
	if err := s.validateWorkflowManifest(manifest); err != nil {
		return nil, err
	}

	record := buildPluginRecordFromManifest(*manifest)

	if manifest.Kind == model.PluginKindTool && s.runtimeClient != nil {
		status, err := s.runtimeClient.RegisterToolPlugin(ctx, *manifest)
		if err != nil {
			return nil, fmt.Errorf("register tool plugin in bridge: %w", err)
		}
		applyRuntimeStatus(record, *status)
	}

	eventType := model.PluginEventInstalled
	summary := "plugin installed"
	if existing, getErr := s.repo.GetByID(ctx, record.Metadata.ID); getErr == nil && existing != nil {
		eventType = model.PluginEventUpdated
		summary = "plugin updated"
	}

	if err := s.persistRecordWithEvent(ctx, record, eventType, model.PluginEventSourceControlPlane, summary, map[string]any{
		"source_type": manifest.Source.Type,
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

func (s *PluginService) GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) SearchCatalog(ctx context.Context, query string) ([]model.MarketplacePluginDTO, error) {
	installed, err := s.repo.List(ctx, model.PluginFilter{})
	if err != nil {
		return nil, err
	}
	installedByID := make(map[string]*model.PluginRecord, len(installed))
	for _, record := range installed {
		installedByID[record.Metadata.ID] = record
	}

	catalogEntries := make([]model.MarketplacePluginDTO, 0)
	needle := strings.ToLower(strings.TrimSpace(query))
	bundle, err := loadBuiltInBundle(s.builtInsDir)
	if err != nil {
		return nil, err
	}
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
		builtInMeta, include, bundleErr := s.resolveBuiltInMetadata(path, manifest, bundle)
		if bundleErr != nil {
			return bundleErr
		}
		if !include {
			return nil
		}
		manifest.Source = normalizePluginSource(manifest.Metadata.ID, manifest.Source, nil, path)
		if needle != "" && !catalogMatchesQuery(*manifest, needle) {
			return nil
		}
		catalogEntries = append(catalogEntries, marketplaceFromManifest(*manifest, installedByID[manifest.Metadata.ID], builtInMeta))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search plugin catalog: %w", err)
	}

	return catalogEntries, nil
}

func (s *PluginService) InstallCatalogEntry(ctx context.Context, entryID string) (*model.PluginRecord, error) {
	manifestPath, manifest, err := s.findCatalogManifest(entryID)
	if err != nil {
		return nil, err
	}
	source := manifest.Source
	if source.Type == "" {
		source.Type = model.PluginSourceCatalog
	}
	if source.Entry == "" {
		source.Entry = entryID
	}
	if source.Path == "" {
		source.Path = manifestPath
	}
	return s.Install(ctx, PluginInstallRequest{
		Path:   manifestPath,
		Source: &source,
	})
}

func (s *PluginService) Enable(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if err := validateExternalTrust(record); err != nil {
		return nil, err
	}
	if err := FirstMissingWorkflowRoleError(record, s.roleStore); err != nil {
		return nil, err
	}
	record.LifecycleState = model.PluginStateEnabled
	record.LastError = ""
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventEnabled, model.PluginEventSourceOperator, "plugin enabled", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) Disable(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	s.tearDownRuntime(ctx, record, "disable")
	record.LifecycleState = model.PluginStateDisabled
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventDisabled, model.PluginEventSourceOperator, "plugin disabled", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) Deactivate(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	s.tearDownRuntime(ctx, record, "deactivate")
	record.LifecycleState = model.PluginStateEnabled
	record.LastError = ""
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventDeactivated, model.PluginEventSourceOperator, "plugin deactivated", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

// tearDownRuntime signals the owning runtime to release whatever
// resources (MCP child processes, WASM module instances, in-proc
// goroutines) the plugin is currently holding. It is safe to call on
// any record — no-op for firstparty-inproc / declarative / inactive
// plugins. Failures are logged and swallowed so administrative state
// changes (disable/deactivate/uninstall) never hang on a sick runtime.
func (s *PluginService) tearDownRuntime(ctx context.Context, record *model.PluginRecord, op string) {
	if record == nil {
		return
	}
	switch record.Kind {
	case model.PluginKindTool:
		if s.runtimeClient == nil {
			return
		}
		if _, err := s.runtimeClient.DisableToolPlugin(ctx, record.Metadata.ID); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"plugin_id": record.Metadata.ID,
				"operation": op,
			}).Warn("tool plugin runtime teardown failed")
		}
	case model.PluginKindIntegration, model.PluginKindWorkflow:
		// WASM-backed plugins cache a compiled module + runtime between
		// invocations; release it so memory doesn't leak past the
		// lifecycle transition. firstparty-inproc holds no cached
		// resource — the GoPluginRuntime implementation should noop.
		if s.goRuntime == nil {
			return
		}
		if err := s.goRuntime.DeactivatePlugin(ctx, record.Metadata.ID); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"plugin_id": record.Metadata.ID,
				"operation": op,
			}).Warn("go plugin runtime teardown failed")
		}
	}
}

func (s *PluginService) Activate(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.LifecycleState == model.PluginStateDisabled {
		return nil, fmt.Errorf("plugin %s is disabled", pluginID)
	}
	if err := validateExternalTrust(record); err != nil {
		return nil, err
	}
	if err := FirstMissingWorkflowRoleError(record, s.roleStore); err != nil {
		record.LastError = err.Error()
		record.LifecycleState = model.PluginStateDegraded
		_ = s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceControlPlane, err.Error(), map[string]any{"operation": "activate"})
		return nil, err
	}
	if err := s.validateActivationPermissions(record); err != nil {
		record.LastError = err.Error()
		record.LifecycleState = model.PluginStateDegraded
		_ = s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceControlPlane, err.Error(), map[string]any{"operation": "activate"})
		return nil, err
	}

	record.LifecycleState = model.PluginStateActivating
	record.LastError = ""
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventActivating, eventSourceForRecord(record), "plugin activating", nil); err != nil {
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

	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventActivated, eventSourceForRecord(record), "plugin activated", nil); err != nil {
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
		if persistErr := s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, eventSourceForRecord(record), err.Error(), map[string]any{"operation": "health"}); persistErr != nil {
			return nil, persistErr
		}
		return nil, err
	}

	applyRuntimeStatus(record, *status)
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventHealth, eventSourceForRecord(record), "plugin health updated", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

// ReconcileHealth walks every active or degraded plugin once and updates
// its lifecycle_state based on a runtime probe. Active plugins whose
// MCP child died transition to "degraded"; degraded plugins that have
// recovered transition back to "active" via CheckHealth's normal
// applyRuntimeStatus path. Per-plugin failures are absorbed (already
// recorded as PluginEventFailed) so one sick plugin doesn't stall the
// sweep.
//
// This is the single-shot primitive that RunHealthLoop drives on a
// ticker; exposing it separately lets tests drive the reconciler
// deterministically without sleeping.
func (s *PluginService) ReconcileHealth(ctx context.Context) {
	targets, err := s.repo.List(ctx, model.PluginFilter{LifecycleState: model.PluginStateActive})
	if err != nil {
		log.WithError(err).Warn("plugin health reconcile: list active failed")
		return
	}
	degraded, err := s.repo.List(ctx, model.PluginFilter{LifecycleState: model.PluginStateDegraded})
	if err != nil {
		log.WithError(err).Warn("plugin health reconcile: list degraded failed")
	} else {
		targets = append(targets, degraded...)
	}

	for _, record := range targets {
		if record == nil {
			continue
		}
		// Skip kinds the reconciler can't probe. firstparty-inproc has no
		// out-of-proc runtime handle, so its state is authoritative as-is.
		if record.Kind == model.PluginKindTool && s.runtimeClient == nil {
			continue
		}
		if isGoHostedHealthPlugin(record) && s.goRuntime == nil {
			continue
		}
		if record.Spec.Runtime == model.PluginRuntimeFirstpartyInproc ||
			record.Spec.Runtime == model.PluginRuntimeDeclarative {
			continue
		}
		if _, err := s.CheckHealth(ctx, record.Metadata.ID); err != nil {
			// CheckHealth already persisted the failure; nothing to do.
			continue
		}
	}
}

// RunHealthLoop starts the background heartbeat ticker. Blocks until
// ctx is canceled. Safe to call as `go svc.RunHealthLoop(ctx, 60*time.Second)`.
// Intervals <= 0 fall back to one minute so misconfiguration doesn't
// turn the loop into a hot spin.
func (s *PluginService) RunHealthLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.WithField("interval", interval).Info("plugin health heartbeat started")
	for {
		select {
		case <-ctx.Done():
			log.Info("plugin health heartbeat stopped")
			return
		case <-ticker.C:
			s.ReconcileHealth(ctx)
		}
	}
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
		if persistErr := s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, eventSourceForRecord(record), err.Error(), map[string]any{"operation": "restart"}); persistErr != nil {
			return nil, persistErr
		}
		return nil, err
	}

	applyRuntimeStatus(record, *status)
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventRestarted, eventSourceForRecord(record), "plugin restarted", nil); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, record), nil
}

func (s *PluginService) RefreshMCP(ctx context.Context, pluginID string) (*model.PluginMCPRefreshResult, error) {
	record, err := s.requireActiveToolPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}

	result, err := s.runtimeClient.RefreshToolPluginMCPSurface(ctx, pluginID)
	if err != nil {
		s.recordMCPFailure(ctx, record, model.MCPInteractionRefresh, pluginID, "bridge_request_failed", err)
		return nil, err
	}

	s.applyMCPRefresh(record, result)
	if err := s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		return appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPDiscovery, model.MCPInteractionRefresh, model.MCPInteractionSucceeded, pluginID, s.summarizeRefreshResult(result), "", "")
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PluginService) CallMCPTool(ctx context.Context, pluginID, toolName string, args map[string]any) (*model.PluginMCPToolCallResult, error) {
	record, err := s.requireActiveToolPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(toolName) == "" {
		err := fmt.Errorf("tool_name is required")
		s.recordMCPFailure(ctx, record, model.MCPInteractionCallTool, toolName, "validation_failed", err)
		return nil, err
	}

	result, err := s.runtimeClient.InvokeToolPluginMCPTool(ctx, pluginID, toolName, args)
	if err != nil {
		s.recordMCPFailure(ctx, record, model.MCPInteractionCallTool, toolName, "bridge_request_failed", err)
		return nil, err
	}

	s.applyMCPLatestInteraction(record, model.MCPInteractionCallTool, model.MCPInteractionSucceeded, toolName, s.summarizeToolCallResult(result), "", "")
	if err := s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		return appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPInteraction, model.MCPInteractionCallTool, model.MCPInteractionSucceeded, toolName, s.summarizeToolCallResult(result), "", "")
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PluginService) ReadMCPResource(ctx context.Context, pluginID, uri string) (*model.PluginMCPResourceReadResult, error) {
	record, err := s.requireActiveToolPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(uri) == "" {
		err := fmt.Errorf("uri is required")
		s.recordMCPFailure(ctx, record, model.MCPInteractionReadResource, uri, "validation_failed", err)
		return nil, err
	}

	result, err := s.runtimeClient.ReadToolPluginMCPResource(ctx, pluginID, uri)
	if err != nil {
		s.recordMCPFailure(ctx, record, model.MCPInteractionReadResource, uri, "bridge_request_failed", err)
		return nil, err
	}

	s.applyMCPLatestInteraction(record, model.MCPInteractionReadResource, model.MCPInteractionSucceeded, uri, s.summarizeResourceReadResult(result), "", "")
	if err := s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		return appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPInteraction, model.MCPInteractionReadResource, model.MCPInteractionSucceeded, uri, s.summarizeResourceReadResult(result), "", "")
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PluginService) GetMCPPrompt(ctx context.Context, pluginID, name string, args map[string]string) (*model.PluginMCPPromptResult, error) {
	record, err := s.requireActiveToolPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		err := fmt.Errorf("name is required")
		s.recordMCPFailure(ctx, record, model.MCPInteractionGetPrompt, name, "validation_failed", err)
		return nil, err
	}

	result, err := s.runtimeClient.GetToolPluginMCPPrompt(ctx, pluginID, name, args)
	if err != nil {
		s.recordMCPFailure(ctx, record, model.MCPInteractionGetPrompt, name, "bridge_request_failed", err)
		return nil, err
	}

	s.applyMCPLatestInteraction(record, model.MCPInteractionGetPrompt, model.MCPInteractionSucceeded, name, s.summarizePromptResult(result), "", "")
	if err := s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		return appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPInteraction, model.MCPInteractionGetPrompt, model.MCPInteractionSucceeded, name, s.summarizePromptResult(result), "", "")
	}); err != nil {
		return nil, err
	}
	return result, nil
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
	if err := s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		if status.RuntimeMetadata != nil && status.RuntimeMetadata.MCP != nil {
			if interaction := status.RuntimeMetadata.MCP.LatestInteraction; interaction != nil {
				if err := appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPInteraction, interaction.Operation, interaction.Status, interaction.Target, interaction.Summary, interaction.ErrorCode, interaction.ErrorMessage); err != nil {
					return err
				}
			} else if status.RuntimeMetadata.MCP.LastDiscoveryAt != nil || status.RuntimeMetadata.MCP.ToolCount > 0 || status.RuntimeMetadata.MCP.ResourceCount > 0 || status.RuntimeMetadata.MCP.PromptCount > 0 {
				if err := appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPDiscovery, model.MCPInteractionRefresh, model.MCPInteractionSucceeded, pluginID, s.summarizeMCPMetadata(status.RuntimeMetadata.MCP), "", ""); err != nil {
					return err
				}
			}
		}
		return stores.appendEvent(ctx, record, model.PluginEventRuntimeSync, eventSourceFromHost(status.Host), "runtime state synchronized", nil)
	}); err != nil {
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
		if persistErr := s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceControlPlane, err.Error(), map[string]any{"operation": operation}); persistErr != nil {
			return nil, persistErr
		}
		return nil, err
	}

	result, err := s.goRuntime.Invoke(ctx, *record, operation, payload)
	if err != nil {
		record.LifecycleState = model.PluginStateDegraded
		record.LastError = err.Error()
		if persistErr := s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, model.PluginEventSourceGoRuntime, err.Error(), map[string]any{"operation": operation}); persistErr != nil {
			return nil, persistErr
		}
		return nil, err
	}
	record.LastError = ""
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventInvoked, model.PluginEventSourceGoRuntime, "plugin invoked", map[string]any{"operation": operation}); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PluginService) Update(ctx context.Context, pluginID string, req PluginInstallRequest) (*model.PluginRecord, error) {
	current, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}

	updated, err := s.Install(ctx, req)
	if err != nil {
		return nil, err
	}
	if updated.Metadata.ID != pluginID {
		return nil, fmt.Errorf("plugin identity mismatch during update: expected %s, got %s", pluginID, updated.Metadata.ID)
	}

	if err := s.appendEvent(ctx, updated, model.PluginEventUpdated, model.PluginEventSourceOperator, "plugin updated", map[string]any{
		"previous_version": current.Metadata.Version,
		"previous_digest":  current.Source.Digest,
		"current_version":  updated.Metadata.Version,
		"current_digest":   updated.Source.Digest,
	}); err != nil {
		return nil, err
	}
	return s.hydrateRecord(ctx, updated), nil
}

func (s *PluginService) Uninstall(ctx context.Context, id string) error {
	return s.UninstallOpts(ctx, id, UninstallOptions{})
}

// UninstallOptions tunes Uninstall's behaviour. Purge=true also removes
// the plugin's on-disk manifest directory so a subsequent SyncBuiltIns
// sweep doesn't re-register it. Use with care — the directory delete
// is recursive and is only executed for manifests that live beneath
// the plugin-service's builtInsDir, so an attacker-controlled path
// can't escape the plugins tree.
type UninstallOptions struct {
	Purge bool
}

func (s *PluginService) UninstallOpts(ctx context.Context, id string, opts UninstallOptions) error {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}
	if rec.LifecycleState == model.PluginStateActive {
		if _, err := s.Disable(ctx, id); err != nil {
			return err
		}
	}
	s.tearDownRuntime(ctx, rec, "uninstall")

	if err := s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.deletePlugin(ctx, id); err != nil {
			return fmt.Errorf("delete plugin: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	purged := false
	if opts.Purge {
		if err := s.purgeManifestDir(rec); err != nil {
			log.WithError(err).WithField("plugin_id", id).Warn("plugin uninstall: manifest purge failed")
		} else {
			purged = true
		}
	}

	payload := map[string]any{"purged": purged}
	s.broadcastOnly(rec, model.PluginEventUninstalled, model.PluginEventSourceOperator, "plugin uninstalled", payload)
	return nil
}

// purgeManifestDir removes the directory containing the plugin's
// manifest.yaml, but ONLY if the path lives beneath s.builtInsDir or
// the resolved source path. Refuses to touch paths outside the
// plugins root to prevent directory-traversal abuses when the source
// path is operator-supplied.
func (s *PluginService) purgeManifestDir(rec *model.PluginRecord) error {
	manifestPath := strings.TrimSpace(rec.Source.Path)
	if manifestPath == "" {
		manifestPath = strings.TrimSpace(rec.ResolvedSourcePath)
	}
	if manifestPath == "" {
		return fmt.Errorf("no manifest path on record")
	}
	absManifest, err := filepath.Abs(manifestPath)
	if err != nil {
		return fmt.Errorf("resolve manifest path: %w", err)
	}
	dir := filepath.Dir(absManifest)

	builtInsDir := strings.TrimSpace(s.builtInsDir)
	if builtInsDir != "" {
		absRoot, err := filepath.Abs(builtInsDir)
		if err != nil {
			return fmt.Errorf("resolve plugins root: %w", err)
		}
		rel, err := filepath.Rel(absRoot, dir)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			return fmt.Errorf("manifest dir %s is not under plugins root %s — refusing purge", dir, absRoot)
		}
	}
	return os.RemoveAll(dir)
}

func (s *PluginService) UpdateConfig(ctx context.Context, id string, config map[string]interface{}) (*model.PluginRecord, error) {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}
	// If the manifest declared a config schema, validate the incoming
	// config against it before persisting. Empty/nil schema accepts
	// anything — this is a strictly additive safety net.
	if rec.Spec.ConfigSchema != nil {
		if err := pluginparser.ValidateConfig(rec.Spec.ConfigSchema, config); err != nil {
			return nil, fmt.Errorf("config schema violation: %w", err)
		}
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
	bundle, err := loadBuiltInBundle(s.builtInsDir)
	if err != nil {
		return nil, err
	}
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
		builtInMeta, include, bundleErr := s.resolveBuiltInMetadata(path, manifest, bundle)
		if bundleErr != nil {
			return bundleErr
		}
		if !include {
			return nil
		}
		manifest.Source = model.PluginSource{
			Type: model.PluginSourceBuiltin,
			Path: path,
		}
		catalog[manifest.Metadata.ID] = marketplaceFromManifest(*manifest, installedByID[manifest.Metadata.ID], builtInMeta)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list plugin catalog: %w", err)
	}

	for _, record := range installed {
		if _, ok := catalog[record.Metadata.ID]; ok {
			continue
		}
		catalog[record.Metadata.ID] = marketplaceFromManifest(record.PluginManifest, record, s.hydrateRecord(ctx, record).BuiltIn)
	}

	items := make([]model.MarketplacePluginDTO, 0, len(catalog))
	for _, item := range catalog {
		items = append(items, item)
	}
	return items, nil
}

// SetRemoteRegistry configures the remote registry client and URL.
func (s *PluginService) SetRemoteRegistry(client RemoteRegistryClient, registryURL ...string) {
	s.remoteRegistry = client
	if len(registryURL) > 0 {
		s.registryURL = registryURL[0]
	}
}

func (s *PluginService) RegistryURL() string {
	return s.registryURL
}

// ListRemotePlugins fetches the catalog from the configured remote registry.
func (s *PluginService) ListRemotePlugins(ctx context.Context) (*model.RemoteMarketplaceResponse, error) {
	response := &model.RemoteMarketplaceResponse{
		Registry: s.registryURL,
		Entries:  []model.MarketplacePluginDTO{},
	}
	if s.remoteRegistry == nil || strings.TrimSpace(s.registryURL) == "" {
		response.ErrorCode = model.RemoteRegistryUnconfigured
		response.Error = "Remote plugin registry is not configured."
		return response, nil
	}

	installed, err := s.repo.List(ctx, model.PluginFilter{})
	if err != nil {
		return nil, err
	}
	installedByID := make(map[string]*model.PluginRecord, len(installed))
	for _, record := range installed {
		installedByID[record.Metadata.ID] = record
	}

	entries, err := s.remoteRegistry.FetchCatalog(ctx, s.registryURL)
	if err != nil {
		response.ErrorCode = model.RemoteRegistryUnavailable
		response.Error = "Remote plugin registry is unavailable."
		return response, nil
	}

	response.Available = true
	response.Entries = make([]model.MarketplacePluginDTO, 0, len(entries))
	for _, entry := range entries {
		response.Entries = append(response.Entries, marketplaceFromRemoteEntry(entry, s.registryURL, installedByID[entry.PluginID]))
	}
	return response, nil
}

// InstallFromRemote downloads a plugin from the remote registry and installs it locally.
func (s *PluginService) InstallFromRemote(ctx context.Context, pluginID, version string) error {
	if s.remoteRegistry == nil || strings.TrimSpace(s.registryURL) == "" {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryUnconfigured,
			Message:    "Remote plugin registry is not configured.",
			StatusCode: 503,
		}
	}

	reader, err := s.remoteRegistry.Download(ctx, pluginID, version, s.registryURL)
	if err != nil {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryDownloadFailed,
			Message:    fmt.Sprintf("Failed to download remote plugin %s@%s.", pluginID, version),
			StatusCode: 502,
			cause:      err,
		}
	}
	defer reader.Close()

	// Write to a temporary directory and install from there.
	tmpDir, err := os.MkdirTemp("", "agentforge-remote-plugin-*")
	if err != nil {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryInvalidArtifact,
			Message:    "Failed to prepare the downloaded remote plugin artifact.",
			StatusCode: 400,
			cause:      err,
		}
	}
	defer os.RemoveAll(tmpDir)

	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	outFile, err := os.Create(manifestPath)
	if err != nil {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryInvalidArtifact,
			Message:    "Failed to prepare the downloaded remote plugin artifact.",
			StatusCode: 400,
			cause:      err,
		}
	}

	if _, err := io.Copy(outFile, reader); err != nil {
		outFile.Close()
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryInvalidArtifact,
			Message:    "Failed to materialize the downloaded remote plugin manifest.",
			StatusCode: 400,
			cause:      err,
		}
	}
	outFile.Close()

	manifest, err := pluginparser.ParseFile(manifestPath)
	if err != nil {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryInvalidArtifact,
			Message:    "Downloaded remote plugin artifact is not a valid manifest.",
			StatusCode: 400,
			cause:      err,
		}
	}
	source := normalizePluginSource(manifest.Metadata.ID, manifest.Source, &model.PluginSource{
		Type:     model.PluginSourceCatalog,
		Registry: s.registryURL,
		Entry:    pluginID,
		Version:  version,
	}, manifestPath)
	record := buildPluginRecordFromManifest(*manifest)
	record.Source = source
	if err := validateExternalTrust(record); err != nil {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryVerificationFailed,
			Message:    "Remote plugin artifact failed trust verification or approval checks.",
			StatusCode: 409,
			cause:      err,
		}
	}

	_, err = s.Install(ctx, PluginInstallRequest{
		Path:   manifestPath,
		Source: &source,
	})
	if err != nil {
		return &RemoteRegistryFailure{
			Code:       model.RemoteRegistryInvalidArtifact,
			Message:    "Downloaded remote plugin artifact failed installation validation.",
			StatusCode: 400,
			cause:      err,
		}
	}
	return nil
}

func (s *PluginService) findCatalogManifest(entryID string) (string, *model.PluginManifest, error) {
	bundle, err := loadBuiltInBundle(s.builtInsDir)
	if err != nil {
		return "", nil, err
	}
	var (
		foundPath     string
		foundManifest *model.PluginManifest
	)

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
		_, include, bundleErr := s.resolveBuiltInMetadata(path, manifest, bundle)
		if bundleErr != nil {
			return bundleErr
		}
		if !include {
			return nil
		}
		if manifest.Metadata.ID != entryID {
			return nil
		}
		foundPath = path
		foundManifest = manifest
		return fs.SkipAll
	})
	if err != nil && !errors.Is(err, fs.SkipAll) {
		return "", nil, fmt.Errorf("search plugin catalog: %w", err)
	}
	if foundManifest == nil {
		return "", nil, repository.ErrNotFound
	}
	return foundPath, foundManifest, nil
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
	if err := s.persistRecordWithEvent(ctx, record, model.PluginEventFailed, eventSourceForRecord(record), cause.Error(), map[string]any{"operation": "activate"}); err != nil {
		return nil, err
	}
	return nil, cause
}

func (s *PluginService) persistRecord(ctx context.Context, record *model.PluginRecord) error {
	return s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		return stores.persistRecord(ctx, record)
	})
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
	if bundle, err := loadBuiltInBundle(s.builtInsDir); err == nil {
		if metadata, include, metaErr := s.resolveBuiltInMetadata(cloned.Source.Path, &cloned.PluginManifest, bundle); metaErr == nil && include && metadata != nil {
			cloned.BuiltIn = metadata
		}
	}
	cloned.RoleDependencies = BuildPluginRoleDependencies(&cloned, s.roleStore)
	if roles, err := ListDependencyRoles(s.roleStore); err == nil {
		cloned.RoleConsumers = BuildPluginRoleConsumers(&cloned, roles)
	}
	return &cloned
}

func (s *PluginService) appendEvent(ctx context.Context, record *model.PluginRecord, eventType model.PluginEventType, source model.PluginEventSource, summary string, payload map[string]any) error {
	return s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		return stores.appendEvent(ctx, record, eventType, source, summary, payload)
	})
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

		// Validate `workflow` action target-kind discriminator shape
		// (bridge-legacy-to-dag-invocation). A workflow step may reference
		// either a plugin id (legacy default) or a DAG workflow UUID; mixing
		// the two shapes is rejected here so manifest authors get an early
		// error rather than a runtime dispatch failure.
		if step.Action == model.WorkflowActionWorkflow {
			if err := validateWorkflowStepTarget(stepID, step.Config); err != nil {
				return err
			}
		}
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
	for _, trigger := range workflow.Triggers {
		if strings.TrimSpace(trigger.Event) == "task.transition" && strings.TrimSpace(trigger.Profile) == "" {
			return fmt.Errorf("workflow task.transition trigger must declare a profile")
		}
	}
	return nil
}

// validateWorkflowStepTarget inspects a `workflow` action step's config for
// the engine-aware discriminator introduced by bridge-legacy-to-dag-invocation
// and rejects conflicting or malformed shapes. Accepted inputs:
//   - pluginId / plugin_id (legacy; implicit targetKind='plugin').
//   - targetKind='plugin' paired with pluginId / plugin_id.
//   - targetKind='dag' paired with targetWorkflowId / target_workflow_id (UUID).
//
// Any other combination — unknown targetKind, mixed shapes, missing target —
// returns a structured error naming the offending step.
func validateWorkflowStepTarget(stepID string, config map[string]any) error {
	if config == nil {
		return nil
	}
	pluginID := firstNonEmptyString(config, "pluginId", "plugin_id")
	targetWorkflowID := firstNonEmptyString(config, "targetWorkflowId", "target_workflow_id")
	rawKind := firstNonEmptyString(config, "targetKind", "target_kind")
	kind := strings.ToLower(strings.TrimSpace(rawKind))

	switch kind {
	case "":
		// Legacy shape: must declare a plugin id, must not declare a DAG target.
		if targetWorkflowID != "" && pluginID == "" {
			return fmt.Errorf("workflow step %s sets targetWorkflowId but omits targetKind; declare targetKind='dag' to invoke a DAG child", stepID)
		}
	case "plugin":
		if targetWorkflowID != "" {
			return fmt.Errorf("workflow step %s declares targetKind='plugin' but also sets targetWorkflowId; plugin targets must use pluginId", stepID)
		}
	case "dag":
		if pluginID != "" {
			return fmt.Errorf("workflow step %s declares targetKind='dag' alongside pluginId; a DAG target must not set pluginId", stepID)
		}
		if targetWorkflowID == "" {
			return fmt.Errorf("workflow step %s declares targetKind='dag' but omits targetWorkflowId", stepID)
		}
		if _, err := uuid.Parse(targetWorkflowID); err != nil {
			return fmt.Errorf("workflow step %s targetWorkflowId %q is not a valid UUID: %w", stepID, targetWorkflowID, err)
		}
	default:
		return fmt.Errorf("workflow step %s declares unknown targetKind %q (expected \"plugin\" or \"dag\")", stepID, kind)
	}
	return nil
}

// firstNonEmptyString scans a config map for the first key in keys that maps
// to a non-empty string value. Used to accept both camelCase and snake_case
// spellings of discriminator keys.
func firstNonEmptyString(config map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := config[k]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func isSupportedWorkflowAction(action model.WorkflowActionType) bool {
	switch action {
	case model.WorkflowActionAgent, model.WorkflowActionReview, model.WorkflowActionTask, model.WorkflowActionWorkflow, model.WorkflowActionApproval:
		return true
	default:
		return false
	}
}

func isExecutableWorkflowProcess(process model.WorkflowProcessMode) bool {
	switch process {
	case model.WorkflowProcessSequential,
		model.WorkflowProcessHierarchical,
		model.WorkflowProcessEventDriven,
		model.WorkflowProcessWave:
		return true
	default:
		return false
	}
}

func unsupportedWorkflowProcessError(record *model.PluginRecord) error {
	if record == nil || record.Spec.Workflow == nil {
		return fmt.Errorf("unsupported workflow process")
	}
	return fmt.Errorf(
		"unsupported workflow process %q for plugin %s; supported modes: sequential, hierarchical, event-driven, wave",
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

func buildPluginRecordFromManifest(manifest model.PluginManifest) *model.PluginRecord {
	return &model.PluginRecord{
		PluginManifest:     manifest,
		LifecycleState:     model.PluginStateInstalled,
		RuntimeHost:        resolveRuntimeHost(manifest.Kind),
		RestartCount:       0,
		LastHealthAt:       nil,
		LastError:          "",
		ResolvedSourcePath: resolveSourcePath(manifest),
		RuntimeMetadata:    initialRuntimeMetadata(manifest),
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
		record.RuntimeMetadata = clonePluginRuntimeMetadata(status.RuntimeMetadata)
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

func resolveInstallManifestPath(pathValue string) (string, error) {
	if strings.TrimSpace(pathValue) == "" {
		return "", fmt.Errorf("plugin manifest path is required")
	}
	resolvedPath := pathValue
	if info, statErr := os.Stat(pathValue); statErr == nil && info.IsDir() {
		resolvedPath = filepath.Join(pathValue, "manifest.yaml")
	}
	return resolvedPath, nil
}

func normalizePluginSource(pluginID string, existing model.PluginSource, override *model.PluginSource, manifestPath string) model.PluginSource {
	source := existing
	if override != nil {
		if override.Type != "" {
			source.Type = override.Type
		}
		if override.Path != "" {
			source.Path = override.Path
		}
		if override.Repository != "" {
			source.Repository = override.Repository
		}
		if override.Ref != "" {
			source.Ref = override.Ref
		}
		if override.Package != "" {
			source.Package = override.Package
		}
		if override.Version != "" {
			source.Version = override.Version
		}
		if override.Registry != "" {
			source.Registry = override.Registry
		}
		if override.Catalog != "" {
			source.Catalog = override.Catalog
		}
		if override.Entry != "" {
			source.Entry = override.Entry
		}
		if override.Digest != "" {
			source.Digest = override.Digest
		}
		if override.Signature != "" {
			source.Signature = override.Signature
		}
		if override.Trust != nil {
			trust := *override.Trust
			source.Trust = &trust
		}
		if override.Release != nil {
			release := *override.Release
			source.Release = &release
		}
	}
	if source.Path == "" {
		source.Path = manifestPath
	}
	if source.Type == "" {
		source.Type = model.PluginSourceLocal
	}
	if source.Type == model.PluginSourceCatalog && source.Entry == "" {
		source.Entry = pluginID
	}
	if isExternalSource(source.Type) {
		source.Trust = normalizeExternalTrust(source)
	}
	return source
}

func normalizeExternalTrust(source model.PluginSource) *model.PluginTrustMetadata {
	trust := &model.PluginTrustMetadata{}
	if source.Trust != nil {
		copied := *source.Trust
		trust = &copied
	}

	if trust.ApprovalState == model.PluginApprovalRejected {
		trust.Status = model.PluginTrustUntrusted
		if trust.Reason == "" {
			trust.Reason = "plugin approval was rejected"
		}
		return trust
	}

	if source.Digest != "" && (source.Signature != "" || trust.ApprovalState == model.PluginApprovalApproved) {
		now := time.Now().UTC()
		trust.Status = model.PluginTrustVerified
		if trust.VerifiedAt == nil {
			trust.VerifiedAt = &now
		}
		if trust.ApprovalState == "" {
			if source.Signature != "" {
				trust.ApprovalState = model.PluginApprovalNotRequired
			} else {
				trust.ApprovalState = model.PluginApprovalApproved
			}
		}
		if trust.Source == "" {
			if source.Signature != "" {
				trust.Source = "signature"
			} else {
				trust.Source = "operator-approval"
			}
		}
		return trust
	}

	trust.Status = model.PluginTrustUntrusted
	if trust.ApprovalState == "" {
		trust.ApprovalState = model.PluginApprovalPending
	}
	if trust.Reason == "" {
		trust.Reason = "external plugins require digest plus signature or approved trust metadata before enablement"
	}
	return trust
}

func validateExternalTrust(record *model.PluginRecord) error {
	if record == nil || !isExternalSource(record.Source.Type) {
		return nil
	}
	if record.Source.Trust != nil && record.Source.Trust.Status == model.PluginTrustVerified {
		return nil
	}
	return fmt.Errorf("plugin %s is untrusted and cannot be enabled until digest verification and signature or operator approval succeeds", record.Metadata.ID)
}

func isExternalSource(sourceType model.PluginSourceType) bool {
	switch sourceType {
	case model.PluginSourceGit, model.PluginSourceNPM, model.PluginSourceCatalog:
		return true
	default:
		return false
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

func (s *PluginService) requireActiveToolPlugin(ctx context.Context, pluginID string) (*model.PluginRecord, error) {
	record, err := s.repo.GetByID(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	if record.Kind != model.PluginKindTool {
		return nil, fmt.Errorf("plugin %s does not expose MCP interaction primitives", pluginID)
	}
	if s.runtimeClient == nil {
		return nil, fmt.Errorf("tool runtime client is not configured")
	}
	if record.LifecycleState != model.PluginStateActive {
		return nil, fmt.Errorf("plugin %s must be active before MCP interactions", pluginID)
	}
	if record.RuntimeHost != "" && record.RuntimeHost != model.PluginHostTSBridge {
		return nil, fmt.Errorf("plugin %s is not hosted by the TS bridge", pluginID)
	}
	return record, nil
}

func (s *PluginService) applyMCPRefresh(record *model.PluginRecord, result *model.PluginMCPRefreshResult) {
	if record == nil || result == nil {
		return
	}
	if result.LifecycleState != "" {
		record.LifecycleState = result.LifecycleState
	}
	if result.RuntimeHost != "" {
		record.RuntimeHost = result.RuntimeHost
	}
	if result.RuntimeMetadata != nil {
		record.RuntimeMetadata = clonePluginRuntimeMetadata(result.RuntimeMetadata)
	}
	metadata := s.ensureMCPRuntimeMetadata(record)
	metadata.Transport = result.Snapshot.Transport
	metadata.LastDiscoveryAt = cloneTimePointer(result.Snapshot.LastDiscoveryAt)
	metadata.ToolCount = result.Snapshot.ToolCount
	metadata.ResourceCount = result.Snapshot.ResourceCount
	metadata.PromptCount = result.Snapshot.PromptCount
	metadata.LatestInteraction = cloneMCPInteractionSummary(result.Snapshot.LatestInteraction)
}

func (s *PluginService) ensureMCPRuntimeMetadata(record *model.PluginRecord) *model.PluginMCPRuntimeMetadata {
	if record.RuntimeMetadata == nil {
		record.RuntimeMetadata = &model.PluginRuntimeMetadata{Compatible: true}
	}
	if record.RuntimeMetadata.MCP == nil {
		record.RuntimeMetadata.MCP = &model.PluginMCPRuntimeMetadata{}
	}
	return record.RuntimeMetadata.MCP
}

func (s *PluginService) applyMCPLatestInteraction(
	record *model.PluginRecord,
	operation model.MCPInteractionOperation,
	status model.MCPInteractionStatus,
	target string,
	summary string,
	errorCode string,
	errorMessage string,
) {
	metadata := s.ensureMCPRuntimeMetadata(record)
	now := time.Now().UTC()
	metadata.LatestInteraction = &model.MCPInteractionSummary{
		Operation:    operation,
		Status:       status,
		At:           &now,
		Target:       target,
		Summary:      summary,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
	}
}

func (s *PluginService) recordMCPFailure(ctx context.Context, record *model.PluginRecord, operation model.MCPInteractionOperation, target, errorCode string, err error) {
	if record == nil || err == nil {
		return
	}
	s.applyMCPLatestInteraction(record, operation, model.MCPInteractionFailed, target, err.Error(), errorCode, err.Error())
	_ = s.withWriteStores(ctx, func(stores *pluginWriteStores) error {
		if err := stores.persistRecord(ctx, record); err != nil {
			return err
		}
		return appendMCPEventWithStores(ctx, stores, record, model.PluginEventMCPInteraction, operation, model.MCPInteractionFailed, target, err.Error(), errorCode, err.Error())
	})
}

func (s *PluginService) appendMCPEvent(
	ctx context.Context,
	record *model.PluginRecord,
	eventType model.PluginEventType,
	operation model.MCPInteractionOperation,
	status model.MCPInteractionStatus,
	target string,
	summary string,
	errorCode string,
	errorMessage string,
) error {
	payload := map[string]any{
		"operation": operation,
		"status":    status,
		"target":    target,
	}
	if summary != "" {
		payload["summary"] = summary
	}
	if errorCode != "" {
		payload["error_code"] = errorCode
	}
	if errorMessage != "" {
		payload["error_message"] = errorMessage
	}
	return s.appendEvent(ctx, record, eventType, model.PluginEventSourceOperator, summaryOrDefault(summary, string(operation)), payload)
}

func appendMCPEventWithStores(
	ctx context.Context,
	stores *pluginWriteStores,
	record *model.PluginRecord,
	eventType model.PluginEventType,
	operation model.MCPInteractionOperation,
	status model.MCPInteractionStatus,
	target string,
	summary string,
	errorCode string,
	errorMessage string,
) error {
	payload := map[string]any{
		"operation": operation,
		"status":    status,
		"target":    target,
	}
	if summary != "" {
		payload["summary"] = summary
	}
	if errorCode != "" {
		payload["error_code"] = errorCode
	}
	if errorMessage != "" {
		payload["error_message"] = errorMessage
	}
	return stores.appendEvent(ctx, record, eventType, model.PluginEventSourceOperator, summaryOrDefault(summary, string(operation)), payload)
}

func summaryOrDefault(summary string, fallback string) string {
	if strings.TrimSpace(summary) != "" {
		return summary
	}
	return fallback
}

func (s *PluginService) summarizeRefreshResult(result *model.PluginMCPRefreshResult) string {
	if result == nil {
		return ""
	}
	return fmt.Sprintf("tools=%d, resources=%d, prompts=%d", result.Snapshot.ToolCount, result.Snapshot.ResourceCount, result.Snapshot.PromptCount)
}

func (s *PluginService) summarizeToolCallResult(result *model.PluginMCPToolCallResult) string {
	if result == nil {
		return ""
	}
	for _, item := range result.Result.Content {
		if strings.TrimSpace(item.Text) != "" {
			return truncateSummary(item.Text)
		}
	}
	return truncateSummary(fmt.Sprintf("tool result blocks=%d", len(result.Result.Content)))
}

func (s *PluginService) summarizeResourceReadResult(result *model.PluginMCPResourceReadResult) string {
	if result == nil || len(result.Result.Contents) == 0 {
		return ""
	}
	if result.Result.Contents[0].URI != "" {
		return truncateSummary(result.Result.Contents[0].URI)
	}
	return truncateSummary(result.Result.Contents[0].Text)
}

func (s *PluginService) summarizePromptResult(result *model.PluginMCPPromptResult) string {
	if result == nil {
		return ""
	}
	if strings.TrimSpace(result.Result.Description) != "" {
		return truncateSummary(result.Result.Description)
	}
	if len(result.Result.Messages) > 0 {
		return truncateSummary(result.Result.Messages[0].Content.Text)
	}
	return ""
}

func (s *PluginService) summarizeMCPMetadata(metadata *model.PluginMCPRuntimeMetadata) string {
	if metadata == nil {
		return ""
	}
	return fmt.Sprintf("tools=%d, resources=%d, prompts=%d", metadata.ToolCount, metadata.ResourceCount, metadata.PromptCount)
}

func truncateSummary(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 160 {
		return value
	}
	return value[:160]
}

func clonePluginRuntimeMetadata(metadata *model.PluginRuntimeMetadata) *model.PluginRuntimeMetadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	if metadata.MCP != nil {
		mcp := *metadata.MCP
		mcp.LastDiscoveryAt = cloneTimePointer(metadata.MCP.LastDiscoveryAt)
		mcp.LatestInteraction = cloneMCPInteractionSummary(metadata.MCP.LatestInteraction)
		cloned.MCP = &mcp
	}
	return &cloned
}

func cloneMCPInteractionSummary(summary *model.MCPInteractionSummary) *model.MCPInteractionSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.At = cloneTimePointer(summary.At)
	return &cloned
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func catalogMatchesQuery(manifest model.PluginManifest, needle string) bool {
	if needle == "" {
		return true
	}
	if strings.Contains(strings.ToLower(manifest.Metadata.ID), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(manifest.Metadata.Name), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(manifest.Metadata.Description), needle) {
		return true
	}
	for _, tag := range manifest.Metadata.Tags {
		if strings.Contains(strings.ToLower(tag), needle) {
			return true
		}
	}
	return false
}

func (s *PluginService) resolveBuiltInMetadata(manifestPath string, manifest *model.PluginManifest, bundle *builtInBundleIndex) (*model.PluginBuiltInMetadata, bool, error) {
	relativePath := relativeBundleManifestPath(s.builtInsDir, manifestPath)
	if !isBuiltInBundleFamily(relativePath) {
		return nil, true, nil
	}
	if bundle == nil {
		return &model.PluginBuiltInMetadata{Official: true}, true, nil
	}

	entry, ok := bundle.byManifest[relativePath]
	if !ok {
		return nil, false, nil
	}
	if manifest != nil {
		if entry.ID != "" && entry.ID != manifest.Metadata.ID {
			return nil, false, fmt.Errorf("built-in bundle entry %s points to manifest %s with mismatched id %s", relativePath, entry.ID, manifest.Metadata.ID)
		}
		if entry.Kind != "" && entry.Kind != manifest.Kind {
			return nil, false, fmt.Errorf("built-in bundle entry %s points to manifest %s with mismatched kind %s", relativePath, entry.Kind, manifest.Kind)
		}
	}
	return builtInMetadataFromEntry(entry), true, nil
}

func marketplaceFromManifest(manifest model.PluginManifest, record *model.PluginRecord, builtIn *model.PluginBuiltInMetadata) model.MarketplacePluginDTO {
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
		BuiltIn:     builtIn,
	}
	if manifest.Source.Trust != nil {
		item.TrustStatus = manifest.Source.Trust.Status
		item.ApprovalState = manifest.Source.Trust.ApprovalState
	}
	if manifest.Source.Type == model.PluginSourceBuiltin || manifest.Source.Type == model.PluginSourceCatalog || manifest.Source.Type == model.PluginSourceLocal {
		item.Installable = true
	}
	if builtIn != nil {
		item.Installable = builtIn.Installable
		if !builtIn.Installable {
			item.BlockedReason = firstNonEmpty(builtIn.InstallBlockedReason, builtIn.ReadinessMessage, builtIn.AvailabilityMessage)
		}
	}
	if record != nil {
		item.Installed = true
	}
	return item
}

func marketplaceFromRemoteEntry(entry RemotePluginEntry, registryURL string, record *model.PluginRecord) model.MarketplacePluginDTO {
	item := model.MarketplacePluginDTO{
		ID:          entry.PluginID,
		Name:        entry.Name,
		Description: entry.Description,
		Version:     entry.Version,
		Author:      entry.Author,
		Kind:        entry.Kind,
		SourceType:  "registry",
		Registry:    registryURL,
		Installable: true,
		Runtime:     entry.Runtime,
	}
	if item.Kind == "" {
		item.Kind = "remote"
	}
	if record != nil {
		item.Installed = true
		item.Installable = false
		item.BlockedReason = "Already installed"
	}
	return item
}
