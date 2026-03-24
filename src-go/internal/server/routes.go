package server

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	pluginruntime "github.com/react-go-quick-starter/server/internal/plugin"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/version"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type taskDecompositionBridgeAdapter struct {
	client *bridge.Client
}

func (a taskDecompositionBridgeAdapter) DecomposeTask(ctx context.Context, req service.BridgeDecomposeRequest) (*service.BridgeDecomposeResponse, error) {
	resp, err := a.client.DecomposeTask(ctx, bridge.DecomposeRequest{
		TaskID:      req.TaskID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Provider:    req.Provider,
		Model:       req.Model,
	})
	if err != nil {
		return nil, err
	}
	subtasks := make([]service.BridgeDecomposeSubtask, 0, len(resp.Subtasks))
	for _, item := range resp.Subtasks {
		subtasks = append(subtasks, service.BridgeDecomposeSubtask{
			Title:       item.Title,
			Description: item.Description,
			Priority:    item.Priority,
		})
	}
	return &service.BridgeDecomposeResponse{
		Summary:  resp.Summary,
		Subtasks: subtasks,
	}, nil
}

func RegisterRoutes(
	e *echo.Echo,
	cfg *config.Config,
	authSvc *service.AuthService,
	cache *repository.CacheRepository,
	projectRepo *repository.ProjectRepository,
	memberRepo *repository.MemberRepository,
	sprintRepo *repository.SprintRepository,
	taskRepo *repository.TaskRepository,
	taskProgressRepo *repository.TaskProgressRepository,
	agentRunRepo *repository.AgentRunRepository,
	notifRepo *repository.NotificationRepository,
	reviewRepo *repository.ReviewRepository,
	hub *ws.Hub,
	bridgeClient *bridge.Client,
	agentSvc *service.AgentService,
) *service.TaskProgressService {
	jwtMw := appMiddleware.JWTMiddleware(cfg.JWTSecret, cache)
	reviewTriggerMw := appMiddleware.ReviewTriggerAuthMiddleware(cfg.JWTSecret, cache, cfg.AgentForgeToken)
	notificationSvc := service.NewNotificationService(notifRepo, hub)
	taskProgressSvc := service.NewTaskProgressService(
		taskRepo,
		taskProgressRepo,
		notificationSvc,
		hub,
		service.TaskProgressConfig{
			WarningAfter:     cfg.TaskProgressWarningAfter,
			StalledAfter:     cfg.TaskProgressStalledAfter,
			AlertCooldown:    cfg.TaskProgressAlertCooldown,
			DetectorInterval: cfg.TaskProgressDetectorInterval,
			ExemptStatuses:   cfg.TaskProgressExemptStatuses,
		},
		func() time.Time { return time.Now().UTC() },
	)
	if imNotifier := service.NewHTTPTaskProgressIMNotifier(cfg.IMNotifyURL, cfg.IMNotifyPlatform, cfg.IMNotifyTargetChatID); imNotifier != nil {
		taskProgressSvc.SetIMNotifier(imNotifier)
	}
	if agentSvc != nil {
		agentSvc.SetProgressTracker(taskProgressSvc)
	}
	reviewSvc := service.NewReviewService(reviewRepo, taskRepo, notificationSvc, hub, bridgeClient, taskProgressSvc)
	taskDecomposeSvc := service.NewTaskDecompositionService(taskRepo, taskDecompositionBridgeAdapter{client: bridgeClient})

	// Health
	healthH := handler.NewHealthHandler(version.Version, version.Commit, version.BuildDate, cfg.Env)
	e.GET("/health", healthH.Health)

	// v1 group
	v1 := e.Group("/api/v1")
	v1.GET("/health", healthH.HealthV1)

	// Auth routes (public, rate-limited)
	authH := handler.NewAuthHandler(authSvc, cfg.JWTAccessTTL)
	authRateLimiter := echomiddleware.RateLimiter(echomiddleware.NewRateLimiterMemoryStore(20))
	auth := v1.Group("/auth")
	auth.POST("/register", authH.Register, authRateLimiter)
	auth.POST("/login", authH.Login, authRateLimiter)
	auth.POST("/refresh", authH.Refresh, authRateLimiter)
	auth.POST("/logout", authH.Logout, jwtMw)

	// User routes (protected)
	users := v1.Group("/users", jwtMw)
	users.GET("/me", authH.GetMe)

	// WebSocket
	wsH := ws.NewHandler(hub, cfg.JWTSecret)
	e.GET("/ws", wsH.HandleWS)

	// --- New resource handlers ---
	projectH := handler.NewProjectHandler(projectRepo)
	memberH := handler.NewMemberHandler(memberRepo)
	sprintH := handler.NewSprintHandler(sprintRepo)
	taskH := handler.NewTaskHandler(taskRepo, taskDecomposeSvc).WithProgress(taskProgressSvc)
	var agentRuntime handler.AgentRuntimeService
	if agentSvc != nil {
		agentRuntime = agentSvc
	}
	agentH := handler.NewAgentHandler(agentRuntime)
	notifH := handler.NewNotificationHandler(notifRepo)
	costH := handler.NewCostHandler(agentRunRepo)
	roleH := handler.NewRoleHandler(cfg.RolesDir)
	reviewH := handler.NewReviewHandler(reviewSvc)
	pluginSvc := service.NewPluginService(
		repository.NewPluginRegistryRepository(),
		bridgeClient,
		pluginruntime.NewWASMRuntimeManager(),
		cfg.PluginsDir,
	)
	pluginH := handler.NewPluginHandler(pluginSvc)
	if agentSvc != nil {
		dispatchSvc := service.NewTaskDispatchService(taskRepo, memberRepo, agentSvc, hub, notificationSvc, taskProgressSvc)
		taskH = taskH.WithDispatcher(dispatchSvc)
		agentH = agentH.WithDispatcher(dispatchSvc)
	}

	// JWT protected routes
	protected := v1.Group("", jwtMw)
	v1.POST("/reviews/trigger", reviewH.Trigger, reviewTriggerMw)

	// Projects
	protected.POST("/projects", projectH.Create)
	protected.GET("/projects", projectH.List)
	protected.GET("/projects/:id", projectH.Get)
	protected.PUT("/projects/:id", projectH.Update)

	// Project-scoped routes
	projectMw := appMiddleware.ProjectMiddleware(projectRepo)
	projectGroup := protected.Group("/projects/:pid", projectMw)
	projectGroup.POST("/members", memberH.Create)
	projectGroup.GET("/members", memberH.List)
	projectGroup.POST("/tasks", taskH.Create)
	projectGroup.GET("/tasks", taskH.List)
	projectGroup.POST("/sprints", sprintH.Create)
	projectGroup.GET("/sprints", sprintH.List)

	// Task operations (not project-scoped, task ID is unique)
	protected.GET("/tasks/:id", taskH.Get)
	protected.PUT("/tasks/:id", taskH.Update)
	protected.DELETE("/tasks/:id", taskH.Delete)
	protected.POST("/tasks/:id/transition", taskH.Transition)
	protected.POST("/tasks/:id/assign", taskH.Assign)
	protected.POST("/tasks/:id/decompose", taskH.Decompose)

	// Member update
	protected.PUT("/members/:id", memberH.Update)

	// Agents
	protected.POST("/agents/spawn", agentH.Spawn)
	protected.GET("/agents", agentH.List)
	protected.GET("/agents/:id", agentH.Get)
	protected.POST("/agents/:id/pause", agentH.Pause)
	protected.POST("/agents/:id/resume", agentH.Resume)
	protected.POST("/agents/:id/kill", agentH.Kill)

	// Notifications
	protected.GET("/notifications", notifH.List)
	protected.PUT("/notifications/:id/read", notifH.MarkRead)
	protected.GET("/reviews/:id", reviewH.Get)
	protected.GET("/tasks/:taskId/reviews", reviewH.ListByTask)
	protected.POST("/reviews/:id/complete", reviewH.Complete)

	// Cost
	protected.GET("/stats/cost", costH.GetStats)

	// Roles
	protected.GET("/roles", roleH.List)
	protected.GET("/roles/:id", roleH.Get)
	protected.POST("/roles", roleH.Create)
	protected.PUT("/roles/:id", roleH.Update)

	// Plugins
	protected.POST("/plugins/discover/builtin", pluginH.DiscoverBuiltIns)
	protected.POST("/plugins/install", pluginH.InstallLocal)
	protected.GET("/plugins", pluginH.List)
	protected.POST("/plugins/:id/enable", pluginH.Enable)
	protected.POST("/plugins/:id/disable", pluginH.Disable)
	protected.POST("/plugins/:id/activate", pluginH.Activate)
	protected.GET("/plugins/:id/health", pluginH.Health)
	protected.POST("/plugins/:id/restart", pluginH.Restart)
	protected.POST("/plugins/:id/invoke", pluginH.Invoke)

	// Bridge-to-registry runtime sync
	e.POST("/internal/plugins/runtime-state", pluginH.SyncRuntimeState)

	return taskProgressSvc
}
