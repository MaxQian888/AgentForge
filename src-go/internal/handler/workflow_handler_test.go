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

// --- DAG def repo stub used by trigger-sync tests ---

type fakeDAGDefRepo struct {
	created  *model.WorkflowDefinition
	updated  *model.WorkflowDefinition
	createErr error
	updateErr error
	getByIDFn func(id uuid.UUID) (*model.WorkflowDefinition, error)
}

func (f *fakeDAGDefRepo) Create(_ context.Context, def *model.WorkflowDefinition) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = def
	return nil
}

func (f *fakeDAGDefRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(id)
	}
	// Return the created or updated record if one is stored.
	if f.created != nil && f.created.ID == id {
		return f.created, nil
	}
	if f.updated != nil && f.updated.ID == id {
		return f.updated, nil
	}
	return &model.WorkflowDefinition{ID: id}, nil
}

func (f *fakeDAGDefRepo) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.WorkflowDefinition, error) {
	return nil, nil
}

func (f *fakeDAGDefRepo) Update(_ context.Context, id uuid.UUID, def *model.WorkflowDefinition) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	def.ID = id
	f.updated = def
	return nil
}

func (f *fakeDAGDefRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

// --- Recording trigger syncer ---

type syncCall struct {
	WorkflowID uuid.UUID
	ProjectID  uuid.UUID
	Nodes      []model.WorkflowNode
}

type recordingTriggerSyncer struct {
	calls []syncCall
	err   error
}

func (r *recordingTriggerSyncer) SyncFromDefinition(_ context.Context, wid, pid uuid.UUID, nodes []model.WorkflowNode, _ *uuid.UUID) error {
	r.calls = append(r.calls, syncCall{wid, pid, nodes})
	return r.err
}

// --- Trigger sync tests ---

func TestCreateDefinition_TriggersSyncAfterCreate(t *testing.T) {
	projectID := uuid.New()

	triggerNodeConfig := `{"source":"schedule","schedule":{"cron":"0 9 * * 1"}}`
	nodes := []model.WorkflowNode{
		{ID: "t1", Type: model.NodeTypeTrigger, Label: "Every Monday", Config: json.RawMessage(triggerNodeConfig)},
	}
	nodesJSON, _ := json.Marshal(nodes)

	defRepo := &fakeDAGDefRepo{}
	// GetByID returns the created def with its full node JSON.
	defRepo.getByIDFn = func(id uuid.UUID) (*model.WorkflowDefinition, error) {
		return &model.WorkflowDefinition{ID: id, ProjectID: projectID, Nodes: nodesJSON}, nil
	}

	syncer := &recordingTriggerSyncer{}

	e := echo.New()
	body := `{"name":"My Workflow","nodes":[{"id":"t1","type":"trigger","label":"Every Monday","config":{"source":"schedule","schedule":{"cron":"0 9 * * 1"}}}],"edges":[]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(&fakeWorkflowRepo{})
	h.WithDAGService(nil, defRepo, nil, nil)
	h.SetTriggerSyncer(syncer)

	if err := h.CreateDefinition(c); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if len(syncer.calls) != 1 {
		t.Fatalf("SyncFromDefinition call count = %d, want 1", len(syncer.calls))
	}
	call := syncer.calls[0]
	if call.ProjectID != projectID {
		t.Fatalf("sync projectID = %s, want %s", call.ProjectID, projectID)
	}
	if len(call.Nodes) != 1 || call.Nodes[0].Type != model.NodeTypeTrigger {
		t.Fatalf("sync nodes = %+v, want 1 trigger node", call.Nodes)
	}
}

func TestUpdateDefinition_TriggersSyncWhenNodesChange(t *testing.T) {
	projectID := uuid.New()
	workflowID := uuid.New()

	triggerNodeConfig := `{"source":"im","im":{"channel":"#dev"}}`
	nodes := []model.WorkflowNode{
		{ID: "t2", Type: model.NodeTypeTrigger, Label: "IM trigger", Config: json.RawMessage(triggerNodeConfig)},
	}
	nodesJSON, _ := json.Marshal(nodes)

	defRepo := &fakeDAGDefRepo{}
	defRepo.getByIDFn = func(id uuid.UUID) (*model.WorkflowDefinition, error) {
		return &model.WorkflowDefinition{ID: id, ProjectID: projectID, Nodes: nodesJSON}, nil
	}

	syncer := &recordingTriggerSyncer{}

	e := echo.New()

	// --- Sub-test A: nodes are in the request body → sync must be called ---
	t.Run("nodes_present_syncs", func(t *testing.T) {
		body := `{"nodes":[{"id":"t2","type":"trigger","label":"IM trigger","config":{"source":"im","im":{"channel":"#dev"}}}]}`
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(workflowID.String())

		h := handler.NewWorkflowHandler(&fakeWorkflowRepo{})
		h.WithDAGService(nil, defRepo, nil, nil)
		h.SetTriggerSyncer(syncer)

		if err := h.UpdateDefinition(c); err != nil {
			t.Fatalf("UpdateDefinition() error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(syncer.calls) != 1 {
			t.Fatalf("SyncFromDefinition call count = %d, want 1", len(syncer.calls))
		}
	})

	// --- Sub-test B: nodes absent (metadata-only update) → sync must NOT be called ---
	t.Run("nodes_absent_no_sync", func(t *testing.T) {
		syncer2 := &recordingTriggerSyncer{}
		body := `{"name":"Renamed Workflow"}`
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(workflowID.String())

		h := handler.NewWorkflowHandler(&fakeWorkflowRepo{})
		h.WithDAGService(nil, defRepo, nil, nil)
		h.SetTriggerSyncer(syncer2)

		if err := h.UpdateDefinition(c); err != nil {
			t.Fatalf("UpdateDefinition() error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if len(syncer2.calls) != 0 {
			t.Fatalf("SyncFromDefinition call count = %d, want 0 (metadata-only update)", len(syncer2.calls))
		}
	})
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
