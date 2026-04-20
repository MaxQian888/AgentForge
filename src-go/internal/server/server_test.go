package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/bridge"
	"github.com/agentforge/server/internal/config"
	"github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/server"
	"github.com/agentforge/server/internal/service"
	"github.com/agentforge/server/internal/ws"
	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
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
	registerTestRoutesWithDependencies(e, cfg, authSvc, cache, nil, nil)
}

func registerTestRoutesWithAgentService(e *echo.Echo, cfg *config.Config, authSvc *service.AuthService, cache *repository.CacheRepository, agentSvc *service.AgentService) {
	registerTestRoutesWithDependencies(e, cfg, authSvc, cache, nil, agentSvc)
}

func registerTestRoutesWithDependencies(e *echo.Echo, cfg *config.Config, authSvc *service.AuthService, cache *repository.CacheRepository, pluginSvc *service.PluginService, agentSvc *service.AgentService) {
	server.RegisterRoutes(e, cfg, authSvc, cache,
		repository.NewUserRepository(nil),
		repository.NewProjectRepository(nil),
		repository.NewMemberRepository(nil),
		repository.NewSprintRepository(nil),
		repository.NewTaskRepository(nil),
		repository.NewEntityLinkRepository(nil),
		repository.NewTaskCommentRepository(nil),
		repository.NewIMReactionEventRepository(nil),
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
		repository.NewDocumentRepo(nil),
		repository.NewLogRepository(nil),
		ws.NewHub(),
		eventbus.NewBus(),
		bridge.NewClient("http://localhost:7778"),
		nil,
		pluginSvc,
		agentSvc,
		nil,
	)
}

