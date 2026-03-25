package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	server.RegisterRoutes(e, cfg, authSvc, cache,
		repository.NewProjectRepository(nil),
		repository.NewMemberRepository(nil),
		repository.NewSprintRepository(nil),
		repository.NewTaskRepository(nil),
		repository.NewTaskProgressRepository(nil),
		repository.NewAgentRunRepository(nil),
		repository.NewNotificationRepository(nil),
		repository.NewReviewRepository(nil),
		repository.NewWorkflowRepository(nil),
		repository.NewAgentTeamRepository(nil),
		repository.NewAgentMemoryRepository(nil),
		ws.NewHub(),
		bridge.NewClient("http://localhost:7778"),
		nil,
		nil,
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
