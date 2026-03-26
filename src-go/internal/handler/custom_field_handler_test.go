package handler_test

import (
	"context"
	"encoding/json"
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
)

type customFieldServiceMock struct {
	definitions []*model.CustomFieldDefinition
	values      []*model.CustomFieldValue
	created     *model.CustomFieldDefinition
	updated     *model.CustomFieldDefinition
	reordered   []uuid.UUID
	setValue    *model.CustomFieldValue
}

func (m *customFieldServiceMock) CreateField(_ context.Context, definition *model.CustomFieldDefinition) error {
	m.created = definition
	return nil
}
func (m *customFieldServiceMock) GetField(_ context.Context, id uuid.UUID) (*model.CustomFieldDefinition, error) {
	for _, definition := range m.definitions {
		if definition.ID == id {
			return definition, nil
		}
	}
	return nil, nil
}
func (m *customFieldServiceMock) ListFields(_ context.Context, projectID uuid.UUID) ([]*model.CustomFieldDefinition, error) {
	return m.definitions, nil
}
func (m *customFieldServiceMock) UpdateField(_ context.Context, definition *model.CustomFieldDefinition) error {
	m.updated = definition
	return nil
}
func (m *customFieldServiceMock) DeleteField(_ context.Context, _ uuid.UUID) error { return nil }
func (m *customFieldServiceMock) ReorderFields(_ context.Context, _ uuid.UUID, orderedIDs []uuid.UUID) error {
	m.reordered = orderedIDs
	return nil
}
func (m *customFieldServiceMock) SetValue(_ context.Context, value *model.CustomFieldValue) error {
	m.setValue = value
	return nil
}
func (m *customFieldServiceMock) ClearValue(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *customFieldServiceMock) GetValuesForTask(_ context.Context, _ uuid.UUID) ([]*model.CustomFieldValue, error) {
	return m.values, nil
}

type customFieldValidator struct{ validator *validator.Validate }

func (v *customFieldValidator) Validate(i interface{}) error { return v.validator.Struct(i) }

func TestCustomFieldHandler_ListDefinitions(t *testing.T) {
	projectID := uuid.New()
	svc := &customFieldServiceMock{
		definitions: []*model.CustomFieldDefinition{{
			ID:        uuid.New(),
			ProjectID: projectID,
			Name:      "Priority",
			FieldType: model.CustomFieldTypeSelect,
			Options:   `["P0"]`,
			SortOrder: 1,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/fields", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewCustomFieldHandler(svc)
	if err := h.ListDefinitions(c); err != nil {
		t.Fatalf("ListDefinitions() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body) != 1 || body[0]["name"] != "Priority" {
		t.Fatalf("unexpected response body: %#v", body)
	}
}

func TestCustomFieldHandler_CreateAndSetValue(t *testing.T) {
	projectID := uuid.New()
	fieldID := uuid.New()
	taskID := uuid.New()
	svc := &customFieldServiceMock{
		definitions: []*model.CustomFieldDefinition{{
			ID:        fieldID,
			ProjectID: projectID,
			Name:      "Priority",
			FieldType: model.CustomFieldTypeSelect,
			Options:   `["P0"]`,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}},
	}

	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/fields", strings.NewReader(`{"name":"Priority","fieldType":"select","options":["P0"],"required":true}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewCustomFieldHandler(svc)
	if err := h.CreateDefinition(createCtx); err != nil {
		t.Fatalf("CreateDefinition() error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}
	if svc.created == nil || svc.created.ProjectID != projectID {
		t.Fatalf("unexpected created definition: %+v", svc.created)
	}

	valueReq := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/tasks/"+taskID.String()+"/fields/"+fieldID.String(), strings.NewReader(`{"value":"P0"}`))
	valueReq.Header.Set("Content-Type", "application/json")
	valueRec := httptest.NewRecorder()
	valueCtx := e.NewContext(valueReq, valueRec)
	valueCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	valueCtx.SetPath("/api/v1/projects/:pid/tasks/:tid/fields/:fid")
	valueCtx.SetParamNames("pid", "tid", "fid")
	valueCtx.SetParamValues(projectID.String(), taskID.String(), fieldID.String())

	if err := h.SetTaskValue(valueCtx); err != nil {
		t.Fatalf("SetTaskValue() error: %v", err)
	}
	if valueRec.Code != http.StatusOK {
		t.Fatalf("value status = %d, want %d", valueRec.Code, http.StatusOK)
	}
	if svc.setValue == nil || svc.setValue.TaskID != taskID || svc.setValue.FieldDefID != fieldID {
		t.Fatalf("unexpected setValue input: %+v", svc.setValue)
	}
}
