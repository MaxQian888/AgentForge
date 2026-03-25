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
	"github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/version"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type bridgeIntentAdapter struct {
	client *bridge.Client
}

func (a bridgeIntentAdapter) ClassifyIntent(ctx context.Context, req service.ClassifyIntentRequest) (*service.ClassifyIntentResponse, error) {
	resp, err := a.client.ClassifyIntent(ctx, bridge.ClassifyIntentRequest{
		Text:      req.Text,
		UserID:    req.UserID,
		ProjectID: req.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	return &service.ClassifyIntentResponse{
		Intent:     resp.Intent,
		Command:    resp.Command,
		Args:       resp.Args,
		Confidence: resp.Confidence,
		Reply:      resp.Reply,
	}, nil
}

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
			Title:         item.Title,
			Description:   item.Description,
			Priority:      item.Priority,
			ExecutionMode: item.ExecutionMode,
		})
	}
	return &service.BridgeDecomposeResponse{
		Summary:  resp.Summary,
		Subtasks: subtasks,
	}, nil
}

type imControlPlaneWSAdapter struct {
	control *service.IMControlPlane
}

func (a imControlPlaneWSAdapter) AttachBridgeListener(ctx context.Context, bridgeID string, afterCursor int64, listener ws.IMBridgeListener) ([]*service.IMControlDelivery, error) {
	return a.control.AttachBridgeListener(ctx, bridgeID, afterCursor, listener)
}

func (a imControlPlaneWSAdapter) AckDelivery(ctx context.Context, bridgeID string, cursor int64, deliveryID string) error {
	return a.control.AckDelivery(ctx, bridgeID, cursor, deliveryID)
}

