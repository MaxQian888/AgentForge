package service

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type fakeFormRepo struct {
	formsBySlug       map[string]*model.FormDefinition
	createdForm       *model.FormDefinition
	updatedForm       *model.FormDefinition
	deletedFormID     uuid.UUID
	createdSubmission *model.FormSubmission
}

func (f *fakeFormRepo) CreateDefinition(_ context.Context, form *model.FormDefinition) error {
	f.createdForm = form
	if f.formsBySlug == nil {
		f.formsBySlug = map[string]*model.FormDefinition{}
	}
	f.formsBySlug[form.Slug] = form
	return nil
}

func (f *fakeFormRepo) GetDefinition(_ context.Context, id uuid.UUID) (*model.FormDefinition, error) {
	for _, form := range f.formsBySlug {
		if form.ID == id {
			return form, nil
		}
	}
	return nil, nil
}

func (f *fakeFormRepo) GetBySlug(_ context.Context, slug string) (*model.FormDefinition, error) {
	return f.formsBySlug[slug], nil
}

func (f *fakeFormRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.FormDefinition, error) {
	result := make([]*model.FormDefinition, 0)
	for _, form := range f.formsBySlug {
		if form.ProjectID == projectID {
			result = append(result, form)
		}
	}
	return result, nil
}

func (f *fakeFormRepo) UpdateDefinition(_ context.Context, form *model.FormDefinition) error {
	f.updatedForm = form
	f.formsBySlug[form.Slug] = form
	return nil
}

func (f *fakeFormRepo) DeleteDefinition(_ context.Context, id uuid.UUID) error {
	f.deletedFormID = id
	return nil
}

func (f *fakeFormRepo) CreateSubmission(_ context.Context, submission *model.FormSubmission) error {
	f.createdSubmission = submission
	return nil
}

type fakeFormTaskRepo struct {
	createdTask *model.Task
}

func (f *fakeFormTaskRepo) Create(_ context.Context, task *model.Task) error {
	f.createdTask = task
	return nil
}

type fakeFormCustomFieldRepo struct {
	values []*model.CustomFieldValue
}

func (f *fakeFormCustomFieldRepo) SetValue(_ context.Context, value *model.CustomFieldValue) error {
	f.values = append(f.values, value)
	return nil
}

func TestFormServiceCreateAndGetForm(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeFormRepo{formsBySlug: map[string]*model.FormDefinition{}}
	service := NewFormService(repo, &fakeFormTaskRepo{}, &fakeFormCustomFieldRepo{})

	form := &model.FormDefinition{
		ProjectID: projectID,
		Name:      "Bug Report",
		Slug:      "bug-report",
		Fields:    `[]`,
	}
	if err := service.CreateForm(context.Background(), form); err != nil {
		t.Fatalf("CreateForm() error = %v", err)
	}
	if form.ID == uuid.Nil {
		t.Fatal("expected CreateForm to assign an ID")
	}
	stored, err := service.GetFormBySlug(context.Background(), "bug-report")
	if err != nil {
		t.Fatalf("GetFormBySlug() error = %v", err)
	}
	if stored == nil || stored.Name != "Bug Report" {
		t.Fatalf("unexpected stored form: %+v", stored)
	}
}

func TestFormServiceSubmitFormCreatesTaskSubmissionAndCustomValues(t *testing.T) {
	projectID := uuid.New()
	assigneeID := uuid.New()
	fieldID := uuid.New()
	repo := &fakeFormRepo{
		formsBySlug: map[string]*model.FormDefinition{
			"bug-report": {
				ID:             uuid.New(),
				ProjectID:      projectID,
				Name:           "Bug Report",
				Slug:           "bug-report",
				Fields:         `[{"key":"title","target":"title"},{"key":"severity","target":"cf:` + fieldID.String() + `"}]`,
				TargetStatus:   "triaged",
				TargetAssignee: &assigneeID,
				IsPublic:       true,
			},
		},
	}
	taskRepo := &fakeFormTaskRepo{}
	fieldRepo := &fakeFormCustomFieldRepo{}
	service := NewFormService(repo, taskRepo, fieldRepo)
	service.now = func() time.Time { return time.Date(2026, 3, 26, 11, 0, 0, 0, time.UTC) }

	task, err := service.SubmitForm(context.Background(), "bug-report", FormSubmissionInput{
		SubmittedBy: "anonymous",
		IPAddress:   "127.0.0.1",
		Values: map[string]string{
			"title":    "Login broken",
			"severity": "P0",
		},
	})
	if err != nil {
		t.Fatalf("SubmitForm() error = %v", err)
	}
	if task == nil || task.Title != "Login broken" || task.Status != "triaged" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if task.AssigneeID == nil || *task.AssigneeID != assigneeID {
		t.Fatalf("unexpected assignee on created task: %+v", task)
	}
	if repo.createdSubmission == nil || repo.createdSubmission.TaskID != task.ID {
		t.Fatalf("submission not recorded correctly: %+v", repo.createdSubmission)
	}
	if len(fieldRepo.values) != 1 || fieldRepo.values[0].Value != `"P0"` {
		t.Fatalf("unexpected custom field values: %+v", fieldRepo.values)
	}
}

func TestFormServiceRateLimitsPublicForms(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeFormRepo{
		formsBySlug: map[string]*model.FormDefinition{
			"public": {
				ID:        uuid.New(),
				ProjectID: projectID,
				Name:      "Public",
				Slug:      "public",
				Fields:    `[{"key":"title","target":"title"}]`,
				IsPublic:  true,
			},
		},
	}
	service := NewFormService(repo, &fakeFormTaskRepo{}, &fakeFormCustomFieldRepo{})
	base := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }

	for i := 0; i < 10; i++ {
		if _, err := service.SubmitForm(context.Background(), "public", FormSubmissionInput{
			IPAddress: "127.0.0.1",
			Values:    map[string]string{"title": "Issue"},
		}); err != nil {
			t.Fatalf("unexpected rate limit before threshold: %v", err)
		}
	}
	if _, err := service.SubmitForm(context.Background(), "public", FormSubmissionInput{
		IPAddress: "127.0.0.1",
		Values:    map[string]string{"title": "Issue"},
	}); err == nil {
		t.Fatal("expected public form rate limit error")
	}
}
