package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

type fakeLifecycleProjectRepo struct {
	projects    map[uuid.UUID]*model.Project
	setArchived func(id, owner uuid.UUID, t time.Time) error
	setUnarchv  func(id uuid.UUID) error
	deleteFn    func(id uuid.UUID) error
}

func (f *fakeLifecycleProjectRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	clone := *p
	return &clone, nil
}

func (f *fakeLifecycleProjectRepo) SetArchived(_ context.Context, id, owner uuid.UUID, t time.Time) error {
	if f.setArchived != nil {
		if err := f.setArchived(id, owner, t); err != nil {
			return err
		}
	}
	p, ok := f.projects[id]
	if !ok {
		return repository.ErrNotFound
	}
	p.Status = model.ProjectStatusArchived
	p.ArchivedAt = &t
	ownerCopy := owner
	p.ArchivedByUserID = &ownerCopy
	return nil
}

func (f *fakeLifecycleProjectRepo) SetUnarchived(_ context.Context, id uuid.UUID) error {
	if f.setUnarchv != nil {
		if err := f.setUnarchv(id); err != nil {
			return err
		}
	}
	p, ok := f.projects[id]
	if !ok {
		return repository.ErrNotFound
	}
	p.Status = model.ProjectStatusActive
	p.ArchivedAt = nil
	p.ArchivedByUserID = nil
	return nil
}

func (f *fakeLifecycleProjectRepo) Delete(_ context.Context, id uuid.UUID) error {
	if f.deleteFn != nil {
		if err := f.deleteFn(id); err != nil {
			return err
		}
	}
	if _, ok := f.projects[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.projects, id)
	return nil
}

type fakeCanceller struct {
	calls []uuid.UUID
	err   error
}

func (c *fakeCanceller) CancelAllActiveForProject(_ context.Context, projectID uuid.UUID, _ string) error {
	c.calls = append(c.calls, projectID)
	return c.err
}

func newActiveProject() *model.Project {
	id := uuid.New()
	return &model.Project{
		ID:     id,
		Name:   "p",
		Slug:   "p",
		Status: model.ProjectStatusActive,
	}
}

func TestProjectLifecycleArchiveFlipsStatusAndTriggersCascade(t *testing.T) {
	project := newActiveProject()
	repo := &fakeLifecycleProjectRepo{projects: map[uuid.UUID]*model.Project{project.ID: project}}
	teamCancel := &fakeCanceller{}
	wfCancel := &fakeCanceller{}
	svc := NewProjectLifecycleService(repo).
		WithTeamCanceller(teamCancel).
		WithWorkflowCanceller(wfCancel)

	owner := uuid.New()
	refreshed, err := svc.Archive(context.Background(), project.ID, owner)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if refreshed.Status != model.ProjectStatusArchived {
		t.Errorf("expected archived status, got %q", refreshed.Status)
	}
	if refreshed.ArchivedAt == nil || refreshed.ArchivedByUserID == nil {
		t.Errorf("expected archived_at and archived_by_user_id to be set")
	}
	if len(teamCancel.calls) != 1 || len(wfCancel.calls) != 1 {
		t.Errorf("expected one team and one workflow cascade call, got teams=%d workflows=%d",
			len(teamCancel.calls), len(wfCancel.calls))
	}
}

func TestProjectLifecycleArchiveRejectsAlreadyArchived(t *testing.T) {
	project := newActiveProject()
	project.Status = model.ProjectStatusArchived
	repo := &fakeLifecycleProjectRepo{projects: map[uuid.UUID]*model.Project{project.ID: project}}
	svc := NewProjectLifecycleService(repo)

	_, err := svc.Archive(context.Background(), project.ID, uuid.New())
	if !errors.Is(err, ErrProjectAlreadyArchived) {
		t.Errorf("expected ErrProjectAlreadyArchived, got %v", err)
	}
}

func TestProjectLifecycleArchiveIgnoresCascadeFailures(t *testing.T) {
	project := newActiveProject()
	repo := &fakeLifecycleProjectRepo{projects: map[uuid.UUID]*model.Project{project.ID: project}}
	failing := &fakeCanceller{err: errors.New("cancel boom")}
	svc := NewProjectLifecycleService(repo).WithTeamCanceller(failing)

	if _, err := svc.Archive(context.Background(), project.ID, uuid.New()); err != nil {
		t.Errorf("Archive should not fail on cascade error, got %v", err)
	}
	if project.Status != model.ProjectStatusArchived {
		t.Errorf("expected status to be archived even when cascade fails")
	}
}

func TestProjectLifecycleUnarchiveRejectsNonArchived(t *testing.T) {
	project := newActiveProject()
	repo := &fakeLifecycleProjectRepo{projects: map[uuid.UUID]*model.Project{project.ID: project}}
	svc := NewProjectLifecycleService(repo)

	_, err := svc.Unarchive(context.Background(), project.ID)
	if !errors.Is(err, ErrProjectNotArchived) {
		t.Errorf("expected ErrProjectNotArchived, got %v", err)
	}
}

func TestProjectLifecycleUnarchiveFlipsBack(t *testing.T) {
	project := newActiveProject()
	project.Status = model.ProjectStatusArchived
	now := time.Now().UTC()
	project.ArchivedAt = &now
	owner := uuid.New()
	project.ArchivedByUserID = &owner
	repo := &fakeLifecycleProjectRepo{projects: map[uuid.UUID]*model.Project{project.ID: project}}
	svc := NewProjectLifecycleService(repo)

	refreshed, err := svc.Unarchive(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("Unarchive failed: %v", err)
	}
	if refreshed.Status != model.ProjectStatusActive {
		t.Errorf("expected active, got %q", refreshed.Status)
	}
	if refreshed.ArchivedAt != nil || refreshed.ArchivedByUserID != nil {
		t.Errorf("expected archived_at/by cleared")
	}
}

func TestProjectLifecycleDeleteRequiresArchived(t *testing.T) {
	project := newActiveProject()
	repo := &fakeLifecycleProjectRepo{projects: map[uuid.UUID]*model.Project{project.ID: project}}
	svc := NewProjectLifecycleService(repo)

	err := svc.DeleteArchived(context.Background(), project.ID, DefaultDeleteOptions())
	if !errors.Is(err, ErrProjectMustBeArchived) {
		t.Errorf("expected ErrProjectMustBeArchived, got %v", err)
	}

	project.Status = model.ProjectStatusArchived
	if err := svc.DeleteArchived(context.Background(), project.ID, DefaultDeleteOptions()); err != nil {
		t.Errorf("DeleteArchived on archived project should succeed, got %v", err)
	}
	if _, ok := repo.projects[project.ID]; ok {
		t.Errorf("project row should have been removed")
	}
}
