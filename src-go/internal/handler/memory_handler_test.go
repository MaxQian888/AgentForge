package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type memoryHandlerValidator struct {
	validator *validator.Validate
}

func (v *memoryHandlerValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type memoryServiceStub struct {
	storeInput  *service.StoreMemoryInput
	searchPID   uuid.UUID
	searchQuery string
	searchLimit int
	deleteID    uuid.UUID

	storeResult  *model.AgentMemory
	searchResult []model.AgentMemoryDTO

	storeErr  error
	searchErr error
	deleteErr error
}

func (s *memoryServiceStub) Store(_ context.Context, input service.StoreMemoryInput) (*model.AgentMemory, error) {
	s.storeInput = &input
	return s.storeResult, s.storeErr
}

func (s *memoryServiceStub) Search(_ context.Context, projectID uuid.UUID, query string, limit int) ([]model.AgentMemoryDTO, error) {
	s.searchPID = projectID
	s.searchQuery = query
	s.searchLimit = limit
	return s.searchResult, s.searchErr
}

func (s *memoryServiceStub) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteID = id
	return s.deleteErr
}

func newMemoryHandlerContext(method, target, body string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = &memoryHandlerValidator{validator: validator.New()}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e, e.NewContext(req, rec), rec
}

func TestMemoryHandlerStoreSearchAndDelete(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	now := time.Date(2026, 3, 30, 18, 0, 0, 0, time.UTC)
	stub := &memoryServiceStub{
		storeResult: &model.AgentMemory{
			ID:             memoryID,
			ProjectID:      projectID,
			Scope:          model.MemoryScopeProject,
			RoleID:         "planner",
			Category:       model.MemoryCategorySemantic,
			Key:            "release-plan",
			Content:        "Coordinate deployment in phases",
			Metadata:       `{"source":"ops"}`,
			RelevanceScore: 0.8,
			AccessCount:    1,
			CreatedAt:      now,
		},
		searchResult: []model.AgentMemoryDTO{{
			ID:        memoryID.String(),
			ProjectID: projectID.String(),
			Scope:     model.MemoryScopeProject,
			Key:       "release-plan",
			Content:   "Coordinate deployment in phases",
			CreatedAt: now.Format(time.RFC3339),
		}},
	}
	h := handler.NewMemoryHandler(stub)

	_, storeCtx, storeRec := newMemoryHandlerContext(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/memory", `{"scope":"project","roleId":"planner","category":"semantic","key":"release-plan","content":"Coordinate deployment in phases","metadata":"{\"source\":\"ops\"}","relevanceScore":0.8}`)
	storeCtx.SetPath("/api/v1/projects/:pid/memory")
	storeCtx.SetParamNames("pid")
	storeCtx.SetParamValues(projectID.String())
	if err := h.Store(storeCtx); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if storeRec.Code != http.StatusCreated || stub.storeInput == nil || stub.storeInput.Key != "release-plan" {
		t.Fatalf("Store() status/input = %d / %#v", storeRec.Code, stub.storeInput)
	}

	_, searchCtx, searchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?q=release&limit=5", "")
	searchCtx.SetPath("/api/v1/projects/:pid/memory")
	searchCtx.SetParamNames("pid")
	searchCtx.SetParamValues(projectID.String())
	searchCtx.QueryParams().Set("q", "release")
	searchCtx.QueryParams().Set("limit", "5")
	if err := h.Search(searchCtx); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if searchRec.Code != http.StatusOK || stub.searchPID != projectID || stub.searchQuery != "release" || stub.searchLimit != 5 {
		t.Fatalf("Search() status/input = %d / %s / %q / %d", searchRec.Code, stub.searchPID, stub.searchQuery, stub.searchLimit)
	}

	_, deleteCtx, deleteRec := newMemoryHandlerContext(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/memory/"+memoryID.String(), "")
	deleteCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
	deleteCtx.SetParamNames("pid", "mid")
	deleteCtx.SetParamValues(projectID.String(), memoryID.String())
	if err := h.Delete(deleteCtx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleteRec.Code != http.StatusNoContent || stub.deleteID != memoryID {
		t.Fatalf("Delete() status/input = %d / %s", deleteRec.Code, stub.deleteID)
	}
}

func TestMemoryHandlerErrorBranches(t *testing.T) {
	projectID := uuid.New()

	t.Run("service unavailable", func(t *testing.T) {
		h := handler.NewMemoryHandler(nil)
		_, ctx, rec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory", "")
		ctx.SetPath("/api/v1/projects/:pid/memory")
		ctx.SetParamNames("pid")
		ctx.SetParamValues(projectID.String())
		if err := h.Search(ctx); err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
	})

	t.Run("invalid project id", func(t *testing.T) {
		h := handler.NewMemoryHandler(&memoryServiceStub{})
		_, ctx, rec := newMemoryHandlerContext(http.MethodPost, "/api/v1/projects/bad/memory", `{}`)
		ctx.SetPath("/api/v1/projects/:pid/memory")
		ctx.SetParamNames("pid")
		ctx.SetParamValues("bad")
		if err := h.Store(ctx); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		h := handler.NewMemoryHandler(&memoryServiceStub{})
		_, ctx, rec := newMemoryHandlerContext(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/memory", `{}`)
		ctx.SetPath("/api/v1/projects/:pid/memory")
		ctx.SetParamNames("pid")
		ctx.SetParamValues(projectID.String())
		if err := h.Store(ctx); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rec.Code)
		}
	})

	t.Run("internal errors", func(t *testing.T) {
		stub := &memoryServiceStub{
			searchErr: errors.New("search failed"),
		}
		h := handler.NewMemoryHandler(stub)

		_, searchCtx, searchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory", "")
		searchCtx.SetPath("/api/v1/projects/:pid/memory")
		searchCtx.SetParamNames("pid")
		searchCtx.SetParamValues(projectID.String())
		if err := h.Search(searchCtx); err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if searchRec.Code != http.StatusInternalServerError {
			t.Fatalf("search status = %d, want 500", searchRec.Code)
		}

		_, deleteCtx, deleteRec := newMemoryHandlerContext(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/memory/not-a-uuid", "")
		deleteCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
		deleteCtx.SetParamNames("pid", "mid")
		deleteCtx.SetParamValues(projectID.String(), "not-a-uuid")
		if err := h.Delete(deleteCtx); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleteRec.Code != http.StatusBadRequest {
			t.Fatalf("delete invalid mid status = %d, want 400", deleteRec.Code)
		}
	})
}

func TestMemoryHandlerDeleteServiceFailureAndDefaultLimit(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	stub := &memoryServiceStub{
		deleteErr:    errors.New("delete failed"),
		searchResult: []model.AgentMemoryDTO{},
	}
	h := handler.NewMemoryHandler(stub)

	_, searchCtx, searchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?limit=0", "")
	searchCtx.SetPath("/api/v1/projects/:pid/memory")
	searchCtx.SetParamNames("pid")
	searchCtx.SetParamValues(projectID.String())
	searchCtx.QueryParams().Set("limit", "0")
	if err := h.Search(searchCtx); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if searchRec.Code != http.StatusOK || stub.searchLimit != 20 {
		t.Fatalf("Search() status/limit = %d / %d", searchRec.Code, stub.searchLimit)
	}

	_, deleteCtx, deleteRec := newMemoryHandlerContext(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/memory/"+memoryID.String(), "")
	deleteCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
	deleteCtx.SetParamNames("pid", "mid")
	deleteCtx.SetParamValues(projectID.String(), memoryID.String())
	if err := h.Delete(deleteCtx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleteRec.Code != http.StatusInternalServerError {
		t.Fatalf("Delete() status = %d, want 500", deleteRec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(deleteRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["message"] == "" {
		t.Fatalf("payload = %#v", payload)
	}
}
