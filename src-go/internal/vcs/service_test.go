package vcs_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/vcs"
	"github.com/agentforge/server/internal/vcs/mock"
)

type fakeRepo struct {
	rows map[uuid.UUID]*model.VCSIntegration
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[uuid.UUID]*model.VCSIntegration{}} }
func (f *fakeRepo) Create(_ context.Context, r *model.VCSIntegration) error {
	f.rows[r.ID] = r
	return nil
}
func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*model.VCSIntegration, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}
func (f *fakeRepo) ListByProject(_ context.Context, p uuid.UUID) ([]*model.VCSIntegration, error) {
	var out []*model.VCSIntegration
	for _, v := range f.rows {
		if v.ProjectID == p {
			out = append(out, v)
		}
	}
	return out, nil
}
func (f *fakeRepo) Update(_ context.Context, r *model.VCSIntegration) error {
	f.rows[r.ID] = r
	return nil
}
func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.rows, id)
	return nil
}

type fakeSecrets struct{ values map[string]string }

func (f *fakeSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	v, ok := f.values[name]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

type recordingAudit struct {
	calls []recordingAuditCall
}

type recordingAuditCall struct {
	Action     string
	ResourceID string
	Payload    string
}

func (r *recordingAudit) Record(_ context.Context, _ uuid.UUID, action, resourceID, payload string, _ *uuid.UUID) {
	r.calls = append(r.calls, recordingAuditCall{Action: action, ResourceID: resourceID, Payload: payload})
}

func opsOf(calls []mock.Call) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.Op
	}
	return out
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func newServiceWithMockProvider(t *testing.T) (*vcs.Service, *fakeRepo, *mock.Provider, *recordingAudit) {
	t.Helper()
	reg := vcs.NewRegistry()
	mp := mock.New()
	reg.Register("github", func(host, token string) (vcs.Provider, error) { return mp, nil })
	repo := newFakeRepo()
	audit := &recordingAudit{}
	svc := vcs.NewService(repo, reg, &fakeSecrets{values: map[string]string{
		"vcs.github.demo.pat":     "ghp_xxx",
		"vcs.github.demo.webhook": "shh",
	}}, "https://agentforge.example/api/v1/vcs/github/webhook", audit)
	return svc, repo, mp, audit
}

