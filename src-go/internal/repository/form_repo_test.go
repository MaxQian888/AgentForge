package repository

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestNewFormRepository(t *testing.T) {
	repo := NewFormRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil FormRepository")
	}
}

func TestFormRepositoryGetBySlugNilDB(t *testing.T) {
	repo := NewFormRepository(nil)
	_, err := repo.GetBySlug(context.Background(), "bug-report")
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetBySlug() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestFormRepositoryRoundTripDefinitionsAndSubmissions(t *testing.T) {
	ctx := context.Background()
	repo := NewFormRepository(openFoundationRepoTestDB(t, &formDefinitionRecord{}, &formSubmissionRecord{}))

	projectID := uuid.New()
	taskID := uuid.New()
	formID := uuid.New()
	now := time.Date(2026, 3, 26, 14, 0, 0, 0, time.UTC)

	form := &model.FormDefinition{
		ID:             formID,
		ProjectID:      projectID,
		Name:           "Bug Report",
		Slug:           "bug-report",
		Fields:         `[{"key":"title","label":"Title","target":"title"}]`,
		TargetStatus:   "todo",
		TargetAssignee: nil,
		IsPublic:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.CreateDefinition(ctx, form); err != nil {
		t.Fatalf("CreateDefinition() error = %v", err)
	}

	storedForm, err := repo.GetBySlug(ctx, "bug-report")
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}
	if storedForm.Name != "Bug Report" || !storedForm.IsPublic {
		t.Fatalf("unexpected stored form: %+v", storedForm)
	}

	submission := &model.FormSubmission{
		ID:          uuid.New(),
		FormID:      formID,
		TaskID:      taskID,
		SubmittedBy: "anonymous",
		SubmittedAt: now,
		IPAddress:   "127.0.0.1",
	}
	if err := repo.CreateSubmission(ctx, submission); err != nil {
		t.Fatalf("CreateSubmission() error = %v", err)
	}

	submissions, err := repo.ListSubmissionsByForm(ctx, formID)
	if err != nil {
		t.Fatalf("ListSubmissionsByForm() error = %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("len(submissions) = %d, want 1", len(submissions))
	}
	if submissions[0].TaskID != taskID {
		t.Fatalf("submissions[0].TaskID = %s, want %s", submissions[0].TaskID, taskID)
	}
}
