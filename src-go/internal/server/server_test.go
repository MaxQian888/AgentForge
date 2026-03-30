package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/server"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

func testConfig() *config.Config {
	return &config.Config{
		Port:            "0",
		JWTSecret:       "test-secret-at-least-32-characters-long",
		JWTAccessTTL:    15 * time.Minute,
		JWTRefreshTTL:   7 * 24 * time.Hour,
		AllowOrigins:    []string{"http://localhost:3000"},
		Env:             "development",
		AgentForgeToken: "test-agentforge-token",
		RolesDir:        "./roles",
	}
}

func registerTestRoutes(e *echo.Echo, cfg *config.Config, authSvc *service.AuthService, cache *repository.CacheRepository) {
	registerTestRoutesWithAgentService(e, cfg, authSvc, cache, nil)
}

func registerTestRoutesWithAgentService(e *echo.Echo, cfg *config.Config, authSvc *service.AuthService, cache *repository.CacheRepository, agentSvc *service.AgentService) {
	server.RegisterRoutes(e, cfg, authSvc, cache,
		repository.NewProjectRepository(nil),
		repository.NewMemberRepository(nil),
		repository.NewSprintRepository(nil),
		repository.NewTaskRepository(nil),
		repository.NewEntityLinkRepository(nil),
		repository.NewTaskCommentRepository(nil),
		repository.NewCustomFieldRepository(nil),
		repository.NewSavedViewRepository(nil),
		repository.NewFormRepository(nil),
		repository.NewAutomationRuleRepository(nil),
		repository.NewAutomationLogRepository(nil),
		repository.NewDashboardRepository(nil),
		repository.NewMilestoneRepository(nil),
		repository.NewTaskProgressRepository(nil),
		repository.NewAgentRunRepository(nil),
		repository.NewAgentPoolQueueRepository(nil),
		repository.NewDispatchAttemptRepository(nil),
		repository.NewNotificationRepository(nil),
		repository.NewReviewRepository(nil),
		repository.NewReviewAggregationRepository(nil),
		repository.NewFalsePositiveRepository(nil),
		repository.NewWorkflowRepository(nil),
		repository.NewAgentTeamRepository(nil),
		repository.NewAgentMemoryRepository(nil),
		repository.NewWikiSpaceRepository(nil),
		repository.NewWikiPageRepository(nil),
		repository.NewPageVersionRepository(nil),
		repository.NewPageCommentRepository(nil),
		repository.NewPageFavoriteRepository(nil),
		repository.NewPageRecentAccessRepository(nil),
		ws.NewHub(),
		bridge.NewClient("http://localhost:7778"),
		nil,
		nil,
		agentSvc,
		nil,
	)
}

func testAgentService() *service.AgentService {
	return service.NewAgentService(
		repository.NewAgentRunRepository(nil),
		repository.NewTaskRepository(nil),
		repository.NewProjectRepository(nil),
		ws.NewHub(),
		bridge.NewClient("http://localhost:7778"),
		nil,
	)
}

func TestNew_Development(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)

	e := server.New(cfg, cache)
	if e == nil {
		t.Fatal("expected non-nil Echo instance")
	}
	if e.Validator == nil {
		t.Error("expected Validator to be set")
	}
}

func TestNew_Production(t *testing.T) {
	cfg := testConfig()
	cfg.Env = "production"
	cache := repository.NewCacheRepository(nil)

	e := server.New(cfg, cache)
	if e == nil {
		t.Fatal("expected non-nil Echo instance")
	}
}

func TestRegisterRoutes_HealthEndpoint(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", rec.Code)
	}

	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestRegisterRoutes_HealthV1Endpoint(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/health: expected 200, got %d", rec.Code)
	}

	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["version"] == "" {
		t.Error("expected non-empty version field")
	}
}

