package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	pluginruntime "github.com/react-go-quick-starter/server/internal/plugin"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
	skillspkg "github.com/react-go-quick-starter/server/internal/skills"
	"github.com/react-go-quick-starter/server/internal/storage"
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
	var contextPayload map[string]any
	if req.CodeContext != nil {
		if contextPayload == nil {
			contextPayload = make(map[string]any)
		}
		contextPayload["codeContext"] = req.CodeContext
	}
	if len(req.FewShotHistory) > 0 {
		if contextPayload == nil {
			contextPayload = make(map[string]any)
		}
		contextPayload["fewShotHistory"] = req.FewShotHistory
	}
	if req.WaveMode {
		if contextPayload == nil {
			contextPayload = make(map[string]any)
		}
		contextPayload["waveMode"] = true
	}

	resp, err := a.client.DecomposeTask(ctx, bridge.DecomposeRequest{
		TaskID:      req.TaskID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Provider:    req.Provider,
		Model:       req.Model,
		Context:     contextPayload,
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

type docDecompositionWikiRepositoryAdapter struct {
	pages  *repository.WikiPageRepository
	spaces *repository.WikiSpaceRepository
}

func (a docDecompositionWikiRepositoryAdapter) GetByID(ctx context.Context, id uuid.UUID) (*model.WikiPage, error) {
	return a.pages.GetByID(ctx, id)
}

func (a docDecompositionWikiRepositoryAdapter) GetSpaceByID(ctx context.Context, id uuid.UUID) (*model.WikiSpace, error) {
	return a.spaces.GetByID(ctx, id)
}

type imControlPlaneWSAdapter struct {
	control *service.IMControlPlane
}

func (a imControlPlaneWSAdapter) AttachBridgeListener(ctx context.Context, bridgeID string, afterCursor int64, listener ws.IMBridgeListener) ([]*service.IMControlDelivery, error) {
	return a.control.AttachBridgeListener(ctx, bridgeID, afterCursor, listener)
}

func (a imControlPlaneWSAdapter) AckDelivery(ctx context.Context, ack *model.IMDeliveryAck) error {
	return a.control.AckDelivery(ctx, ack)
}

func (a imControlPlaneWSAdapter) DetachBridgeListener(bridgeID string) {
	a.control.DetachBridgeListener(bridgeID)
}

type RouteServices struct {
	TaskProgress *service.TaskProgressService
	Automation   *service.AutomationEngineService
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
	entityLinkRepo *repository.EntityLinkRepository,
	taskCommentRepo *repository.TaskCommentRepository,
	imReactionEventRepo *repository.IMReactionEventRepository,
	customFieldRepo *repository.CustomFieldRepository,
	savedViewRepo *repository.SavedViewRepository,
	formRepo *repository.FormRepository,
	automationRuleRepo *repository.AutomationRuleRepository,
	automationLogRepo *repository.AutomationLogRepository,
	dashboardRepo *repository.DashboardRepository,
	milestoneRepo *repository.MilestoneRepository,
	taskProgressRepo *repository.TaskProgressRepository,
	agentRunRepo *repository.AgentRunRepository,
	agentPoolQueueRepo *repository.AgentPoolQueueRepository,
	dispatchAttemptRepo *repository.DispatchAttemptRepository,
	notifRepo *repository.NotificationRepository,
	reviewRepo *repository.ReviewRepository,
	reviewAggRepo *repository.ReviewAggregationRepository,
	falsePosRepo *repository.FalsePositiveRepository,
	workflowRepo *repository.WorkflowRepository,
	teamRepo *repository.AgentTeamRepository,
	memoryRepo *repository.AgentMemoryRepository,
	wikiSpaceRepo *repository.WikiSpaceRepository,
	wikiPageRepo *repository.WikiPageRepository,
	pageVersionRepo *repository.PageVersionRepository,
	pageCommentRepo *repository.PageCommentRepository,
	pageFavoriteRepo *repository.PageFavoriteRepository,
	pageRecentAccessRepo *repository.PageRecentAccessRepository,
	documentRepo *repository.DocumentRepo,
	logRepo *repository.LogRepository,
	hub *ws.Hub,
	bridgeClient *bridge.Client,
	bridgeHealthSvc *service.BridgeHealthService,
	pluginSvc *service.PluginService,
	agentSvc *service.AgentService,
	schedulerSvc handler.SchedulerService,
) *RouteServices {
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
	wikiSvc := service.NewWikiService(
		wikiSpaceRepo,
		wikiPageRepo,
		pageVersionRepo,
		pageCommentRepo,
		pageFavoriteRepo,
		pageRecentAccessRepo,
		hub,
	).WithNotificationCreator(notificationSvc)
	if agentSvc != nil {
		agentSvc.SetProgressTracker(taskProgressSvc)
		agentSvc.SetIMProgressNotifier(imControlPlane)
	}
	memorySvc := service.NewMemoryService(memoryRepo)
	os.MkdirAll("./data/uploads", 0o755)
	docStorage := storage.NewLocalStorage("./data/uploads")
	documentSvc := service.NewDocumentService(documentRepo, docStorage, memorySvc)
	episodicMemorySvc := service.NewEpisodicMemoryService(memoryRepo)
	memoryExplorerSvc := service.NewMemoryExplorerService(memoryRepo).WithEpisodic(episodicMemorySvc)
	memoryAPI := service.NewMemoryAPIService(memorySvc, memoryExplorerSvc)
	var teamSvc *service.TeamService
	if agentSvc != nil {
		teamSvc = service.NewTeamService(teamRepo, agentRunRepo, agentSvc, taskRepo, projectRepo, memorySvc, hub, agentSvc.TeamArtifactService())
		agentSvc.SetTeamService(teamSvc)
		agentSvc.SetMemoryService(memorySvc)
	}
	reviewSvc := service.NewReviewService(reviewRepo, taskRepo, notificationSvc, hub, bridgeClient, taskProgressSvc)
	reviewSvc.SetIMProgressNotifier(imControlPlane)
	reviewSvc.WithProjectRepository(projectRepo)
	reviewAggSvc := service.NewReviewAggregationService(reviewRepo, reviewAggRepo, falsePosRepo, taskRepo)
	reviewSvc.WithAggregationService(reviewAggSvc)
	reviewSvc.WithDocWriteback(entityLinkRepo, wikiPageRepo, pageVersionRepo)
	taskDecomposeSvc := service.NewTaskDecompositionService(taskRepo, taskDecompositionBridgeAdapter{client: bridgeClient})
	docDecomposeSvc := service.NewDocDecompositionService(taskRepo, docDecompositionWikiRepositoryAdapter{pages: wikiPageRepo, spaces: wikiSpaceRepo}, entityLinkRepo)
	entityLinkSvc := service.NewEntityLinkService(entityLinkRepo, taskRepo, wikiPageRepo).WithHub(hub)
	taskCommentSvc := service.NewTaskCommentService(taskCommentRepo, memberRepo, notificationSvc, taskRepo).WithHub(hub)
	taskSvc := service.NewTaskService(taskRepo, hub).WithEntityLinkSyncer(entityLinkSvc)
	wikiSvc.WithEntityLinkSyncer(entityLinkSvc)

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
	users.PUT("/me", authH.UpdateMe)
	users.PUT("/me/password", authH.ChangePassword)

	// WebSocket
	wsH := ws.NewHandler(hub, cfg.JWTSecret)
	e.GET("/ws", wsH.HandleWS)
	if agentSvc != nil {
		e.GET("/ws/bridge", ws.NewBridgeHandler(agentSvc).HandleWS)
	}
	e.GET("/ws/im-bridge", ws.NewIMControlHandler(imControlPlaneWSAdapter{control: imControlPlane}).HandleWS)

	// --- New resource handlers ---
	projectH := handler.NewProjectHandler(projectRepo, bridgeClient, wikiSvc)
	memberH := handler.NewMemberHandler(memberRepo)
	sprintH := handler.NewSprintHandler(sprintRepo, taskRepo).WithHub(hub)
	taskH := handler.NewTaskHandler(taskRepo, taskDecomposeSvc).WithProgress(taskProgressSvc).WithHub(hub)
	wikiH := handler.NewWikiHandler(wikiSvc)
	entityLinkH := handler.NewEntityLinkHandler(entityLinkSvc)
	taskCommentH := handler.NewTaskCommentHandler(taskCommentSvc)
	docDecomposeH := handler.NewDocDecompositionHandler(docDecomposeSvc)
	var agentRuntime handler.AgentRuntimeService
	if agentSvc != nil {
		agentRuntime = agentSvc
	}
	agentH := handler.NewAgentHandler(agentRuntime)
	bridgeHealthH := handler.NewBridgeHealthHandler(bridgeHealthSvc)
	bridgePoolH := handler.NewBridgePoolHandler(bridgeClient)
	bridgeRuntimeCatalogH := handler.NewBridgeRuntimeCatalogHandler(bridgeClient)
	bridgeToolsAllowlist := append([]string{}, cfg.BridgeToolManifestAllowlist...)
	if strings.TrimSpace(cfg.PluginRegistryURL) != "" {
		bridgeToolsAllowlist = append(bridgeToolsAllowlist, cfg.PluginRegistryURL)
	}
	bridgeToolsH := handler.NewBridgeToolsHandler(bridgeClient, bridgeToolsAllowlist...)
	bridgeAIH := handler.NewBridgeAIHandler(bridgeClient)
	bridgeConvH := handler.NewBridgeConversationHandler(bridgeClient)
	notifH := handler.NewNotificationHandler(notifRepo)
	workflowH := handler.NewWorkflowHandler(workflowRepo)
	// DAG workflow engine
	dagDefRepo := repository.NewWorkflowDefinitionRepository(taskRepo.DB())
	dagExecRepo := repository.NewWorkflowExecutionRepository(taskRepo.DB())
	dagNodeExecRepo := repository.NewWorkflowNodeExecutionRepository(taskRepo.DB())
	dagWorkflowSvc := service.NewDAGWorkflowService(dagDefRepo, dagExecRepo, dagNodeExecRepo, hub)
	dagWorkflowSvc.SetTaskRepo(taskRepo)
	dagRunMappingRepo := repository.NewWorkflowRunMappingRepository(taskRepo.DB())
	dagWorkflowSvc.SetRunMappingRepo(dagRunMappingRepo)
	if agentSvc != nil {
		dagWorkflowSvc.SetAgentSpawner(agentSvc)
		agentSvc.SetDAGWorkflowService(dagWorkflowSvc)
	}
	workflowH = workflowH.WithDAGService(dagWorkflowSvc, dagDefRepo, dagExecRepo, dagNodeExecRepo)
	// Template and review services
	templateSvc := service.NewWorkflowTemplateService(dagDefRepo, dagWorkflowSvc)
	workflowH = workflowH.WithTemplateService(templateSvc)
	wfReviewRepo := repository.NewWorkflowPendingReviewRepository(taskRepo.DB())
	dagWorkflowSvc.SetReviewRepo(wfReviewRepo)
	workflowH = workflowH.WithReviewRepo(wfReviewRepo)
	// Seed system templates on startup, then wire team-to-workflow adapter
	go func() {
		_ = templateSvc.SeedSystemTemplates(context.Background())
		// Wire workflow adapter to team service after templates are seeded
		if teamSvc != nil {
			adapter := service.NewTeamWorkflowAdapter(templateSvc)
			teamSvc.SetWorkflowAdapter(adapter)
		}
	}()
	roleH := handler.NewRoleHandler(cfg.RolesDir).WithBridgeClient(bridgeClient)
	skillsH := handler.NewSkillsHandler(skillspkg.NewService(filepath.Dir(cfg.RolesDir)))
	customFieldH := handler.NewCustomFieldHandler(service.NewCustomFieldService(customFieldRepo))
	savedViewH := handler.NewSavedViewHandler(service.NewSavedViewService(savedViewRepo), memberRepo)
	formH := handler.NewFormHandler(service.NewFormService(formRepo, taskRepo, customFieldRepo))
	automationH := handler.NewAutomationHandler(automationRuleRepo, automationLogRepo)
	milestoneH := handler.NewMilestoneHandler(service.NewMilestoneService(milestoneRepo, taskRepo, sprintRepo))
	dashboardCrudSvc := service.NewDashboardService(dashboardRepo)
	dashboardDataSvc := service.NewDashboardWidgetService(taskRepo, sprintRepo, agentRunRepo, cache)
	dashboardH := handler.NewDashboardHandler(dashboardCrudSvc, dashboardDataSvc)
	var teamRuntime handler.TeamRuntimeService
	if teamSvc != nil {
		teamRuntime = teamSvc
	}
	teamH := handler.NewTeamHandler(teamRuntime)
	memoryH := handler.NewMemoryHandler(memoryAPI)
	reviewH := handler.NewReviewHandler(reviewSvc).WithAggregationService(reviewAggSvc)
	workflowRoleStore := role.NewFileStore(cfg.RolesDir)
	memberH = memberH.WithRoleStore(workflowRoleStore)
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
	roleH = roleH.WithPluginCatalog(pluginSvc)
	roleH = roleH.WithMemberCatalog(memberRepo)
	roleH = roleH.WithQueueCatalog(agentPoolQueueRepo)
	roleH = roleH.WithRunCatalog(agentRunRepo)
	if agentSvc != nil {
		agentSvc.SetPluginCatalog(pluginSvc)
	}
	automationEngine := service.NewAutomationEngineService(
		automationRuleRepo,
		automationLogRepo,
		taskRepo,
		customFieldRepo,
		notificationSvc,
		nil,
		pluginSvc,
	)
	automationEngine.SetIMChannelResolver(imControlPlane)
	taskH = taskH.WithAutomation(automationEngine)
	customFieldH = customFieldH.WithAutomation(automationEngine)
	reviewSvc.SetAutomationEvaluator(automationEngine)
	if agentSvc != nil {
		agentSvc.SetAutomationEvaluator(automationEngine)
	}
	reviewSvc.WithExecutionPlanner(service.NewReviewExecutionPlanner(pluginSvc))
	recommendSvc := service.NewAssignmentRecommender(taskRepo, memberRepo, agentRunRepo)
	taskH = taskH.WithRecommender(recommendSvc)
	var dispatchSvc *service.TaskDispatchService
	var budgetSvc *service.BudgetGovernanceService
	var dispatchPreflightH *handler.DispatchPreflightHandler
	var dispatchStatsH *handler.DispatchStatsHandler
	var dispatchHistoryH *handler.DispatchHistoryHandler
	var queueManagementH *handler.QueueManagementHandler
	var budgetQueryH *handler.BudgetQueryHandler
	if agentSvc != nil {
		budgetSvc = service.NewBudgetGovernanceService(sprintRepo, taskRepo)
		budgetSvc.SetAutomationEvaluator(automationEngine)
		agentSvc.SetDispatchBudgetChecker(budgetSvc)
		agentSvc.SetDispatchMemberReader(memberRepo)
		agentSvc.SetDispatchAttemptRecorder(dispatchAttemptRepo)
		dispatchSvc = service.NewTaskDispatchService(taskRepo, memberRepo, agentSvc, hub, notificationSvc, taskProgressSvc)
		dispatchSvc = dispatchSvc.WithQueueWriter(agentSvc)
		dispatchSvc = dispatchSvc.WithBudgetChecker(budgetSvc)
		dispatchSvc = dispatchSvc.WithAttemptRecorder(dispatchAttemptRepo)
		dispatchSvc = dispatchSvc.WithRoleStore(workflowRoleStore)
		dispatchPreflightH = handler.NewDispatchPreflightHandler(taskRepo, memberRepo, budgetSvc, agentSvc).WithRunReader(agentSvc).WithRoleStore(workflowRoleStore)
		dispatchStatsH = handler.NewDispatchStatsHandler(dispatchAttemptRepo, agentPoolQueueRepo)
		dispatchHistoryH = handler.NewDispatchHistoryHandler(dispatchAttemptRepo)
		queueManagementH = handler.NewQueueManagementHandler(agentSvc)
		budgetQueryH = handler.NewBudgetQueryHandler(budgetSvc)
		taskH = taskH.WithDispatcher(dispatchSvc)
		agentH = agentH.WithDispatcher(dispatchSvc)
	}
	documentH := handler.NewDocumentHandler(documentSvc)
	logSvc := service.NewLogService(logRepo, hub)
	logH := handler.NewLogHandler(logSvc)

	costQuerySvc := service.NewCostQueryService(taskRepo, sprintRepo, agentRunRepo, budgetSvc)
	costH := handler.NewCostHandler(costQuerySvc)
	workflowRunRepo := repository.NewWorkflowPluginRunRepository()
	workflowExec := service.NewWorkflowExecutionService(
		pluginSvc,
		workflowRunRepo,
		workflowRoleStore,
		service.NewWorkflowStepRouterExecutor(agentSvc, reviewSvc, dispatchSvc),
	)
	automationEngine.SetWorkflowStarter(workflowExec)
	taskWorkflowSvc := service.NewTaskWorkflowService(workflowRepo, hub)
	taskWorkflowSvc.SetTaskRepository(taskRepo)
	taskWorkflowSvc.SetNotifier(notifRepo)
	taskWorkflowSvc.SetProgressRecorder(taskProgressSvc)
	taskWorkflowSvc.SetWorkflowRuntime(workflowExec)
	if dispatchSvc != nil {
		taskWorkflowSvc.SetDispatcher(dispatchSvc)
	}
	taskH = taskH.WithWorkflowService(taskWorkflowSvc)
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
	projectGroup.POST("/members/bulk-update", memberH.BulkUpdate)
	projectGroup.POST("/tasks", taskH.Create)
	projectGroup.GET("/tasks", taskH.List)
	projectGroup.POST("/links", entityLinkH.Create)
	projectGroup.GET("/links", entityLinkH.List)
	projectGroup.DELETE("/links/:linkId", entityLinkH.Delete)
	projectGroup.GET("/tasks/:tid/comments", taskCommentH.List)
	projectGroup.POST("/tasks/:tid/comments", taskCommentH.Create)
	projectGroup.PATCH("/tasks/:tid/comments/:cid", taskCommentH.Update)
	projectGroup.DELETE("/tasks/:tid/comments/:cid", taskCommentH.Delete)
	projectGroup.GET("/fields", customFieldH.ListDefinitions)
	projectGroup.POST("/fields", customFieldH.CreateDefinition)
	projectGroup.PUT("/fields/reorder", customFieldH.ReorderDefinitions)
	projectGroup.PUT("/fields/:fid", customFieldH.UpdateDefinition)
	projectGroup.DELETE("/fields/:fid", customFieldH.DeleteDefinition)
	projectGroup.GET("/tasks/:tid/fields", customFieldH.ListTaskValues)
	projectGroup.PUT("/tasks/:tid/fields/:fid", customFieldH.SetTaskValue)
	projectGroup.DELETE("/tasks/:tid/fields/:fid", customFieldH.ClearTaskValue)
	projectGroup.GET("/views", savedViewH.List)
	projectGroup.POST("/views", savedViewH.Create)
	projectGroup.PUT("/views/:vid", savedViewH.Update)
	projectGroup.DELETE("/views/:vid", savedViewH.Delete)
	projectGroup.POST("/views/:vid/default", savedViewH.SetDefault)
	projectGroup.GET("/forms", formH.List)
	projectGroup.POST("/forms", formH.Create)
	projectGroup.PUT("/forms/:formId", formH.Update)
	projectGroup.DELETE("/forms/:formId", formH.Delete)
	projectGroup.GET("/automations", automationH.ListRules)
	projectGroup.POST("/automations", automationH.CreateRule)
	projectGroup.PUT("/automations/:rid", automationH.UpdateRule)
	projectGroup.DELETE("/automations/:rid", automationH.DeleteRule)
	projectGroup.GET("/automations/logs", automationH.ListLogs)
	projectGroup.GET("/dashboards", dashboardH.List)
	projectGroup.POST("/dashboards", dashboardH.Create)
	projectGroup.PUT("/dashboards/:did", dashboardH.Update)
	projectGroup.DELETE("/dashboards/:did", dashboardH.Delete)
	projectGroup.GET("/dashboards/:did/widgets", dashboardH.ListWidgets)
	projectGroup.POST("/dashboards/:did/widgets", dashboardH.SaveWidget)
	projectGroup.DELETE("/dashboards/:did/widgets/:wid", dashboardH.DeleteWidget)
	projectGroup.GET("/dashboard/widgets/:type", dashboardH.WidgetData)
	projectGroup.GET("/milestones", milestoneH.List)
	projectGroup.POST("/milestones", milestoneH.Create)
	projectGroup.PUT("/milestones/:mid", milestoneH.Update)
	projectGroup.DELETE("/milestones/:mid", milestoneH.Delete)
	projectGroup.POST("/sprints", sprintH.Create)
	projectGroup.GET("/sprints", sprintH.List)
	projectGroup.PUT("/sprints/:sid", sprintH.Update)
	projectGroup.GET("/sprints/:sid/metrics", sprintH.Metrics)
	projectGroup.GET("/workflow", workflowH.Get)
	projectGroup.PUT("/workflow", workflowH.Put)
	projectGroup.POST("/workflows", workflowH.CreateDefinition)
	projectGroup.GET("/workflows", workflowH.ListDefinitions)
	projectGroup.POST("/memory", memoryH.Store)
	projectGroup.GET("/memory", memoryH.Search)
	projectGroup.GET("/memory/stats", memoryH.Stats)
	projectGroup.GET("/memory/export", memoryH.Export)
	projectGroup.POST("/memory/bulk-delete", memoryH.BulkDelete)
	projectGroup.POST("/memory/cleanup", memoryH.Cleanup)
	projectGroup.GET("/memory/:mid", memoryH.Get)
	projectGroup.PATCH("/memory/:mid", memoryH.Update)
	projectGroup.DELETE("/memory/:mid", memoryH.Delete)
	docs := projectGroup.Group("/documents")
	docs.POST("/upload", documentH.Upload)
	docs.GET("", documentH.List)
	docs.GET("/:did", documentH.Get)
	docs.DELETE("/:did", documentH.Delete)
	projectGroup.GET("/logs", logH.List)
	projectGroup.POST("/logs", logH.Create)
	if dispatchPreflightH != nil {
		projectGroup.GET("/dispatch/preflight", dispatchPreflightH.Get)
	}
	if dispatchStatsH != nil {
		projectGroup.GET("/dispatch/stats", dispatchStatsH.Get)
	}
	if queueManagementH != nil {
		projectGroup.GET("/queue", queueManagementH.List)
		projectGroup.DELETE("/queue/:entryId", queueManagementH.Cancel)
	}
	if budgetQueryH != nil {
		projectGroup.GET("/budget/summary", budgetQueryH.ProjectSummary)
	}
	projectGroup.GET("/wiki/pages", wikiH.ListPages)
	projectGroup.POST("/wiki/pages", wikiH.CreatePage)
	projectGroup.GET("/wiki/pages/:id", wikiH.GetPage)
	projectGroup.PUT("/wiki/pages/:id", wikiH.UpdatePage)
	projectGroup.DELETE("/wiki/pages/:id", wikiH.DeletePage)
	projectGroup.PATCH("/wiki/pages/:id/move", wikiH.MovePage)
	projectGroup.GET("/wiki/pages/:id/versions", wikiH.ListVersions)
	projectGroup.POST("/wiki/pages/:id/versions", wikiH.CreateVersion)
	projectGroup.GET("/wiki/pages/:id/versions/:vid", wikiH.GetVersion)
	projectGroup.POST("/wiki/pages/:id/versions/:vid/restore", wikiH.RestoreVersion)
	projectGroup.GET("/wiki/pages/:id/comments", wikiH.ListComments)
	projectGroup.POST("/wiki/pages/:id/comments", wikiH.CreateComment)
	projectGroup.PATCH("/wiki/pages/:id/comments/:cid", wikiH.UpdateComment)
	projectGroup.DELETE("/wiki/pages/:id/comments/:cid", wikiH.DeleteComment)
	projectGroup.POST("/wiki/pages/:id/decompose-tasks", docDecomposeH.Decompose)
	projectGroup.GET("/wiki/templates", wikiH.ListTemplates)
	projectGroup.POST("/wiki/templates", wikiH.CreateTemplate)
	projectGroup.POST("/wiki/pages/:id/templates", wikiH.CreateTemplateFromPage)
	projectGroup.POST("/wiki/pages/from-template", wikiH.CreatePageFromTemplate)
	projectGroup.GET("/wiki/favorites", wikiH.ListFavorites)
	projectGroup.PUT("/wiki/pages/:id/favorite", wikiH.ToggleFavorite)
	projectGroup.GET("/wiki/recent", wikiH.ListRecentAccess)
	projectGroup.PUT("/wiki/pages/:id/pin", wikiH.TogglePinned)
	protected.GET("/wiki/pages/:id", wikiH.GetPageContext)

	// Task operations (not project-scoped, task ID is unique)
	protected.GET("/tasks/:id", taskH.Get)
	protected.PUT("/tasks/:id", taskH.Update)
	protected.DELETE("/tasks/:id", taskH.Delete)
	protected.POST("/tasks/:id/transition", taskH.Transition)
	protected.POST("/tasks/:id/assign", taskH.Assign)
	protected.GET("/tasks/:id/recommend-assignee", taskH.RecommendAssignee)
	protected.POST("/tasks/:id/decompose", taskH.Decompose)
	if dispatchHistoryH != nil {
		protected.GET("/tasks/:tid/dispatch/history", dispatchHistoryH.Get)
	}
	v1.GET("/forms/:slug", formH.GetBySlug)
	v1.POST("/forms/:slug/submit", formH.Submit)

	// Workflow definitions & executions (not project-scoped, ID is unique)
	protected.GET("/workflows/:id", workflowH.GetDefinition)
	protected.PUT("/workflows/:id", workflowH.UpdateDefinition)
	protected.DELETE("/workflows/:id", workflowH.DeleteDefinition)
	protected.POST("/workflows/:id/execute", workflowH.StartExecution)
	protected.GET("/workflows/:id/executions", workflowH.ListExecutions)
	protected.GET("/executions/:id", workflowH.GetExecution)
	protected.POST("/executions/:id/cancel", workflowH.CancelExecution)
	protected.POST("/executions/:id/review", workflowH.ResolveHumanReview)
	protected.POST("/executions/:id/events", workflowH.HandleExternalEvent)

	// Workflow reviews
	projectGroup.GET("/workflow-reviews", workflowH.ListPendingReviews)

		// Workflow templates
		protected.GET("/workflow-templates", workflowH.ListTemplates)
		protected.POST("/workflows/:id/publish-template", workflowH.PublishTemplate)
		protected.POST("/workflow-templates/:id/duplicate", workflowH.DuplicateTemplate)
		protected.POST("/workflow-templates/:id/clone", workflowH.CloneTemplate)
		protected.POST("/workflow-templates/:id/execute", workflowH.ExecuteTemplate)
		protected.DELETE("/workflow-templates/:id", workflowH.DeleteTemplate)

	// Members (global)
	protected.PUT("/members/:id", memberH.Update)
	protected.DELETE("/members/:id", memberH.Delete)

	// Agents
	protected.POST("/agents/spawn", agentH.Spawn)
	protected.GET("/agents", agentH.List)
	protected.GET("/agents/pool", agentH.Pool)
	protected.GET("/bridge/health", bridgeHealthH.Get)
	protected.GET("/bridge/pool", bridgePoolH.Get)
	protected.GET("/bridge/runtimes", bridgeRuntimeCatalogH.Get)
	protected.GET("/bridge/tools", bridgeToolsH.List)
	protected.POST("/bridge/tools/install", bridgeToolsH.Install)
	protected.POST("/bridge/tools/uninstall", bridgeToolsH.Uninstall)
	protected.POST("/bridge/tools/:id/restart", bridgeToolsH.Restart)
	protected.POST("/ai/decompose", bridgeAIH.Decompose)
	protected.POST("/ai/generate", bridgeAIH.Generate)
	protected.POST("/ai/classify-intent", bridgeAIH.ClassifyIntent)

	// Bridge conversation management & runtime control
	protected.POST("/bridge/fork", bridgeConvH.Fork)
	protected.POST("/bridge/rollback", bridgeConvH.Rollback)
	protected.POST("/bridge/revert", bridgeConvH.Revert)
	protected.POST("/bridge/unrevert", bridgeConvH.Unrevert)
	protected.GET("/bridge/diff/:task_id", bridgeConvH.GetDiff)
	protected.GET("/bridge/messages/:task_id", bridgeConvH.GetMessages)
	protected.POST("/bridge/command", bridgeConvH.ExecuteCommand)
	protected.POST("/bridge/shell", bridgeConvH.ExecuteShell)
	protected.POST("/bridge/interrupt", bridgeConvH.Interrupt)
	protected.POST("/bridge/model", bridgeConvH.SwitchModel)
	protected.POST("/bridge/thinking", bridgeConvH.SetThinkingBudget)
	protected.GET("/bridge/mcp-status/:task_id", bridgeConvH.GetMCPStatus)
	protected.POST("/bridge/permission-response/:request_id", bridgeConvH.PermissionResponse)
	protected.POST("/bridge/opencode/provider-auth/:provider/start", bridgeConvH.StartOpenCodeProviderAuth)
	protected.POST("/bridge/opencode/provider-auth/:request_id/complete", bridgeConvH.CompleteOpenCodeProviderAuth)
	protected.GET("/bridge/active", bridgeConvH.GetActive)
	protected.GET("/bridge/plugins", bridgeConvH.ListPlugins)
	protected.POST("/bridge/plugins/:id/enable", bridgeConvH.EnablePlugin)
	protected.POST("/bridge/plugins/:id/disable", bridgeConvH.DisablePlugin)
	protected.GET("/agents/:id", agentH.Get)
	protected.POST("/agents/:id/pause", agentH.Pause)
	protected.POST("/agents/:id/resume", agentH.Resume)
	protected.POST("/agents/:id/kill", agentH.Kill)
	protected.GET("/agents/:id/logs", agentH.Logs)

	// Teams
	protected.POST("/teams/start", teamH.Start)
	protected.GET("/teams", teamH.List)
	protected.GET("/teams/:id", teamH.Get)
	protected.PUT("/teams/:id", teamH.Update)
	protected.DELETE("/teams/:id", teamH.Delete)
	protected.POST("/teams/:id/cancel", teamH.Cancel)
	protected.POST("/teams/:id/retry", teamH.Retry)
	protected.GET("/teams/:id/artifacts", teamH.ListArtifacts)

	// Notifications
	protected.GET("/notifications", notifH.List)
	protected.PUT("/notifications/:id/read", notifH.MarkRead)
	protected.PUT("/notifications/read-all", notifH.MarkAllRead)

	// Scheduler control plane
	protected.GET("/scheduler/stats", schedulerH.GetStats)
	protected.GET("/scheduler/jobs", schedulerH.ListJobs)
	protected.GET("/scheduler/jobs/:jobKey", schedulerH.GetJob)
	protected.GET("/scheduler/jobs/:jobKey/preview", schedulerH.GetPreview)
	protected.GET("/scheduler/jobs/:jobKey/config-metadata", schedulerH.GetConfigMetadata)
	protected.GET("/scheduler/jobs/:jobKey/runs", schedulerH.ListRuns)
	protected.POST("/scheduler/jobs/:jobKey/pause", schedulerH.PauseJob)
	protected.POST("/scheduler/jobs/:jobKey/resume", schedulerH.ResumeJob)
	protected.POST("/scheduler/jobs/:jobKey/cancel", schedulerH.CancelJob)
	protected.POST("/scheduler/jobs/:jobKey/runs/cleanup", schedulerH.CleanupRuns)
	protected.PUT("/scheduler/jobs/:jobKey", schedulerH.UpdateJob)
	protected.POST("/scheduler/jobs/:jobKey/trigger", schedulerH.TriggerManual)
	e.GET("/internal/scheduler/jobs", schedulerH.ListJobs)
	e.POST("/internal/scheduler/jobs/:jobKey/trigger", schedulerH.TriggerCron)
	protected.GET("/reviews", reviewH.ListAll)
	protected.GET("/reviews/:id", reviewH.Get)
	protected.GET("/tasks/:taskId/reviews", reviewH.ListByTask)
	protected.POST("/reviews/:id/complete", reviewH.Complete)
	protected.POST("/reviews/:id/approve", reviewH.Approve)
	protected.POST("/reviews/:id/reject", reviewH.Reject)
	protected.POST("/reviews/:id/request-changes", reviewH.RequestChanges)
	protected.POST("/reviews/:id/false-positive", reviewH.MarkFalsePositive)
	v1.POST("/reviews/ci-result", reviewH.IngestCIResult, reviewTriggerMw)

	// Cost & Stats
	protected.GET("/stats/cost", costH.GetStats)
	statsSvc := service.NewStatsService(taskRepo, agentRunRepo)
	statsH := handler.NewStatsHandler(statsSvc)
	protected.GET("/stats/velocity", statsH.Velocity)
	protected.GET("/stats/agent-performance", statsH.AgentPerformance)

	protected.GET("/sprints/:sid/burndown", sprintH.Burndown)
	if budgetQueryH != nil {
		protected.GET("/sprints/:sid/budget", budgetQueryH.SprintDetail)
	}

	// Roles
	protected.GET("/skills", skillsH.List)
	protected.GET("/skills/:id", skillsH.Get)
	protected.POST("/skills/verify", skillsH.Verify)
	protected.POST("/skills/sync-mirrors", skillsH.SyncMirrors)
	protected.GET("/roles", roleH.List)
	protected.GET("/roles/skills", roleH.ListSkills)
	protected.GET("/roles/:id/references", roleH.GetReferences)
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
	protected.GET("/plugins/marketplace/remote", pluginH.ListRemotePlugins)
	protected.POST("/plugins/marketplace/:id/install-remote", pluginH.InstallRemotePlugin)
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

	// Marketplace integration
	marketplaceH := handler.NewMarketplaceHandler(pluginSvc, cfg.MarketplaceURL, cfg.PluginsDir, cfg.RolesDir).
		WithWorkflowTemplateRepo(dagDefRepo)
	protected.POST("/marketplace/install", marketplaceH.Install)
	protected.POST("/marketplace/uninstall", marketplaceH.Uninstall)
	protected.POST("/marketplace/sideload", marketplaceH.Sideload)
	protected.GET("/marketplace/updates", marketplaceH.Updates)
	protected.GET("/marketplace/installed", marketplaceH.Installed)
	protected.GET("/marketplace/consumption", marketplaceH.Consumption)
	protected.GET("/marketplace/built-in-skills", marketplaceH.ListBuiltInSkills)
	protected.GET("/marketplace/built-in-skills/:id", marketplaceH.GetBuiltInSkill)

	// IM Bridge
	imSvc := service.NewIMService(cfg.IMNotifyURL, cfg.IMNotifyPlatform, imControlPlane)
	imSvc.SetDeliverySecret(cfg.IMControlSharedSecret)
	if bridgeClient != nil {
		imSvc.SetClassifier(bridgeIntentAdapter{client: bridgeClient})
	}
	if agentSvc != nil {
		agentSvc.SetIMNotifier(imSvc)
		agentSvc.SetIMChannelResolver(imControlPlane)
	}
	imSvc.SetActionExecutor(service.NewBackendIMActionExecutor(
		dispatchSvc,
		taskDecomposeSvc,
		reviewSvc,
		taskSvc,
		imControlPlane,
		taskProgressSvc,
		taskWorkflowSvc,
		wikiSvc,
		imReactionEventRepo,
		taskCommentRepo,
	))
	automationEngine.SetIMSender(imSvc)
	wikiSvc.WithIMForwarder(imSvc, cfg.IMNotifyPlatform, cfg.IMNotifyTargetChatID).WithIMChannelResolver(imControlPlane)
	imH := handler.NewIMHandler(imSvc)
	imControlH := handler.NewIMControlHandler(imControlPlane, imSvc)
	v1.POST("/im/message", imH.HandleMessage)
	v1.POST("/im/command", imH.HandleCommand)
	v1.POST("/intent", imH.HandleIntent)
	v1.POST("/im/action", imH.HandleAction)
	v1.POST("/im/bridge/register", imControlH.Register)
	v1.POST("/im/bridge/heartbeat", imControlH.Heartbeat)
	v1.POST("/im/bridge/unregister", imControlH.Unregister)
	v1.POST("/im/bridge/bind", imControlH.BindAction)
	v1.POST("/im/bridge/ack", imControlH.AckDelivery)
	protected.GET("/im/channels", imControlH.ListChannels)
	protected.POST("/im/channels", imControlH.SaveChannel)
	protected.PUT("/im/channels/:id", imControlH.SaveChannel)
	protected.DELETE("/im/channels/:id", imControlH.DeleteChannel)
	protected.GET("/im/bridge/status", imControlH.GetStatus)
	protected.GET("/im/deliveries", imControlH.ListDeliveries)
	protected.POST("/im/deliveries/:id/retry", imControlH.RetryDelivery)
	protected.POST("/im/deliveries/retry-batch", imControlH.RetryBatchDeliveries)
	protected.POST("/im/test-send", imControlH.TestSend)
	protected.GET("/im/event-types", imControlH.ListEventTypes)
	protected.POST("/im/send", imH.Send)
	protected.POST("/im/notify", imH.Notify)

	// Bridge-to-registry runtime sync
	e.POST("/internal/plugins/runtime-state", pluginH.SyncRuntimeState)

	return &RouteServices{
		TaskProgress: taskProgressSvc,
		Automation:   automationEngine,
	}
}
