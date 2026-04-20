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

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type memoryHandlerValidator struct {
	validator *validator.Validate
}

func (v *memoryHandlerValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type memoryServiceStub struct {
	storeInput   *service.StoreMemoryInput
	searchInput  *service.MemoryExplorerQuery
	getProjectID uuid.UUID
	getMemoryID  uuid.UUID
	getRoleID    string
	bulkProject  uuid.UUID
	bulkIDs      []uuid.UUID
	bulkRoleID   string
	cleanupInput *service.MemoryCleanupInput
	deleteID     uuid.UUID

	storeResult  *model.AgentMemory
	searchResult []model.AgentMemoryDTO
	detailResult *model.AgentMemoryDetailDTO
	statsResult  *model.MemoryExplorerStatsDTO
	exportResult *service.EpisodicMemoryExport
	bulkDeleted  int64
	cleanupCount int64

	storeErr   error
	searchErr  error
	getErr     error
	statsErr   error
	exportErr  error
	bulkErr    error
	cleanupErr error
	deleteErr  error
}

func (s *memoryServiceStub) Store(_ context.Context, input service.StoreMemoryInput) (*model.AgentMemory, error) {
	s.storeInput = &input
	return s.storeResult, s.storeErr
}

func (s *memoryServiceStub) Update(_ context.Context, input service.UpdateMemoryInput) (*model.AgentMemory, error) {
	s.storeInput = &service.StoreMemoryInput{
		ProjectID: input.ProjectID,
		RoleID:    input.RoleID,
	}
	if input.Key != nil {
		s.storeInput.Key = *input.Key
	}
	if input.Content != nil {
		s.storeInput.Content = *input.Content
	}
	if input.Tags != nil {
		s.storeInput.Tags = append([]string(nil), (*input.Tags)...)
	}
	return s.storeResult, s.storeErr
}

func (s *memoryServiceStub) Search(_ context.Context, input service.MemoryExplorerQuery) ([]model.AgentMemoryDTO, error) {
	s.searchInput = &input
	return s.searchResult, s.searchErr
}

func (s *memoryServiceStub) Get(_ context.Context, projectID uuid.UUID, id uuid.UUID, roleID string) (*model.AgentMemoryDetailDTO, error) {
	s.getProjectID = projectID
	s.getMemoryID = id
	s.getRoleID = roleID
	return s.detailResult, s.getErr
}

func (s *memoryServiceStub) Stats(_ context.Context, input service.MemoryExplorerQuery) (*model.MemoryExplorerStatsDTO, error) {
	s.searchInput = &input
	return s.statsResult, s.statsErr
}

func (s *memoryServiceStub) ExportEpisodic(_ context.Context, input service.MemoryExplorerQuery) (*service.EpisodicMemoryExport, error) {
	s.searchInput = &input
	return s.exportResult, s.exportErr
}

func (s *memoryServiceStub) BulkDelete(_ context.Context, projectID uuid.UUID, ids []uuid.UUID, roleID string) (int64, error) {
	s.bulkProject = projectID
	s.bulkIDs = append([]uuid.UUID(nil), ids...)
	s.bulkRoleID = roleID
	return s.bulkDeleted, s.bulkErr
}

func (s *memoryServiceStub) CleanupEpisodic(_ context.Context, input service.MemoryCleanupInput) (int64, error) {
	s.cleanupInput = &input
	return s.cleanupCount, s.cleanupErr
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
	start := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
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
			CreatedAt:      start,
			UpdatedAt:      end,
		},
		searchResult: []model.AgentMemoryDTO{{
			ID:        memoryID.String(),
			ProjectID: projectID.String(),
			Scope:     model.MemoryScopeProject,
			Key:       "release-plan",
			Content:   "Coordinate deployment in phases",
			CreatedAt: start.Format(time.RFC3339),
			UpdatedAt: end.Format(time.RFC3339),
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

	_, searchCtx, searchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?query=release&scope=project&category=semantic&roleId=planner&startAt="+start.Format(time.RFC3339)+"&endAt="+end.Format(time.RFC3339)+"&limit=5", "")
	searchCtx.SetPath("/api/v1/projects/:pid/memory")
	searchCtx.SetParamNames("pid")
	searchCtx.SetParamValues(projectID.String())
	if err := h.Search(searchCtx); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if searchRec.Code != http.StatusOK || stub.searchInput == nil {
		t.Fatalf("Search() status/input = %d / %#v", searchRec.Code, stub.searchInput)
	}
	if stub.searchInput.Query != "release" || stub.searchInput.Scope != model.MemoryScopeProject || stub.searchInput.Category != model.MemoryCategorySemantic || stub.searchInput.RoleID != "planner" || stub.searchInput.Limit != 5 {
		t.Fatalf("searchInput = %#v", stub.searchInput)
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

func TestMemoryHandlerExplorerRoutes(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	stub := &memoryServiceStub{
		detailResult: &model.AgentMemoryDetailDTO{AgentMemoryDTO: model.AgentMemoryDTO{ID: memoryID.String(), ProjectID: projectID.String(), Key: "release-plan", CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339)}},
		statsResult:  &model.MemoryExplorerStatsDTO{TotalCount: 3},
		exportResult: &service.EpisodicMemoryExport{ProjectID: projectID.String()},
		bulkDeleted:  2,
		cleanupCount: 5,
	}
	h := handler.NewMemoryHandler(stub)

	_, getCtx, getRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/"+memoryID.String()+"?roleId=planner", "")
	getCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
	getCtx.SetParamNames("pid", "mid")
	getCtx.SetParamValues(projectID.String(), memoryID.String())
	if err := h.Get(getCtx); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if getRec.Code != http.StatusOK || stub.getProjectID != projectID || stub.getMemoryID != memoryID || stub.getRoleID != "planner" {
		t.Fatalf("Get() status/captured = %d / %s / %s / %q", getRec.Code, stub.getProjectID, stub.getMemoryID, stub.getRoleID)
	}

	_, statsCtx, statsRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/stats?category=semantic", "")
	statsCtx.SetPath("/api/v1/projects/:pid/memory/stats")
	statsCtx.SetParamNames("pid")
	statsCtx.SetParamValues(projectID.String())
	if err := h.Stats(statsCtx); err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if statsRec.Code != http.StatusOK || stub.searchInput == nil || stub.searchInput.Category != model.MemoryCategorySemantic {
		t.Fatalf("Stats() status/input = %d / %#v", statsRec.Code, stub.searchInput)
	}

	_, exportCtx, exportRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/export?scope=role&roleId=planner", "")
	exportCtx.SetPath("/api/v1/projects/:pid/memory/export")
	exportCtx.SetParamNames("pid")
	exportCtx.SetParamValues(projectID.String())
	if err := h.Export(exportCtx); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if exportRec.Code != http.StatusOK || stub.searchInput == nil || stub.searchInput.Scope != model.MemoryScopeRole || stub.searchInput.RoleID != "planner" {
		t.Fatalf("Export() status/input = %d / %#v", exportRec.Code, stub.searchInput)
	}

	_, bulkCtx, bulkRec := newMemoryHandlerContext(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/memory/bulk-delete", `{"ids":["`+memoryID.String()+`"],"roleId":"planner"}`)
	bulkCtx.SetPath("/api/v1/projects/:pid/memory/bulk-delete")
	bulkCtx.SetParamNames("pid")
	bulkCtx.SetParamValues(projectID.String())
	if err := h.BulkDelete(bulkCtx); err != nil {
		t.Fatalf("BulkDelete() error = %v", err)
	}
	if bulkRec.Code != http.StatusOK || stub.bulkProject != projectID || len(stub.bulkIDs) != 1 || stub.bulkRoleID != "planner" {
		t.Fatalf("BulkDelete() status/input = %d / %s / %#v / %q", bulkRec.Code, stub.bulkProject, stub.bulkIDs, stub.bulkRoleID)
	}

	_, cleanupCtx, cleanupRec := newMemoryHandlerContext(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/memory/cleanup", `{"scope":"role","roleId":"planner","before":"`+now.Format(time.RFC3339)+`"}`)
	cleanupCtx.SetPath("/api/v1/projects/:pid/memory/cleanup")
	cleanupCtx.SetParamNames("pid")
	cleanupCtx.SetParamValues(projectID.String())
	if err := h.Cleanup(cleanupCtx); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if cleanupRec.Code != http.StatusOK || stub.cleanupInput == nil || stub.cleanupInput.Scope != model.MemoryScopeRole || stub.cleanupInput.RoleID != "planner" || stub.cleanupInput.Before == nil {
		t.Fatalf("Cleanup() status/input = %d / %#v", cleanupRec.Code, stub.cleanupInput)
	}

	stub.storeResult = &model.AgentMemory{
		ID:        memoryID,
		ProjectID: projectID,
		Scope:     model.MemoryScopeProject,
		Category:  model.MemoryCategoryEpisodic,
		Key:       "release-note",
		Content:   "Pinned release detail",
		Metadata:  `{"kind":"operator_note","tags":["ops"]}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, updateCtx, updateRec := newMemoryHandlerContext(http.MethodPatch, "/api/v1/projects/"+projectID.String()+"/memory/"+memoryID.String()+"?roleId=planner", `{"key":"release-note","content":"Pinned release detail","tags":["ops","release"]}`)
	updateCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
	updateCtx.SetParamNames("pid", "mid")
	updateCtx.SetParamValues(projectID.String(), memoryID.String())
	if err := h.Update(updateCtx); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updateRec.Code != http.StatusOK || stub.storeInput == nil || stub.storeInput.Key != "release-note" || stub.storeInput.Content != "Pinned release detail" || len(stub.storeInput.Tags) != 2 {
		t.Fatalf("Update() status/input = %d / %#v", updateRec.Code, stub.storeInput)
	}
}

func TestMemoryHandlerSearchSupportsLegacyQAndErrorBranches(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()
	stub := &memoryServiceStub{searchResult: []model.AgentMemoryDTO{}, getErr: repository.ErrNotFound, cleanupErr: errors.New("cleanup before or retentionDays is required")}
	h := handler.NewMemoryHandler(stub)

	_, searchCtx, searchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?q=release", "")
	searchCtx.SetPath("/api/v1/projects/:pid/memory")
	searchCtx.SetParamNames("pid")
	searchCtx.SetParamValues(projectID.String())
	if err := h.Search(searchCtx); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if searchRec.Code != http.StatusOK || stub.searchInput == nil || stub.searchInput.Query != "release" {
		t.Fatalf("legacy search input = %#v status=%d", stub.searchInput, searchRec.Code)
	}

	_, taggedSearchCtx, taggedSearchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?query=release&tag=ops", "")
	taggedSearchCtx.SetPath("/api/v1/projects/:pid/memory")
	taggedSearchCtx.SetParamNames("pid")
	taggedSearchCtx.SetParamValues(projectID.String())
	if err := h.Search(taggedSearchCtx); err != nil {
		t.Fatalf("Search(tagged) error = %v", err)
	}
	if taggedSearchRec.Code != http.StatusOK || stub.searchInput == nil || stub.searchInput.Tag != "ops" {
		t.Fatalf("tagged search input = %#v status=%d", stub.searchInput, taggedSearchRec.Code)
	}

	_, badSearchCtx, badSearchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?startAt=bad", "")
	badSearchCtx.SetPath("/api/v1/projects/:pid/memory")
	badSearchCtx.SetParamNames("pid")
	badSearchCtx.SetParamValues(projectID.String())
	if err := h.Search(badSearchCtx); err != nil {
		t.Fatalf("Search(bad) error = %v", err)
	}
	if badSearchRec.Code != http.StatusBadRequest {
		t.Fatalf("Search(bad) status = %d, want 400", badSearchRec.Code)
	}

	_, getCtx, getRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/"+memoryID.String(), "")
	getCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
	getCtx.SetParamNames("pid", "mid")
	getCtx.SetParamValues(projectID.String(), memoryID.String())
	if err := h.Get(getCtx); err != nil {
		t.Fatalf("Get(notfound) error = %v", err)
	}
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("Get(notfound) status = %d, want 404", getRec.Code)
	}

	_, cleanupCtx, cleanupRec := newMemoryHandlerContext(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/memory/cleanup", `{}`)
	cleanupCtx.SetPath("/api/v1/projects/:pid/memory/cleanup")
	cleanupCtx.SetParamNames("pid")
	cleanupCtx.SetParamValues(projectID.String())
	if err := h.Cleanup(cleanupCtx); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if cleanupRec.Code != http.StatusBadRequest {
		t.Fatalf("Cleanup() status = %d, want 400", cleanupRec.Code)
	}
}

func TestMemoryHandlerRoleScopedExplorerQueriesRequireRoleID(t *testing.T) {
	projectID := uuid.New()
	stub := &memoryServiceStub{}
	h := handler.NewMemoryHandler(stub)

	_, searchCtx, searchRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory?scope=role", "")
	searchCtx.SetPath("/api/v1/projects/:pid/memory")
	searchCtx.SetParamNames("pid")
	searchCtx.SetParamValues(projectID.String())
	if err := h.Search(searchCtx); err != nil {
		t.Fatalf("Search(role scope) error = %v", err)
	}
	if searchRec.Code != http.StatusBadRequest {
		t.Fatalf("Search(role scope) status = %d, want 400", searchRec.Code)
	}

	_, statsCtx, statsRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/stats?scope=role", "")
	statsCtx.SetPath("/api/v1/projects/:pid/memory/stats")
	statsCtx.SetParamNames("pid")
	statsCtx.SetParamValues(projectID.String())
	if err := h.Stats(statsCtx); err != nil {
		t.Fatalf("Stats(role scope) error = %v", err)
	}
	if statsRec.Code != http.StatusBadRequest {
		t.Fatalf("Stats(role scope) status = %d, want 400", statsRec.Code)
	}

	_, exportCtx, exportRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/export?scope=role", "")
	exportCtx.SetPath("/api/v1/projects/:pid/memory/export")
	exportCtx.SetParamNames("pid")
	exportCtx.SetParamValues(projectID.String())
	if err := h.Export(exportCtx); err != nil {
		t.Fatalf("Export(role scope) error = %v", err)
	}
	if exportRec.Code != http.StatusBadRequest {
		t.Fatalf("Export(role scope) status = %d, want 400", exportRec.Code)
	}

	if stub.searchInput != nil {
		t.Fatalf("service should not be called for malformed role-scoped explorer query: %#v", stub.searchInput)
	}
}

func TestMemoryHandlerServiceUnavailableAndAccessDenied(t *testing.T) {
	projectID := uuid.New()
	memoryID := uuid.New()

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

	stub := &memoryServiceStub{getErr: service.ErrMemoryAccessDenied}
	h = handler.NewMemoryHandler(stub)
	_, getCtx, getRec := newMemoryHandlerContext(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/memory/"+memoryID.String(), "")
	getCtx.SetPath("/api/v1/projects/:pid/memory/:mid")
	getCtx.SetParamNames("pid", "mid")
	getCtx.SetParamValues(projectID.String(), memoryID.String())
	if err := h.Get(getCtx); err != nil {
		t.Fatalf("Get(access denied) error = %v", err)
	}
	if getRec.Code != http.StatusForbidden {
		t.Fatalf("Get(access denied) status = %d, want 403", getRec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["message"] == "" {
		t.Fatalf("payload = %#v", payload)
	}
}
