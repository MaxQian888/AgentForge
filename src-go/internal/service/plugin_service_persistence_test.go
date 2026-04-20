package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPluginService_RollsBackPluginPersistenceWhenAuditWriteFails(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeManifest(t, pluginsDir, "local/repo-search.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: repo-search
  name: Repo Search
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["tool.js"]
`)
	db := openPluginPersistenceTestDB(t)
	registryRepo := repository.NewPluginRegistryRepository(db)
	instanceRepo := repository.NewPluginInstanceRepository(db)
	eventStore := &failingPluginEventStore{
		repo: repository.NewPluginEventRepository(db),
		err:  errors.New("forced audit failure"),
	}
	svc := service.NewPluginService(registryRepo, &fakePluginRuntimeClient{}, nil, pluginsDir).
		WithInstanceStore(instanceRepo).
		WithEventStore(eventStore)

	if _, err := svc.RegisterLocalPath(ctx, manifestPath); err == nil || !strings.Contains(err.Error(), "forced audit failure") {
		t.Fatalf("expected forced audit failure, got %v", err)
	}

	if _, err := registryRepo.GetByID(ctx, "repo-search"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected plugin registry rollback, got %v", err)
	}
	if _, err := instanceRepo.GetCurrentByPluginID(ctx, "repo-search"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected plugin instance rollback, got %v", err)
	}
	events, err := eventStore.ListByPluginID(ctx, "repo-search", 10)
	if err != nil {
		t.Fatalf("list plugin events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected plugin event rollback, got %+v", events)
	}
}

func TestPluginService_DatabaseBackedDefaultsSurviveServiceRestart(t *testing.T) {
	ctx := context.Background()
	pluginsDir := t.TempDir()
	manifestPath := writeManifest(t, pluginsDir, "local/wasm-feishu.yaml", `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: wasm-feishu
  name: WASM Feishu
  version: 1.0.0
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
`)
	db := openPluginPersistenceTestDB(t)
	goRuntime := &fakeGoPluginRuntime{}
	svc := service.NewPluginService(repository.NewPluginRegistryRepository(db), &fakePluginRuntimeClient{}, goRuntime, pluginsDir)

	record, err := svc.RegisterLocalPath(ctx, manifestPath)
	if err != nil {
		t.Fatalf("register local path: %v", err)
	}
	if _, err := svc.Activate(ctx, record.Metadata.ID); err != nil {
		t.Fatalf("activate plugin: %v", err)
	}

	restarted := service.NewPluginService(repository.NewPluginRegistryRepository(db), &fakePluginRuntimeClient{}, goRuntime, pluginsDir)
	loaded, err := restarted.GetByID(ctx, record.Metadata.ID)
	if err != nil {
		t.Fatalf("get persisted plugin after restart: %v", err)
	}
	if loaded.CurrentInstance == nil {
		t.Fatal("expected current instance to survive service restart")
	}
	if loaded.CurrentInstance.LifecycleState != model.PluginStateActive {
		t.Fatalf("expected active current instance, got %+v", loaded.CurrentInstance)
	}

	events, err := restarted.ListEvents(ctx, record.Metadata.ID, 10)
	if err != nil {
		t.Fatalf("list persisted events after restart: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected persisted plugin events after restart")
	}
}

type failingPluginEventStore struct {
	repo *repository.PluginEventRepository
	err  error
}

func (s *failingPluginEventStore) Append(ctx context.Context, event *model.PluginEventRecord) error {
	if err := s.repo.Append(ctx, event); err != nil {
		return err
	}
	return s.err
}

func (s *failingPluginEventStore) ListByPluginID(ctx context.Context, pluginID string, limit int) ([]*model.PluginEventRecord, error) {
	return s.repo.ListByPluginID(ctx, pluginID, limit)
}

func (s *failingPluginEventStore) WithDB(db *gorm.DB) repository.PluginEventDBBinder {
	return &failingPluginEventStore{
		repo: repository.NewPluginEventRepository(db),
		err:  s.err,
	}
}

func openPluginPersistenceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	schema := []string{
		`CREATE TABLE plugins (
			plugin_id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			description TEXT,
			tags TEXT,
			manifest TEXT NOT NULL,
			source_type TEXT NOT NULL,
			source_path TEXT,
			runtime TEXT NOT NULL,
			lifecycle_state TEXT NOT NULL,
			runtime_host TEXT NOT NULL,
			last_health_at DATETIME,
			last_error TEXT,
			restart_count INTEGER NOT NULL DEFAULT 0,
			resolved_source_path TEXT,
			runtime_metadata TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE plugin_instances (
			plugin_id TEXT PRIMARY KEY,
			project_id TEXT,
			runtime_host TEXT NOT NULL,
			lifecycle_state TEXT NOT NULL,
			resolved_source_path TEXT,
			runtime_metadata TEXT,
			restart_count INTEGER NOT NULL DEFAULT 0,
			last_health_at DATETIME,
			last_error TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(plugin_id) REFERENCES plugins(plugin_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE plugin_events (
			id TEXT PRIMARY KEY,
			plugin_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			event_source TEXT NOT NULL,
			lifecycle_state TEXT,
			summary TEXT,
			payload TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(plugin_id) REFERENCES plugins(plugin_id) ON DELETE CASCADE
		)`,
	}

	for _, stmt := range schema {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create plugin persistence schema: %v", err)
		}
	}

	return db
}
