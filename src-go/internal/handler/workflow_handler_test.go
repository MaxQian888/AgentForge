package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeWorkflowRepo struct {
	config    *model.WorkflowConfig
	getErr    error
	upsertErr error
	upserted  *model.WorkflowConfig
}

func (f *fakeWorkflowRepo) GetByProject(_ context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.config == nil {
		return nil, errors.New("not found")
	}
	return f.config, nil
}

func (f *fakeWorkflowRepo) Upsert(_ context.Context, projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) (*model.WorkflowConfig, error) {
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	wf := &model.WorkflowConfig{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Transitions: transitions,
		Triggers:    triggers,
	}
	f.upserted = wf
	return wf, nil
}

type fakeWorkflowTemplateService struct {
	templates         []*model.WorkflowDefinition
	publishedTemplate *model.WorkflowDefinition
	duplicatedTemplate *model.WorkflowDefinition
	deletedTemplateID uuid.UUID
}

func (f *fakeWorkflowTemplateService) ListTemplates(_ context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error) {
	_ = projectID
	_ = query
	_ = category
	_ = source
	return f.templates, nil
}

func (f *fakeWorkflowTemplateService) CloneTemplate(_ context.Context, templateID uuid.UUID, projectID uuid.UUID, overrides map[string]any) (*model.WorkflowDefinition, error) {
	_ = projectID
	_ = overrides
	return &model.WorkflowDefinition{ID: templateID, Name: "Clone", Status: model.WorkflowDefStatusActive}, nil
}

func (f *fakeWorkflowTemplateService) CreateFromTemplate(_ context.Context, templateID uuid.UUID, projectID uuid.UUID, taskID *uuid.UUID, variables map[string]any) (*model.WorkflowExecution, error) {
	_ = projectID
	_ = taskID
	_ = variables
	return &model.WorkflowExecution{ID: uuid.New(), WorkflowID: templateID, Status: model.WorkflowExecStatusPending}, nil
}

func (f *fakeWorkflowTemplateService) PublishDefinitionAsTemplate(_ context.Context, definitionID uuid.UUID, projectID uuid.UUID, name string, description string) (*model.WorkflowDefinition, error) {
	_ = projectID
	f.publishedTemplate = &model.WorkflowDefinition{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategoryUser,
		SourceID:    &definitionID,
	}
	return f.publishedTemplate, nil
}

func (f *fakeWorkflowTemplateService) DuplicateTemplate(_ context.Context, templateID uuid.UUID, projectID uuid.UUID, name string, description string) (*model.WorkflowDefinition, error) {
	_ = projectID
	f.duplicatedTemplate = &model.WorkflowDefinition{
		ID:          uuid.New(),
		Name:        name,
		Description: description,
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategoryUser,
		SourceID:    &templateID,
	}
	return f.duplicatedTemplate, nil
}

func (f *fakeWorkflowTemplateService) DeleteTemplate(_ context.Context, templateID uuid.UUID, projectID uuid.UUID) error {
	_ = projectID
	f.deletedTemplateID = templateID
	return nil
}

func TestWorkflowHandler_Get_ReturnsDefault(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeWorkflowRepo{getErr: errors.New("not found")}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(repo)
	if err := h.Get(c); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var dto model.WorkflowConfigDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dto.ProjectID != projectID.String() {
		t.Fatalf("projectId = %q, want %q", dto.ProjectID, projectID.String())
	}
	if len(dto.Transitions) != 0 {
		t.Fatalf("transitions = %v, want empty", dto.Transitions)
	}
}

func TestWorkflowHandler_Put_Success(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeWorkflowRepo{}

	body := `{"transitions":{"inbox":["triaged"]},"triggers":[{"fromStatus":"triaged","toStatus":"assigned","action":"auto_assign"}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(repo)
	if err := h.Put(c); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.upserted == nil {
		t.Fatal("expected upserted config")
	}

	var dto model.WorkflowConfigDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(dto.Transitions) != 1 {
		t.Fatalf("transitions count = %d, want 1", len(dto.Transitions))
	}
	if len(dto.Triggers) != 1 {
		t.Fatalf("triggers count = %d, want 1", len(dto.Triggers))
	}
}

func TestWorkflowHandler_Put_RepoError(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeWorkflowRepo{upsertErr: errors.New("db error")}

	body := `{"transitions":{},"triggers":[]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(repo)
	if err := h.Put(c); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestWorkflowHandler_TemplateManagementEndpoints(t *testing.T) {
	projectID := uuid.New()
	templateID := uuid.New()
	definitionID := uuid.New()
	templateSvc := &fakeWorkflowTemplateService{
		templates: []*model.WorkflowDefinition{{
			ID:          templateID,
			ProjectID:   projectID,
			Name:        "Plan Code Review",
			Description: "Default workflow template",
			Status:      model.WorkflowDefStatusTemplate,
			Category:    model.WorkflowCategorySystem,
		}},
	}

	e := echo.New()
	h := handler.NewWorkflowHandler(&fakeWorkflowRepo{}).WithTemplateService(templateSvc)

	listReq := httptest.NewRequest(http.MethodGet, "/?category=system", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	if err := h.ListTemplates(listCtx); err != nil {
		t.Fatalf("ListTemplates() error: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListTemplates() status = %d, want %d", listRec.Code, http.StatusOK)
	}

	publishReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Project Template","description":"Reusable flow"}`))
	publishReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	publishRec := httptest.NewRecorder()
	publishCtx := e.NewContext(publishReq, publishRec)
	publishCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	publishCtx.SetParamNames("id")
	publishCtx.SetParamValues(definitionID.String())
	if err := h.PublishTemplate(publishCtx); err != nil {
		t.Fatalf("PublishTemplate() error: %v", err)
	}
	if publishRec.Code != http.StatusCreated {
		t.Fatalf("PublishTemplate() status = %d, want %d", publishRec.Code, http.StatusCreated)
	}
	if templateSvc.publishedTemplate == nil || templateSvc.publishedTemplate.Name != "Project Template" {
		t.Fatalf("publishedTemplate = %+v", templateSvc.publishedTemplate)
	}

	duplicateReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Template Copy","description":"Custom copy"}`))
	duplicateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	duplicateRec := httptest.NewRecorder()
	duplicateCtx := e.NewContext(duplicateReq, duplicateRec)
	duplicateCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	duplicateCtx.SetParamNames("id")
	duplicateCtx.SetParamValues(templateID.String())
	if err := h.DuplicateTemplate(duplicateCtx); err != nil {
		t.Fatalf("DuplicateTemplate() error: %v", err)
	}
	if duplicateRec.Code != http.StatusCreated {
		t.Fatalf("DuplicateTemplate() status = %d, want %d", duplicateRec.Code, http.StatusCreated)
	}
	if templateSvc.duplicatedTemplate == nil || templateSvc.duplicatedTemplate.Name != "Template Copy" {
		t.Fatalf("duplicatedTemplate = %+v", templateSvc.duplicatedTemplate)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/", nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	deleteCtx.SetParamNames("id")
	deleteCtx.SetParamValues(templateID.String())
	if err := h.DeleteTemplate(deleteCtx); err != nil {
		t.Fatalf("DeleteTemplate() error: %v", err)
	}
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("DeleteTemplate() status = %d, want %d", deleteRec.Code, http.StatusNoContent)
	}
	if templateSvc.deletedTemplateID != templateID {
		t.Fatalf("deletedTemplateID = %s, want %s", templateSvc.deletedTemplateID, templateID)
	}
}
