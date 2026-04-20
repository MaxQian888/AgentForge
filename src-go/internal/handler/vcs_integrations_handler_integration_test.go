package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/vcs"
	"github.com/react-go-quick-starter/server/internal/vcs/mock"
)

// vcsIntegrationDDL mirrors migration 072 but spelled in SQLite-friendly
// form so the handler+service+repo chain can be exercised without a
// running Postgres.
const vcsIntegrationDDL = `
CREATE TABLE IF NOT EXISTS vcs_integrations (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	host TEXT NOT NULL,
	owner TEXT NOT NULL,
	repo TEXT NOT NULL,
	default_branch TEXT NOT NULL DEFAULT 'main',
	webhook_id TEXT,
	webhook_secret_ref TEXT NOT NULL,
	token_secret_ref TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	acting_employee_id TEXT,
	last_synced_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
)`

func openSQLiteForVCS(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(vcsIntegrationDDL).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

type stubSecretsResolver struct{ values map[string]string }

func (s *stubSecretsResolver) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	v, ok := s.values[name]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

// TestEndToEndCRUDWithMockProvider drives the full Handler+Service+Repo
// stack against a mock provider and SQLite-backed repository. It is the
// strongest in-process assurance that wiring stays consistent.
func TestEndToEndCRUDWithMockProvider(t *testing.T) {
	db := openSQLiteForVCS(t)
	repo := repository.NewVCSIntegrationRepo(db)

	reg := vcs.NewRegistry()
	mp := mock.New()
	reg.Register("github", func(_, _ string) (vcs.Provider, error) { return mp, nil })

	secretsAdapter := &stubSecretsResolver{values: map[string]string{
		"vcs.github.demo.pat":     "ghp_xxx",
		"vcs.github.demo.webhook": "shh",
	}}
	svc := vcs.NewService(repo, reg, secretsAdapter, "https://agentforge.example/api/v1/vcs/github/webhook", nil)
	h := handler.NewVCSIntegrationsHandler(svc)

	e := echo.New()
	projectID := uuid.New()

	// CREATE
	body, _ := json.Marshal(map[string]any{
		"provider":         "github",
		"host":             "github.com",
		"owner":            "octocat",
		"repo":             "hello",
		"defaultBranch":    "main",
		"tokenSecretRef":   "vcs.github.demo.pat",
		"webhookSecretRef": "vcs.github.demo.webhook",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(projectID.String())
	if err := h.Create(c); err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created model.VCSIntegration
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if created.WebhookID == nil || *created.WebhookID == "" {
		t.Fatalf("expected webhook id to be persisted, got %+v", created.WebhookID)
	}

	// Mock recorder must show the validate call AND the webhook create.
	ops := []string{}
	for _, call := range mp.Calls() {
		ops = append(ops, call.Op)
	}
	if !sliceContains(ops, "GetPullRequest") || !sliceContains(ops, "CreateWebhook") {
		t.Errorf("expected validate+webhook ops, got %v", ops)
	}

	// LIST returns the row
	rec = httptest.NewRecorder()
	c = e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	c.SetParamNames("pid")
	c.SetParamValues(projectID.String())
	if err := h.List(c); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list: code=%d err=%v", rec.Code, err)
	}
	var listed []model.VCSIntegration
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected list to contain created id, got %+v", listed)
	}

	// SYNC stamps last_synced_at and returns 202
	rec = httptest.NewRecorder()
	c = e.NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	c.SetParamNames("id")
	c.SetParamValues(created.ID.String())
	if err := h.Sync(c); err != nil || rec.Code != http.StatusAccepted {
		t.Fatalf("sync: code=%d err=%v", rec.Code, err)
	}

	// DELETE — must invoke DeleteWebhook before row delete
	rec = httptest.NewRecorder()
	c = e.NewContext(httptest.NewRequest(http.MethodDelete, "/", nil), rec)
	c.SetParamNames("id")
	c.SetParamValues(created.ID.String())
	if err := h.Delete(c); err != nil || rec.Code != http.StatusNoContent {
		t.Fatalf("delete: code=%d err=%v", rec.Code, err)
	}
	postDeleteOps := []string{}
	for _, call := range mp.Calls() {
		postDeleteOps = append(postDeleteOps, call.Op)
	}
	if !sliceContains(postDeleteOps, "DeleteWebhook") {
		t.Errorf("expected DeleteWebhook to fire on row delete; ops=%v", postDeleteOps)
	}

	// Row should be gone from PG
	if _, err := repo.Get(context.Background(), created.ID); err == nil {
		t.Errorf("expected row removed from DB after delete")
	}
}

// TestEndToEnd_CreateBlockedByMissingSecret asserts that a token ref the
// secrets store cannot resolve produces a 400 before any host call lands.
func TestEndToEnd_CreateBlockedByMissingSecret(t *testing.T) {
	db := openSQLiteForVCS(t)
	repo := repository.NewVCSIntegrationRepo(db)

	reg := vcs.NewRegistry()
	mp := mock.New()
	reg.Register("github", func(_, _ string) (vcs.Provider, error) { return mp, nil })

	svc := vcs.NewService(repo, reg, &stubSecretsResolver{values: map[string]string{
		"vcs.github.demo.webhook": "shh",
	}}, "https://agentforge.example/api/v1/vcs/github/webhook", nil)
	h := handler.NewVCSIntegrationsHandler(svc)

	e := echo.New()
	body, _ := json.Marshal(map[string]any{
		"provider":         "github",
		"host":             "github.com",
		"owner":            "o",
		"repo":             "r",
		"tokenSecretRef":   "missing.pat",
		"webhookSecretRef": "vcs.github.demo.webhook",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(uuid.New().String())
	if err := h.Create(c); err != nil {
		t.Fatalf("Create handler returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(mp.Calls()) != 0 {
		t.Errorf("expected no host calls when secret resolution failed; got %v", mp.Calls())
	}
}

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