func (a imControlPlaneWSAdapter) DetachBridgeListener(bridgeID string) {
	a.control.DetachBridgeListener(bridgeID)
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
	workflowRepo *repository.WorkflowRepository,
	teamRepo *repository.AgentTeamRepository,
	memoryRepo *repository.AgentMemoryRepository,
	hub *ws.Hub,
	bridgeClient *bridge.Client,
	pluginSvc *service.PluginService,
	agentSvc *service.AgentService,
	schedulerSvc handler.SchedulerService,
) *service.TaskProgressService {
	jwtMw := appMiddleware.JWTMiddleware(cfg.JWTSecret, cache)
	reviewTriggerMw := appMiddleware.ReviewTriggerAuthMiddleware(cfg.JWTSecret, cache, cfg.AgentForgeToken)
	notificationSvc := service.NewNotificationService(notifRepo, hub)
	imControlPlane := service.NewIMControlPlane(service.IMControlPlaneConfig{
		HeartbeatTTL:              cfg.IMBridgeHeartbeatTTL,
		ProgressHeartbeatInterval: cfg.IMBridgeProgressInterval,
		DeliverySecret:            cfg.IMControlSharedSecret,
	})
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
	if notifier := service.NewMultiTaskProgressIMNotifier(
		service.NewControlPlaneTaskProgressIMNotifier(imControlPlane),
		service.NewHTTPTaskProgressIMNotifier(cfg.IMNotifyURL, cfg.IMNotifyPlatform, cfg.IMNotifyTargetChatID, cfg.IMControlSharedSecret),
	); notifier != nil {
		taskProgressSvc.SetIMNotifier(notifier)
	}
	if agentSvc != nil {
		agentSvc.SetProgressTracker(taskProgressSvc)
		agentSvc.SetIMProgressNotifier(imControlPlane)
	}
	memorySvc := service.NewMemoryService(memoryRepo)
	var teamSvc *service.TeamService
	if agentSvc != nil {
		teamSvc = service.NewTeamService(teamRepo, agentRunRepo, agentSvc, taskRepo, projectRepo, memorySvc, hub)
		agentSvc.SetTeamService(teamSvc)
		agentSvc.SetMemoryService(memorySvc)
	}
	reviewSvc := service.NewReviewService(reviewRepo, taskRepo, notificationSvc, hub, bridgeClient, taskProgressSvc)
	reviewSvc.SetIMProgressNotifier(imControlPlane)
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
	if agentSvc != nil {
		e.GET("/ws/bridge", ws.NewBridgeHandler(agentSvc).HandleWS)
	}
	e.GET("/ws/im-bridge", ws.NewIMControlHandler(imControlPlaneWSAdapter{control: imControlPlane}).HandleWS)

	// --- New resource handlers ---
	projectH := handler.NewProjectHandler(projectRepo, bridgeClient)
	memberH := handler.NewMemberHandler(memberRepo)
	sprintH := handler.NewSprintHandler(sprintRepo, taskRepo).WithHub(hub)
	taskH := handler.NewTaskHandler(taskRepo, taskDecomposeSvc).WithProgress(taskProgressSvc).WithHub(hub)
	var agentRuntime handler.AgentRuntimeService
	if agentSvc != nil {
		agentRuntime = agentSvc
	}
	agentH := handler.NewAgentHandler(agentRuntime)
	notifH := handler.NewNotificationHandler(notifRepo)
	workflowH := handler.NewWorkflowHandler(workflowRepo)
	costH := handler.NewCostHandler(agentRunRepo)
	roleH := handler.NewRoleHandler(cfg.RolesDir).WithBridgeClient(bridgeClient)
	var teamRuntime handler.TeamRuntimeService
	if teamSvc != nil {
		teamRuntime = teamSvc
	}
	teamH := handler.NewTeamHandler(teamRuntime)
	memoryH := handler.NewMemoryHandler(memorySvc)
	reviewH := handler.NewReviewHandler(reviewSvc)
	workflowRoleStore := role.NewFileStore(cfg.RolesDir)
	if pluginSvc == nil {
		pluginSvc = service.NewPluginService(
			repository.NewPluginRegistryRepository(),
			bridgeClient,
			pluginruntime.NewWASMRuntimeManager(),
			cfg.PluginsDir,
		)
	}
	pluginSvc = pluginSvc.
		WithRoleStore(workflowRoleStore).
		WithBroadcaster(ws.NewPluginEventBroadcaster(hub))
	reviewSvc.WithExecutionPlanner(service.NewReviewExecutionPlanner(pluginSvc))
	var dispatchSvc *service.TaskDispatchService
	if agentSvc != nil {
		dispatchSvc = service.NewTaskDispatchService(taskRepo, memberRepo, agentSvc, hub, notificationSvc, taskProgressSvc)
		dispatchSvc = dispatchSvc.WithQueueWriter(agentSvc)
		taskH = taskH.WithDispatcher(dispatchSvc)
		agentH = agentH.WithDispatcher(dispatchSvc)
	}
	workflowRunRepo := repository.NewWorkflowPluginRunRepository()
	workflowExec := service.NewWorkflowExecutionService(
		pluginSvc,
		workflowRunRepo,
		workflowRoleStore,
		service.NewWorkflowStepRouterExecutor(agentSvc, reviewSvc, dispatchSvc),
	)
	pluginH := handler.NewPluginHandler(pluginSvc).WithWorkflowExecution(workflowExec)
	schedulerH := handler.NewSchedulerHandler(schedulerSvc)

	// JWT protected routes
	protected := v1.Group("", jwtMw)
	v1.POST("/reviews/trigger", reviewH.Trigger, reviewTriggerMw)

	// Projects
	protected.POST("/projects", projectH.Create)
	protected.GET("/projects", projectH.List)
	protected.GET("/projects/:id", projectH.Get)
	protected.PUT("/projects/:id", projectH.Update)
	protected.DELETE("/projects/:id", projectH.Delete)

	// Project-scoped routes
	projectMw := appMiddleware.ProjectMiddleware(projectRepo)
	projectGroup := protected.Group("/projects/:pid", projectMw)
	projectGroup.POST("/members", memberH.Create)
	projectGroup.GET("/members", memberH.List)
	projectGroup.POST("/tasks", taskH.Create)
	projectGroup.GET("/tasks", taskH.List)
	projectGroup.POST("/sprints", sprintH.Create)
	projectGroup.GET("/sprints", sprintH.List)
	projectGroup.PUT("/sprints/:sid", sprintH.Update)
	projectGroup.GET("/sprints/:sid/metrics", sprintH.Metrics)
	projectGroup.GET("/workflow", workflowH.Get)
	projectGroup.PUT("/workflow", workflowH.Put)
	projectGroup.POST("/memory", memoryH.Store)
	projectGroup.GET("/memory", memoryH.Search)
	projectGroup.DELETE("/memory/:mid", memoryH.Delete)

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
	protected.GET("/agents/pool", agentH.Pool)
	protected.GET("/agents/:id", agentH.Get)
	protected.POST("/agents/:id/pause", agentH.Pause)
	protected.POST("/agents/:id/resume", agentH.Resume)
	protected.POST("/agents/:id/kill", agentH.Kill)
	protected.GET("/agents/:id/logs", agentH.Logs)

	// Teams
	protected.POST("/teams/start", teamH.Start)
	protected.GET("/teams", teamH.List)
	protected.GET("/teams/:id", teamH.Get)
	protected.POST("/teams/:id/cancel", teamH.Cancel)
	protected.POST("/teams/:id/retry", teamH.Retry)

	// Notifications
	protected.GET("/notifications", notifH.List)
	protected.PUT("/notifications/:id/read", notifH.MarkRead)

	// Scheduler control plane
	protected.GET("/scheduler/jobs", schedulerH.ListJobs)
	protected.GET("/scheduler/jobs/:jobKey/runs", schedulerH.ListRuns)
	protected.PUT("/scheduler/jobs/:jobKey", schedulerH.UpdateJob)
	protected.POST("/scheduler/jobs/:jobKey/trigger", schedulerH.TriggerManual)
	e.GET("/internal/scheduler/jobs", schedulerH.ListJobs)
	e.POST("/internal/scheduler/jobs/:jobKey/trigger", schedulerH.TriggerCron)
	protected.GET("/reviews/:id", reviewH.Get)
	protected.GET("/tasks/:taskId/reviews", reviewH.ListByTask)
	protected.POST("/reviews/:id/complete", reviewH.Complete)
	protected.POST("/reviews/:id/approve", reviewH.Approve)
	protected.POST("/reviews/:id/reject", reviewH.Reject)

	// Cost & Stats
	protected.GET("/stats/cost", costH.GetStats)
	statsSvc := service.NewStatsService(taskRepo, agentRunRepo)
	statsH := handler.NewStatsHandler(statsSvc)
	protected.GET("/stats/velocity", statsH.Velocity)
	protected.GET("/stats/agent-performance", statsH.AgentPerformance)

	protected.GET("/sprints/:sid/burndown", sprintH.Burndown)

	// Roles
	protected.GET("/roles", roleH.List)
	protected.GET("/roles/:id", roleH.Get)
	protected.POST("/roles", roleH.Create)
	protected.PUT("/roles/:id", roleH.Update)
	protected.DELETE("/roles/:id", roleH.Delete)
	protected.POST("/roles/preview", roleH.Preview)
	protected.POST("/roles/sandbox", roleH.Sandbox)

	// Plugins
	protected.GET("/plugins/discover", pluginH.DiscoverBuiltIns)
	protected.POST("/plugins/discover/builtin", pluginH.DiscoverBuiltIns)
	protected.POST("/plugins/install", pluginH.InstallLocal)
	protected.GET("/plugins/catalog", pluginH.SearchCatalog)
	protected.POST("/plugins/catalog/install", pluginH.InstallCatalogEntry)
	protected.GET("/plugins/marketplace", pluginH.Marketplace)
	protected.GET("/plugins", pluginH.List)
	protected.DELETE("/plugins/:id", pluginH.Uninstall)
	protected.POST("/plugins/:id/update", pluginH.Update)
	protected.PUT("/plugins/:id/config", pluginH.UpdateConfig)
	protected.PUT("/plugins/:id/enable", pluginH.Enable)
	protected.POST("/plugins/:id/enable", pluginH.Enable)
	protected.POST("/plugins/:id/deactivate", pluginH.Deactivate)
	protected.PUT("/plugins/:id/disable", pluginH.Disable)
	protected.POST("/plugins/:id/disable", pluginH.Disable)
	protected.POST("/plugins/:id/activate", pluginH.Activate)
	protected.GET("/plugins/:id/health", pluginH.Health)
	protected.POST("/plugins/:id/restart", pluginH.Restart)
	protected.POST("/plugins/:id/invoke", pluginH.Invoke)
	protected.POST("/plugins/:id/mcp/refresh", pluginH.RefreshMCP)
	protected.POST("/plugins/:id/mcp/tools/call", pluginH.CallMCPTool)
	protected.POST("/plugins/:id/mcp/resources/read", pluginH.ReadMCPResource)
	protected.POST("/plugins/:id/mcp/prompts/get", pluginH.GetMCPPrompt)
	protected.GET("/plugins/:id/events", pluginH.ListEvents)
	protected.POST("/plugins/:id/workflow-runs", pluginH.StartWorkflowRun)
	protected.GET("/plugins/:id/workflow-runs", pluginH.ListWorkflowRuns)
	protected.GET("/plugins/workflow-runs/:runId", pluginH.GetWorkflowRun)

	// IM Bridge
	imSvc := service.NewIMService(cfg.IMNotifyURL, cfg.IMNotifyPlatform, imControlPlane)
	imSvc.SetDeliverySecret(cfg.IMControlSharedSecret)
	if bridgeClient != nil {
		imSvc.SetClassifier(bridgeIntentAdapter{client: bridgeClient})
	}
	imH := handler.NewIMHandler(imSvc)
	imControlH := handler.NewIMControlHandler(imControlPlane)
	v1.POST("/im/message", imH.HandleMessage)
	v1.POST("/im/command", imH.HandleCommand)
	v1.POST("/intent", imH.HandleIntent)
	v1.POST("/im/action", imH.HandleAction)
	v1.POST("/im/bridge/register", imControlH.Register)
	v1.POST("/im/bridge/heartbeat", imControlH.Heartbeat)
	v1.POST("/im/bridge/unregister", imControlH.Unregister)
	v1.POST("/im/bridge/bind", imControlH.BindAction)
	v1.POST("/im/bridge/ack", imControlH.AckDelivery)
	protected.POST("/im/send", imH.Send)
	protected.POST("/im/notify", imH.Notify)

	// Bridge-to-registry runtime sync
	e.POST("/internal/plugins/runtime-state", pluginH.SyncRuntimeState)

	return taskProgressSvc
}
