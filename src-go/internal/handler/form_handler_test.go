package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type formServiceMock struct {
	form        *model.FormDefinition
	createdForm *model.FormDefinition
	submitInput service.FormSubmissionInput
	submitSlug  string
	task        *model.Task
}

func (m *formServiceMock) CreateForm(_ context.Context, form *model.FormDefinition) error {
	m.createdForm = form
	return nil
}
func (m *formServiceMock) GetForm(_ context.Context, _ uuid.UUID) (*model.FormDefinition, error) {
	return m.form, nil
}
func (m *formServiceMock) ListForms(_ context.Context, _ uuid.UUID) ([]*model.FormDefinition, error) {
	if m.form == nil {
		return nil, nil
	}
	return []*model.FormDefinition{m.form}, nil
}
func (m *formServiceMock) UpdateForm(_ context.Context, _ *model.FormDefinition) error { return nil }
func (m *formServiceMock) DeleteForm(_ context.Context, _ uuid.UUID) error             { return nil }
func (m *formServiceMock) GetFormBySlug(_ context.Context, _ string) (*model.FormDefinition, error) {
	return m.form, nil
}
func (m *formServiceMock) SubmitForm(_ context.Context, slug string, input service.FormSubmissionInput) (*model.Task, error) {
	m.submitSlug = slug
	m.submitInput = input
	return m.task, nil
}

func TestFormHandler_CreateAndSubmit(t *testing.T) {
	projectID := uuid.New()
	formID := uuid.New()
	taskID := uuid.New()
	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	svc := &formServiceMock{
		form: &model.FormDefinition{
			ID:        formID,
			ProjectID: projectID,
			Name:      "Bug Report",
			Slug:      "bug-report",
			Fields:    `[]`,
			IsPublic:  true,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Title:     "Login broken",
			Status:    model.TaskStatusInbox,
			Priority:  "medium",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	h := handler.NewFormHandler(svc)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/forms", strings.NewReader(`{"name":"Bug Report","slug":"bug-report","fields":[],"isPublic":true}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := h.Create(createCtx); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/forms/bug-report/submit", strings.NewReader(`{"submittedBy":"anon","values":{"title":"Login broken"}}`))
	submitReq.Header.Set("Content-Type", "application/json")
	submitRec := httptest.NewRecorder()
	submitCtx := e.NewContext(submitReq, submitRec)
	submitCtx.SetPath("/api/v1/forms/:slug/submit")
	submitCtx.SetParamNames("slug")
	submitCtx.SetParamValues("bug-report")

	if err := h.Submit(submitCtx); err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if submitRec.Code != http.StatusCreated {
		t.Fatalf("submit status = %d, want %d", submitRec.Code, http.StatusCreated)
	}
	if svc.submitSlug != "bug-report" || svc.submitInput.Values["title"] != "Login broken" {
		t.Fatalf("unexpected submit input: slug=%s input=%+v", svc.submitSlug, svc.submitInput)
	}
}

func TestFormHandler_PrivateFormRequiresAuth(t *testing.T) {
	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	svc := &formServiceMock{
		form: &model.FormDefinition{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Name:      "Private",
			Slug:      "private",
			Fields:    `[]`,
			IsPublic:  false,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	h := handler.NewFormHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/forms/private/submit", strings.NewReader(`{"values":{"title":"Need access"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/forms/:slug/submit")
	c.SetParamNames("slug")
	c.SetParamValues("private")

	if err := h.Submit(c); err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