func TestRegisterRoutes_AuthRegister(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	body := `{"email":"test@example.com","password":"password123","name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /api/v1/auth/register: expected 500 or 503, got %d", rec.Code)
	}
}

func TestRegisterRoutes_AuthLogin(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusInternalServerError && rec.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /api/v1/auth/login: expected 401, 500, or 503, got %d", rec.Code)
	}
}

func TestRegisterRoutes_ProtectedEndpointWithoutAuth(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/v1/users/me: expected 401, got %d", rec.Code)
	}
}

func TestRegisterRoutes_TaskDecomposeWithoutAuth(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/00000000-0000-0000-0000-000000000000/decompose", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/v1/tasks/:id/decompose: expected 401, got %d", rec.Code)
	}
}

func TestRegisterRoutes_PluginsWithoutAuth(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/plugins: expected 401 or 404, got %d", rec.Code)
	}
}

func TestRegisterRoutes_PluginControlPlaneCompatibilityRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/plugins/discover":                {},
		http.MethodPut + " /api/v1/plugins/:id/enable":              {},
		http.MethodPut + " /api/v1/plugins/:id/disable":             {},
		http.MethodGet + " /api/v1/plugins/:id/events":              {},
		http.MethodPost + " /api/v1/plugins/:id/mcp/refresh":        {},
		http.MethodPost + " /api/v1/plugins/:id/mcp/tools/call":     {},
		http.MethodPost + " /api/v1/plugins/:id/mcp/resources/read": {},
		http.MethodPost + " /api/v1/plugins/:id/mcp/prompts/get":    {},
		http.MethodPost + " /api/v1/plugins/discover/builtin":       {},
		http.MethodPost + " /api/v1/plugins/:id/enable":             {},
		http.MethodPost + " /api/v1/plugins/:id/disable":            {},
	}

	routes := e.Routes()
	for _, route := range routes {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected plugin compatibility routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_InternalSchedulerRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodGet + " /internal/scheduler/jobs":                  {},
		http.MethodPost + " /internal/scheduler/jobs/:jobKey/trigger": {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected internal scheduler routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_FoundationWorkspaceRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/projects/:pid/fields":                  {},
		http.MethodPost + " /api/v1/projects/:pid/fields":                 {},
		http.MethodPut + " /api/v1/projects/:pid/fields/reorder":          {},
		http.MethodGet + " /api/v1/projects/:pid/views":                   {},
		http.MethodPost + " /api/v1/projects/:pid/views":                  {},
		http.MethodGet + " /api/v1/projects/:pid/forms":                   {},
		http.MethodGet + " /api/v1/forms/:slug":                           {},
		http.MethodPost + " /api/v1/forms/:slug/submit":                   {},
		http.MethodGet + " /api/v1/projects/:pid/automations":             {},
		http.MethodGet + " /api/v1/projects/:pid/automations/logs":        {},
		http.MethodGet + " /api/v1/projects/:pid/dashboards":              {},
		http.MethodGet + " /api/v1/projects/:pid/dashboard/widgets/:type": {},
		http.MethodGet + " /api/v1/projects/:pid/milestones":              {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}
	if len(expected) != 0 {
		t.Fatalf("expected workspace foundation routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_WikiRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/projects/:pid/wiki/pages":                      {},
		http.MethodPost + " /api/v1/projects/:pid/wiki/pages":                     {},
		http.MethodGet + " /api/v1/projects/:pid/wiki/pages/:id":                  {},
		http.MethodPut + " /api/v1/projects/:pid/wiki/pages/:id":                  {},
		http.MethodDelete + " /api/v1/projects/:pid/wiki/pages/:id":               {},
		http.MethodPatch + " /api/v1/projects/:pid/wiki/pages/:id/move":           {},
		http.MethodGet + " /api/v1/projects/:pid/wiki/pages/:id/versions":         {},
		http.MethodPost + " /api/v1/projects/:pid/wiki/pages/:id/versions":        {},
		http.MethodGet + " /api/v1/projects/:pid/wiki/pages/:id/comments":         {},
		http.MethodPost + " /api/v1/projects/:pid/wiki/pages/:id/comments":        {},
		http.MethodPost + " /api/v1/projects/:pid/wiki/pages/:id/decompose-tasks": {},
		http.MethodGet + " /api/v1/projects/:pid/wiki/templates":                  {},
		http.MethodPost + " /api/v1/projects/:pid/wiki/pages/from-template":       {},
		http.MethodGet + " /api/v1/projects/:pid/wiki/favorites":                  {},
		http.MethodPut + " /api/v1/projects/:pid/wiki/pages/:id/favorite":         {},
		http.MethodGet + " /api/v1/projects/:pid/wiki/recent":                     {},
		http.MethodPut + " /api/v1/projects/:pid/wiki/pages/:id/pin":              {},
		http.MethodGet + " /api/v1/wiki/pages/:id":                                {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected wiki routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_TaskLinkingRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodPost + " /api/v1/projects/:pid/links":                      {},
		http.MethodGet + " /api/v1/projects/:pid/links":                       {},
		http.MethodDelete + " /api/v1/projects/:pid/links/:linkId":            {},
		http.MethodGet + " /api/v1/projects/:pid/tasks/:tid/comments":         {},
		http.MethodPost + " /api/v1/projects/:pid/tasks/:tid/comments":        {},
		http.MethodPatch + " /api/v1/projects/:pid/tasks/:tid/comments/:cid":  {},
		http.MethodDelete + " /api/v1/projects/:pid/tasks/:tid/comments/:cid": {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected task linking routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_IMOperatorRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/im/channels":              {},
		http.MethodPost + " /api/v1/im/channels":             {},
		http.MethodPut + " /api/v1/im/channels/:id":          {},
		http.MethodDelete + " /api/v1/im/channels/:id":       {},
		http.MethodGet + " /api/v1/im/bridge/status":         {},
		http.MethodGet + " /api/v1/im/deliveries":            {},
		http.MethodPost + " /api/v1/im/deliveries/:id/retry": {},
		http.MethodGet + " /api/v1/im/event-types":           {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected IM operator routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_DispatcherInfraGapRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutesWithAgentService(e, cfg, authSvc, cache, testAgentService())

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/projects/:pid/queue":             {},
		http.MethodDelete + " /api/v1/projects/:pid/queue/:entryId": {},
		http.MethodGet + " /api/v1/projects/:pid/budget/summary":    {},
		http.MethodGet + " /api/v1/sprints/:sid/budget":             {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected dispatcher infra gap routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_DispatcherInfraGapRoutesRequireAuth(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutesWithAgentService(e, cfg, authSvc, cache, testAgentService())

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "queue list", method: http.MethodGet, path: "/api/v1/projects/" + uuid.NewString() + "/queue"},
		{name: "queue cancel", method: http.MethodDelete, path: "/api/v1/projects/" + uuid.NewString() + "/queue/" + uuid.NewString()},
		{name: "project budget summary", method: http.MethodGet, path: "/api/v1/projects/" + uuid.NewString() + "/budget/summary"},
		{name: "sprint budget detail", method: http.MethodGet, path: "/api/v1/sprints/" + uuid.NewString() + "/budget"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", tc.name, rec.Code)
		}
	}
}

func TestRegisterRoutes_LogoutWithoutAuth(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/v1/auth/logout: expected 401, got %d", rec.Code)
	}
}

func TestRegisterRoutes_WSWithoutToken(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /ws: expected 401, got %d", rec.Code)
	}
}

func TestRegisterRoutes_NotFound(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /nonexistent: expected 404, got %d", rec.Code)
	}
}