func signedBearerToken(t *testing.T, secret string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &service.Claims{
		UserID: uuid.NewString(),
		Email:  "role-test@example.com",
		JTI:    uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return "Bearer " + token
}

func testAgentService() *service.AgentService {
	return service.NewAgentService(
		repository.NewAgentRunRepository(nil),
		repository.NewTaskRepository(nil),
		repository.NewProjectRepository(nil),
		ws.NewHub(),
		nil,
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
	redisServer := miniredis.RunT(t)
	cache := repository.NewCacheRepository(redis.NewClient(&redis.Options{Addr: redisServer.Addr()}))
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
	redisServer := miniredis.RunT(t)
	cache := repository.NewCacheRepository(redis.NewClient(&redis.Options{Addr: redisServer.Addr()}))
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

func TestRegisterRoutes_RoleEndpointsUsePluginCatalogForDependencyDiagnostics(t *testing.T) {
	cfg := testConfig()
	cfg.RolesDir = filepath.Join(t.TempDir(), "roles")
	roleDir := filepath.Join(cfg.RolesDir, "design-lead")
	if err := os.MkdirAll(roleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(roleDir, "role.yaml"), []byte(`apiVersion: agentforge/v1
kind: Role
metadata:
  id: design-lead
  name: Design Lead
identity:
  role: Design Lead
  goal: Review design consistency
capabilities:
  tools:
    external:
      - design-mcp
`), 0o600); err != nil {
		t.Fatalf("seed role error = %v", err)
	}

	redisServer := miniredis.RunT(t)
	cache := repository.NewCacheRepository(redis.NewClient(&redis.Options{Addr: redisServer.Addr()}))
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)
	pluginRepo := repository.NewPluginRegistryRepository(nil)
	if err := pluginRepo.Save(context.Background(), &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindTool,
			Metadata: model.PluginMetadata{
				ID:      "design-mcp",
				Name:    "Design MCP",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:   model.PluginRuntimeMCP,
				Transport: "stdio",
				Command:   "node",
				Args:      []string{"tool.js"},
			},
		},
		LifecycleState: model.PluginStateActive,
	}); err != nil {
		t.Fatalf("save plugin record: %v", err)
	}
	pluginSvc := service.NewPluginService(pluginRepo, nil, nil, t.TempDir())

	e := server.New(cfg, cache)
	registerTestRoutesWithDependencies(e, cfg, authSvc, cache, pluginSvc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/roles/design-lead", nil)
	req.Header.Set("Authorization", signedBearerToken(t, cfg.JWTSecret))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/roles/:id status = %d, want 200", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	dependencies, ok := payload["pluginDependencies"].([]any)
	if !ok || len(dependencies) != 1 {
		t.Fatalf("pluginDependencies = %#v, want 1 dependency", payload["pluginDependencies"])
	}
	dependency := dependencies[0].(map[string]any)
	if dependency["pluginId"] != "design-mcp" || dependency["status"] != "active" {
		t.Fatalf("dependency = %#v, want active design-mcp", dependency)
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

	// Wiki routes have been replaced by unified /knowledge/assets routes.
	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/projects/:pid/knowledge/assets":                      {},
		http.MethodPost + " /api/v1/projects/:pid/knowledge/assets":                     {},
		http.MethodGet + " /api/v1/projects/:pid/knowledge/assets/:id":                  {},
		http.MethodPut + " /api/v1/projects/:pid/knowledge/assets/:id":                  {},
		http.MethodDelete + " /api/v1/projects/:pid/knowledge/assets/:id":               {},
		http.MethodPatch + " /api/v1/projects/:pid/knowledge/assets/:id/move":           {},
		http.MethodGet + " /api/v1/projects/:pid/knowledge/assets/:id/versions":         {},
		http.MethodPost + " /api/v1/projects/:pid/knowledge/assets/:id/versions":        {},
		http.MethodGet + " /api/v1/projects/:pid/knowledge/assets/:id/comments":         {},
		http.MethodPost + " /api/v1/projects/:pid/knowledge/assets/:id/comments":        {},
		http.MethodPost + " /api/v1/projects/:pid/knowledge/assets/:id/decompose-tasks": {},
		http.MethodGet + " /api/v1/projects/:pid/knowledge/assets/tree":                 {},
		http.MethodGet + " /api/v1/projects/:pid/knowledge/search":                      {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected knowledge asset routes to be registered, missing: %+v", expected)
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
		http.MethodGet + " /api/v1/im/channels":                {},
		http.MethodPost + " /api/v1/im/channels":               {},
		http.MethodPut + " /api/v1/im/channels/:id":            {},
		http.MethodDelete + " /api/v1/im/channels/:id":         {},
		http.MethodGet + " /api/v1/im/bridge/status":           {},
		http.MethodGet + " /api/v1/im/deliveries":              {},
		http.MethodPost + " /api/v1/im/deliveries/:id/retry":   {},
		http.MethodPost + " /api/v1/im/deliveries/retry-batch": {},
		http.MethodPost + " /api/v1/im/test-send":              {},
		http.MethodGet + " /api/v1/im/event-types":             {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected IM operator routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_WorkflowTemplateRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodGet + " /api/v1/workflow-templates":                {},
		http.MethodPost + " /api/v1/workflows/:id/publish-template":   {},
		http.MethodPost + " /api/v1/workflow-templates/:id/duplicate": {},
		http.MethodPost + " /api/v1/workflow-templates/:id/clone":     {},
		http.MethodPost + " /api/v1/workflow-templates/:id/execute":   {},
		http.MethodDelete + " /api/v1/workflow-templates/:id":         {},
		http.MethodGet + " /api/v1/projects/:pid/workflow-reviews":    {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected workflow template routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_MemoryRoutesPresent(t *testing.T) {
	cfg := testConfig()
	cache := repository.NewCacheRepository(nil)
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	expected := map[string]struct{}{
		http.MethodPost + " /api/v1/projects/:pid/memory":             {},
		http.MethodGet + " /api/v1/projects/:pid/memory":              {},
		http.MethodGet + " /api/v1/projects/:pid/memory/stats":        {},
		http.MethodGet + " /api/v1/projects/:pid/memory/export":       {},
		http.MethodPost + " /api/v1/projects/:pid/memory/bulk-delete": {},
		http.MethodPost + " /api/v1/projects/:pid/memory/cleanup":     {},
		http.MethodGet + " /api/v1/projects/:pid/memory/:mid":         {},
		http.MethodPatch + " /api/v1/projects/:pid/memory/:mid":       {},
		http.MethodDelete + " /api/v1/projects/:pid/memory/:mid":      {},
	}

	for _, route := range e.Routes() {
		delete(expected, route.Method+" "+route.Path)
	}

	if len(expected) != 0 {
		t.Fatalf("expected memory routes to be registered, missing: %+v", expected)
	}
}

func TestRegisterRoutes_IMTestSendUsesConfiguredSenderWiring(t *testing.T) {
	cfg := testConfig()
	redisServer := miniredis.RunT(t)
	cache := repository.NewCacheRepository(redis.NewClient(&redis.Options{Addr: redisServer.Addr()}))
	userRepo := repository.NewUserRepository(nil)
	authSvc := service.NewAuthService(userRepo, cache, cfg)

	e := server.New(cfg, cache)
	registerTestRoutes(e, cfg, authSvc, cache)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/im/test-send", strings.NewReader(`{"platform":"slack","channelId":"C123","text":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", signedBearerToken(t, cfg.JWTSecret))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/im/test-send status = %d, want 200", rec.Code)
	}

	var payload model.IMTestSendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.DeliveryID == "" {
		t.Fatalf("delivery id = %q, want non-empty", payload.DeliveryID)
	}
	if payload.Status != model.IMDeliveryStatusFailed {
		t.Fatalf("status = %q, want failed when no live notify target is available", payload.Status)
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
