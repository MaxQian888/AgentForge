package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
)

func newTestVCSIntegrationRepo(t *testing.T) *VCSIntegrationRepo {
	return NewVCSIntegrationRepo(openFoundationRepoTestDB(t, &vcsIntegrationRecord{}))
}

func TestVCSIntegrationRepo_CreateGetListUpdateDelete(t *testing.T) {
	ctx := context.Background()
	repo := newTestVCSIntegrationRepo(t)
	pid := uuid.New()

	rec := &model.VCSIntegration{
		ProjectID:        pid,
		Provider:         "github",
		Host:             "github.com",
		Owner:            "octocat",
		Repo:             "hello",
		DefaultBranch:    "main",
		WebhookSecretRef: "vcs.github.octocat-hello.webhook",
		TokenSecretRef:   "vcs.github.octocat-hello.pat",
		Status:           "active",
	}
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Fatal("expected ID to be assigned by repo")
	}

	got, err := repo.Get(ctx, rec.ID)
	if err != nil || got == nil || got.Repo != "hello" || got.Status != "active" {
		t.Fatalf("Get: %v %+v", err, got)
	}

	list, err := repo.ListByProject(ctx, pid)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByProject: err=%v len=%d", err, len(list))
	}

	rec.Status = "auth_expired"
	hookID := "hook-99"
	rec.WebhookID = &hookID
	if err := repo.Update(ctx, rec); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(ctx, rec.ID)
	if got.Status != "auth_expired" {
		t.Errorf("expected status update to persist; got %q", got.Status)
	}
	if got.WebhookID == nil || *got.WebhookID != "hook-99" {
		t.Errorf("expected webhook_id update; got %+v", got.WebhookID)
	}

	if err := repo.Delete(ctx, rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestVCSIntegrationRepo_FindByRepo(t *testing.T) {
	ctx := context.Background()
	repo := newTestVCSIntegrationRepo(t)

	pidA := uuid.New()
	pidB := uuid.New()
	mustInsert := func(p uuid.UUID, owner, name string) {
		if err := repo.Create(ctx, &model.VCSIntegration{
			ProjectID:        p,
			Provider:         "github",
			Host:             "github.com",
			Owner:            owner,
			Repo:             name,
			DefaultBranch:    "main",
			WebhookSecretRef: "w",
			TokenSecretRef:   "t",
			Status:           "active",
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	mustInsert(pidA, "octocat", "hello")
	mustInsert(pidB, "octocat", "hello")
	mustInsert(pidA, "octocat", "world")

	rows, err := repo.FindByRepo(ctx, "github.com", "octocat", "hello")
	if err != nil {
		t.Fatalf("FindByRepo: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 integrations across projects, got %d", len(rows))
	}
}

func TestVCSIntegrationRepo_UpdateMissingReturnsErrNotFound(t *testing.T) {
	repo := newTestVCSIntegrationRepo(t)
	err := repo.Update(context.Background(), &model.VCSIntegration{ID: uuid.New(), Status: "paused"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestVCSIntegrationRepo_DeleteMissingReturnsErrNotFound(t *testing.T) {
	repo := newTestVCSIntegrationRepo(t)
	if err := repo.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