func TestService_CreateValidatesPATAndCreatesWebhook(t *testing.T) {
	svc, _, mp, audit := newServiceWithMockProvider(t)

	rec, err := svc.Create(context.Background(), vcs.CreateInput{
		ProjectID:        uuid.New(),
		Provider:         "github",
		Host:             "github.com",
		Owner:            "octocat",
		Repo:             "hello",
		DefaultBranch:    "main",
		TokenSecretRef:   "vcs.github.demo.pat",
		WebhookSecretRef: "vcs.github.demo.webhook",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.WebhookID == nil || *rec.WebhookID == "" {
		t.Errorf("expected webhook_id to be persisted; got %+v", rec.WebhookID)
	}
	ops := opsOf(mp.Calls())
	if !contains(ops, "GetPullRequest") || !contains(ops, "CreateWebhook") {
		t.Errorf("expected validate+webhook calls; got %v", ops)
	}
	if len(audit.calls) != 1 || audit.calls[0].Action != "vcs.integration.create" {
		t.Errorf("expected one create audit event, got %+v", audit.calls)
	}
	// Audit payload must NOT leak token / webhook value / hook id.
	for _, banned := range []string{"ghp_xxx", "shh", "hook-"} {
		if strings.Contains(audit.calls[0].Payload, banned) {
			t.Errorf("audit payload leaked %q: %s", banned, audit.calls[0].Payload)
		}
	}
}

func TestService_CreateRejectsUnknownProvider(t *testing.T) {
	svc, _, _, _ := newServiceWithMockProvider(t)
	_, err := svc.Create(context.Background(), vcs.CreateInput{
		ProjectID:        uuid.New(),
		Provider:         "svn",
		Host:             "svn.example",
		Owner:            "o",
		Repo:             "r",
		TokenSecretRef:   "vcs.github.demo.pat",
		WebhookSecretRef: "vcs.github.demo.webhook",
	})
	if !errors.Is(err, vcs.ErrProviderUnsupported) {
		t.Fatalf("expected ErrProviderUnsupported, got %v", err)
	}
}

func TestService_CreateMapsAuthExpired(t *testing.T) {
	svc, _, mp, _ := newServiceWithMockProvider(t)
	mp.NextError(vcs.ErrAuthExpired)
	_, err := svc.Create(context.Background(), vcs.CreateInput{
		ProjectID:        uuid.New(),
		Provider:         "github",
		Host:             "github.com",
		Owner:            "o",
		Repo:             "r",
		TokenSecretRef:   "vcs.github.demo.pat",
		WebhookSecretRef: "vcs.github.demo.webhook",
	})
	if !errors.Is(err, vcs.ErrAuthExpired) {
		t.Fatalf("expected ErrAuthExpired, got %v", err)
	}
}

func TestService_CreateRequiresPublicCallback(t *testing.T) {
	reg := vcs.NewRegistry()
	reg.Register("github", func(host, token string) (vcs.Provider, error) { return mock.New(), nil })
	svc := vcs.NewService(newFakeRepo(), reg, &fakeSecrets{values: map[string]string{
		"vcs.github.demo.pat":     "ghp_xxx",
		"vcs.github.demo.webhook": "shh",
	}}, "", nil)
	_, err := svc.Create(context.Background(), vcs.CreateInput{
		ProjectID:        uuid.New(),
		Provider:         "github",
		Host:             "github.com",
		Owner:            "o",
		Repo:             "r",
		TokenSecretRef:   "vcs.github.demo.pat",
		WebhookSecretRef: "vcs.github.demo.webhook",
	})
	if !errors.Is(err, vcs.ErrPublicBaseURLNotConfigured) {
		t.Fatalf("expected ErrPublicBaseURLNotConfigured, got %v", err)
	}
}

func TestService_CreateRejectsUnresolvableTokenSecret(t *testing.T) {
	reg := vcs.NewRegistry()
	reg.Register("github", func(host, token string) (vcs.Provider, error) { return mock.New(), nil })
	svc := vcs.NewService(newFakeRepo(), reg, &fakeSecrets{values: map[string]string{
		"vcs.github.demo.webhook": "shh",
	}}, "https://example/cb", nil)
	_, err := svc.Create(context.Background(), vcs.CreateInput{
		ProjectID:        uuid.New(),
		Provider:         "github",
		Host:             "github.com",
		Owner:            "o",
		Repo:             "r",
		TokenSecretRef:   "missing.pat",
		WebhookSecretRef: "vcs.github.demo.webhook",
	})
	if !errors.Is(err, vcs.ErrSecretNotResolvable) {
		t.Fatalf("expected ErrSecretNotResolvable, got %v", err)
	}
}

func TestService_DeleteRemovesWebhookFirst(t *testing.T) {
	svc, repo, mp, audit := newServiceWithMockProvider(t)
	wh := "hook-1"
	rec := &model.VCSIntegration{
		ID:               uuid.New(),
		ProjectID:        uuid.New(),
		Provider:         "github",
		Host:             "github.com",
		Owner:            "o",
		Repo:             "r",
		WebhookID:        &wh,
		TokenSecretRef:   "vcs.github.demo.pat",
		WebhookSecretRef: "vcs.github.demo.webhook",
	}
	repo.rows[rec.ID] = rec

	if err := svc.Delete(context.Background(), rec.ID, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := repo.rows[rec.ID]; ok {
		t.Errorf("expected row removed")
	}
	if !contains(opsOf(mp.Calls()), "DeleteWebhook") {
		t.Errorf("expected DeleteWebhook to be called before row delete")
	}
	if len(audit.calls) != 1 || audit.calls[0].Action != "vcs.integration.delete" {
		t.Errorf("expected delete audit event, got %+v", audit.calls)
	}
}

func TestService_QueueSyncStampsLastSynced(t *testing.T) {
	svc, repo, _, audit := newServiceWithMockProvider(t)
	rec := &model.VCSIntegration{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Provider:  "github",
		Host:      "github.com",
		Owner:     "o",
		Repo:      "r",
		Status:    "active",
	}
	repo.rows[rec.ID] = rec
	out, err := svc.QueueSync(context.Background(), rec.ID, nil)
	if err != nil {
		t.Fatalf("QueueSync: %v", err)
	}
	if out.LastSyncedAt == nil {
		t.Fatal("expected LastSyncedAt to be stamped")
	}
	if len(audit.calls) != 1 || audit.calls[0].Action != "vcs.integration.sync" {
		t.Errorf("expected sync audit event, got %+v", audit.calls)
	}
}
