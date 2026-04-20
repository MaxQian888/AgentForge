package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/employee"
	"github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/knowledge"
	"github.com/react-go-quick-starter/server/internal/knowledge/liveartifact"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	pluginruntime "github.com/react-go-quick-starter/server/internal/plugin"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
	skillspkg "github.com/react-go-quick-starter/server/internal/skills"
	"github.com/react-go-quick-starter/server/internal/storage"
	"github.com/react-go-quick-starter/server/internal/trigger"
	"github.com/react-go-quick-starter/server/internal/version"
	"github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// wsHubAdapter bridges the eventbus.Publisher to the nodetypes.BroadcastHub
// interface that EffectApplier consumes (eventType, projectID, payload). On
// master ws.Hub no longer exposes BroadcastEvent directly; events flow through
// the eventbus and are fanned out to WebSocket clients by the core.ws-fanout
// observer module.
type wsHubAdapter struct {
	bus eventbus.Publisher
}

func (a wsHubAdapter) BroadcastEvent(eventType, projectID string, payload map[string]any) {
	_ = eventbus.PublishLegacy(context.Background(), a.bus, eventType, projectID, payload)
}

// projectTemplateAuditEmitter adapts the AuditService to the narrow
// ProjectCreatedFromTemplateEmitter contract the project handler consumes.
// The wrapper builds the canonical AuditEvent and hands it to RecordEvent;
// downstream sanitization / enqueue is the service's responsibility.
type projectTemplateAuditEmitter struct {
	svc *service.AuditService
}

func (p projectTemplateAuditEmitter) EmitProjectCreatedFromTemplate(
	ctx context.Context,
	projectID, actorUserID, templateID uuid.UUID,
	templateVersion int,
	templateSource string,
) {
	if p.svc == nil {
		return
	}
	event := &model.AuditEvent{
		ProjectID:    projectID,
		OccurredAt:   time.Now().UTC(),
		ActorUserID:  &actorUserID,
		ActionID:     string(appMiddleware.ActionProjectCreatedFromTemplate),
		ResourceType: model.AuditResourceTypeProject,
		ResourceID:   projectID.String(),
		PayloadSnapshotJSON: fmt.Sprintf(
			`{"templateId":%q,"templateSource":%q,"snapshotVersion":%d}`,
			templateID.String(), templateSource, templateVersion,
		),
	}
	_ = p.svc.RecordEvent(ctx, event)
}

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

// knowledgeEventBusAdapter bridges *eventbus.Bus to the knowledge.eventPublisher interface.
type knowledgeEventBusAdapter struct {
	bus *eventbus.Bus
}

func (a knowledgeEventBusAdapter) PublishKnowledgeEvent(ctx context.Context, eventType string, projectID string, payload map[string]any) error {
	return eventbus.PublishLegacy(ctx, a.bus, eventType, projectID, payload)
}

// employeeRoleRegistryAdapter bridges *role.FileStore (Get-based API) to the
// employee.RoleRegistry.Has(string) bool interface.
type employeeRoleRegistryAdapter struct {
	store *role.FileStore
}

func (a employeeRoleRegistryAdapter) Has(roleID string) bool {
	if a.store == nil || roleID == "" {
		return false
	}
	_, err := a.store.Get(roleID)
	return err == nil
}

// reviewWorkflowLauncherAdapter composes the template and DAG services into
// the ReviewWorkflowLauncher interface. It clones the system:code-review
// template once per project (idempotent — repeated calls reuse the existing
// active workflow with the same name) then starts a new execution.
type reviewWorkflowLauncherAdapter struct {
	templates *service.WorkflowTemplateService
	dag       *service.DAGWorkflowService
	defRepo   *repository.WorkflowDefinitionRepository
}

func (a reviewWorkflowLauncherAdapter) LaunchReviewWorkflow(ctx context.Context, projectID uuid.UUID, seed map[string]any) (uuid.UUID, error) {
	tmpl, err := a.templates.FindTemplateByName(ctx, service.TemplateSystemCodeReview)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find code-review template: %w", err)
	}

	// Reuse an active clone with the same name in this project, if one exists.
	var wfID uuid.UUID
	existing, listErr := a.defRepo.ListByProject(ctx, projectID)
	if listErr == nil {
		for _, def := range existing {
			if def.Name == tmpl.Name && def.Status == model.WorkflowDefStatusActive {
				wfID = def.ID
				break
			}
		}
	}
	if wfID == uuid.Nil {
		clone, cloneErr := a.templates.CloneTemplate(ctx, tmpl.ID, projectID, nil)
		if cloneErr != nil {
			return uuid.Nil, fmt.Errorf("clone template: %w", cloneErr)
		}
		wfID = clone.ID
	}

	exec, startErr := a.dag.StartExecution(ctx, wfID, nil, service.StartOptions{Seed: seed})
	if startErr != nil {
		return uuid.Nil, fmt.Errorf("start execution: %w", startErr)
	}
	return exec.ID, nil
}

type RouteServices struct {
	TaskProgress *service.TaskProgressService
	Automation   *service.AutomationEngineService
	AuditSink    *service.AuditSink
	Invitation   *service.InvitationService
}

func RegisterRoutes(
	e *echo.Echo,
	cfg *config.Config,
	authSvc *service.AuthService,
	cache *repository.CacheRepository,
	userRepo *repository.UserRepository,
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
	bus *eventbus.Bus,
	bridgeClient *bridge.Client,
	bridgeHealthSvc *service.BridgeHealthService,
	pluginSvc *service.PluginService,
	agentSvc *service.AgentService,
	schedulerSvc handler.SchedulerService,
) *RouteServices {
	jwtMw := appMiddleware.JWTMiddleware(cfg.JWTSecret, cache)
	reviewTriggerMw := appMiddleware.ReviewTriggerAuthMiddleware(cfg.JWTSecret, cache, cfg.AgentForgeToken)

	// Install the project-scoped RBAC member lookup. After this point any
	// route wrapped in appMiddleware.Require(actionID) consults memberRepo
	// to resolve the caller's projectRole.
	appMiddleware.SetMemberLookup(memberRepo)

	// Audit subsystem. The sink owns its own goroutine + retry queue +
	// disk spill; the service is the seam handlers and the RBAC middleware
	// emit through. Persistence is asynchronous and never blocks the
	// originating request.
	auditEventRepo := repository.NewAuditEventRepository(taskRepo.DB())
	auditSink := service.NewAuditSink(auditEventRepo, service.AuditSinkConfig{})
	auditSink.Start(context.Background())
	auditSvc := service.NewAuditService(auditSink, auditEventRepo, func(actionID string) bool {
		_, ok := appMiddleware.MinRoleFor(appMiddleware.ActionID(actionID))
		return ok
	})
	// Register the RBAC emitter so allow + deny paths produce audit events.
	appMiddleware.SetAuditEmitter(func(ctx context.Context, e appMiddleware.AuditEmission) {
		event := &model.AuditEvent{
			ProjectID:              e.ProjectID,
			OccurredAt:             time.Now().UTC(),
			ActorUserID:            e.ActorUserID,
			ActorProjectRoleAtTime: e.ActorProjectRoleAtTime,
			ActionID:               string(e.ActionID),
			ResourceType:           model.AuditResourceTypeAuth,
			RequestID:              e.RequestID,
			IP:                     e.IP,
			UserAgent:              e.UserAgent,
			SystemInitiated:        false,
		}
		if !e.Allowed {
			event.PayloadSnapshotJSON = service.SanitizeAuditPayload(map[string]any{"outcome": "denied"})
		} else {
			event.PayloadSnapshotJSON = service.SanitizeAuditPayload(map[string]any{"outcome": "allowed"})
		}
		_ = auditSvc.RecordEvent(ctx, event)
	})

	notificationSvc := service.NewNotificationService(notifRepo, hub, bus)
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
		bus,
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
		bus,
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
	// teamSvc is constructed below after templateSvc is built — TeamService now
	// requires the workflow template service to start team executions.
	var teamSvc *service.TeamService
	reviewSvc := service.NewReviewService(reviewRepo, taskRepo, notificationSvc, hub, bus, bridgeClient, taskProgressSvc)
	reviewSvc.SetIMProgressNotifier(imControlPlane)
	reviewSvc.WithProjectRepository(projectRepo)
	reviewAggSvc := service.NewReviewAggregationService(reviewRepo, reviewAggRepo, falsePosRepo, taskRepo)
	reviewSvc.WithAggregationService(reviewAggSvc)
	reviewSvc.WithDocWriteback(entityLinkRepo, wikiPageRepo, pageVersionRepo)
	taskDecomposeSvc := service.NewTaskDecompositionService(taskRepo, taskDecompositionBridgeAdapter{client: bridgeClient})
	docDecomposeSvc := service.NewDocDecompositionService(taskRepo, docDecompositionWikiRepositoryAdapter{pages: wikiPageRepo, spaces: wikiSpaceRepo}, entityLinkRepo)
	entityLinkSvc := service.NewEntityLinkService(entityLinkRepo, taskRepo, wikiPageRepo).WithHub(hub).WithBus(bus)
	taskCommentSvc := service.NewTaskCommentService(taskCommentRepo, memberRepo, notificationSvc, taskRepo).WithHub(hub).WithBus(bus)
	taskSvc := service.NewTaskService(taskRepo, hub, bus).WithEntityLinkSyncer(entityLinkSvc)
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

	// Per-project permissions endpoint. Frontend consumes this as the
	// canonical "what can I do here" source rather than mirroring the matrix.
	permissionsH := handler.NewPermissionsHandler(memberRepo)
	v1.GET("/auth/me/projects/:pid/permissions", permissionsH.Get, jwtMw)

	// WebSocket
	wsH := ws.NewHandler(hub, cfg.JWTSecret)
	e.GET("/ws", wsH.HandleWS)
	if agentSvc != nil {
		e.GET("/ws/bridge", ws.NewBridgeHandler(agentSvc).HandleWS)
	}
	e.GET("/ws/im-bridge", ws.NewIMControlHandler(imControlPlaneWSAdapter{control: imControlPlane}).HandleWS)

	// --- Project templates ---
	// Service first so the project handler can consume it via WithTemplateClone.
	projectTemplateRepo := repository.NewProjectTemplateRepository(taskRepo.DB())
	projectTemplateSvc := service.NewProjectTemplateService(projectTemplateRepo, projectRepo)
	// Register the built-in system templates bundle idempotently on startup.
	if err := service.RegisterBuiltInProjectTemplates(context.Background(), projectTemplateRepo); err != nil {
		// Non-fatal — templates can still be added by users; just log via stdout.
		fmt.Printf("register built-in project templates: %v\n", err)
	}
	projectTemplateH := handler.NewProjectTemplateHandler(projectTemplateSvc)

	// --- Project lifecycle (archive / unarchive / delete) ---
	// TeamService and DAGWorkflowService cascades are wired later, after
	// those services are constructed; the lifecycle service still gets
	// consulted by the delete handler so it must exist here.
	projectLifecycleSvc := service.NewProjectLifecycleService(projectRepo)

	// --- New resource handlers ---
	projectH := handler.NewProjectHandler(projectRepo, bridgeClient, wikiSvc).
		WithUserLookup(userRepo).
		WithTemplateClone(projectTemplateSvc).
		WithTemplateAuditEmitter(projectTemplateAuditEmitter{svc: auditSvc}).
		WithLifecycleService(projectLifecycleSvc)
	memberH := handler.NewMemberHandler(memberRepo)
	// Invitation flow wiring. Repo + service + handler are constructed here
	// so the audit emitter uses the same auditSvc the rest of the project
	// already consumes. Delivery is left nil by default; UI fallback "copy
	// accept link" path carries the plaintext token returned by Create.
	invitationRepo := repository.NewInvitationRepository(taskRepo.DB())
	invitationSvc := service.NewInvitationService(
		invitationRepo,
		memberRepo,
		userRepo,
		projectRepo,
		service.InvitationServiceConfig{AcceptURLBase: cfg.FrontendAcceptInvitationURL},
	).WithAuditEmitter(auditSvc)
	if notificationSvc != nil {
		invitationSvc = invitationSvc.WithDelivery(service.NewInvitationNotificationDelivery(
			notificationSvc, invitationRepo, userRepo, projectRepo, cfg.FrontendAcceptInvitationURL,
		))
	}
	invitationH := handler.NewInvitationHandler(invitationSvc)
	sprintH := handler.NewSprintHandler(sprintRepo, taskRepo).WithHub(hub).WithBus(bus)
	taskH := handler.NewTaskHandler(taskRepo, taskDecomposeSvc).WithProgress(taskProgressSvc).WithHub(hub).WithBus(bus)
	// wikiH is kept for reference but routes have been migrated to knowledgeH.
	_ = handler.NewWikiHandler(wikiSvc)
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
	// Hoisted: needed by EffectApplier construction below.
	dagRunMappingRepo := repository.NewWorkflowRunMappingRepository(taskRepo.DB())
	wfReviewRepo := repository.NewWorkflowPendingReviewRepository(taskRepo.DB())

	// Build node-type registry and seed with built-ins.
	nodeRegistry := nodetypes.NewRegistry(nil)
	if err := nodetypes.RegisterBuiltins(nodeRegistry, nodetypes.BuiltinDeps{
		TaskRepo: taskRepo,
		DefRepo:  dagDefRepo,
	}); err != nil {
		panic(fmt.Errorf("register built-in node types: %w", err))
	}
	nodeRegistry.LockGlobal()

	// Build effect applier with all back-end deps.
	effectApplier := &nodetypes.EffectApplier{
		Hub:         wsHubAdapter{bus: bus},
		TaskRepo:    taskRepo,
		NodeRepo:    dagNodeExecRepo,
		ExecRepo:    dagExecRepo,
		MappingRepo: dagRunMappingRepo,
		ReviewRepo:  wfReviewRepo,
		// AgentSpawner wired below once agentSvc is known to exist.
	}

	dagWorkflowSvc := service.NewDAGWorkflowService(
		dagDefRepo, dagExecRepo, dagNodeExecRepo, hub, bus, nodeRegistry, effectApplier,
	)
	dagWorkflowSvc.SetTaskRepo(taskRepo)
	dagWorkflowSvc.SetRunMappingRepo(dagRunMappingRepo)
	dagWorkflowSvc.SetReviewRepo(wfReviewRepo)
	dagWorkflowSvc.SetProjectStatusLookup(projectRepo)

	// Sub-workflow wiring: persist parent↔child linkage rows, install the
	// recursion guard (walks the parent chain via the link repo), and wire
	// the DAG engine adapter. Plugin engine adapter is registered later,
	// once the workflow plugin runtime is constructed.
	parentLinkRepo := repository.NewWorkflowRunParentLinkRepository(taskRepo.DB())
	dagWorkflowSvc.SetParentLinkRepo(parentLinkRepo)
	linkRepoAdapter := service.NewSubWorkflowLinkRepoAdapter(parentLinkRepo)
	effectApplier.SubWorkflowLinks = linkRepoAdapter
	effectApplier.SubWorkflowGuard = nodetypes.NewRecursionGuard(linkRepoAdapter, dagWorkflowSvc, nodetypes.MaxSubWorkflowDepth)
	effectApplier.SubWorkflowEngines = nodetypes.NewSubWorkflowEngineRegistry(
		service.NewDAGSubWorkflowEngine(dagWorkflowSvc, dagDefRepo),
	)

	if agentSvc != nil {
		effectApplier.AgentSpawner = agentSvc
		agentSvc.SetDAGWorkflowService(dagWorkflowSvc)
	}

	// Trigger infrastructure: registrar materializes trigger-node config into
	// workflow_triggers rows on workflow save; router matches incoming events
	// and starts executions via StartOptions.
	triggerRepo := repository.NewWorkflowTriggerRepository(taskRepo.DB())
	triggerRegistrar := trigger.NewRegistrar(triggerRepo).
		WithDAGResolver(dagDefRepo)
	triggerRouter := trigger.NewRouter(
		triggerRepo,
		trigger.NoopIdempotencyStore{},
		trigger.NewDAGEngineAdapter(dagWorkflowSvc),
	)
	workflowH.SetTriggerSyncer(triggerRegistrar)
	triggerH := handler.NewTriggerHandler(triggerRouter).WithQueryRepo(triggerRepo)
	triggerH.RegisterRoutes(e)

	// Start the schedule ticker in the background. It evaluates enabled
	// source=schedule workflow_triggers every minute boundary and fires the
	// router when a trigger's cron matches. Cancellation falls back to the
	// server's shutdown context on main exit.
	go trigger.NewScheduleTicker(triggerRepo, triggerRouter, nil).Run(context.Background())

	workflowH = workflowH.WithDAGService(dagWorkflowSvc, dagDefRepo, dagExecRepo, dagNodeExecRepo).
		WithParentLinkReader(parentLinkRepo)
	// Template and review services
	templateSvc := service.NewWorkflowTemplateService(dagDefRepo, dagWorkflowSvc)
	workflowH = workflowH.WithTemplateService(templateSvc)
	workflowH = workflowH.WithReviewRepo(wfReviewRepo)
	// Construct TeamService now that templateSvc exists. Team startup delegates
	// to WorkflowTemplateService.CreateFromStrategy, which maps strategy names
	// to seeded system templates.
	if agentSvc != nil {
		teamSvc = service.NewTeamService(teamRepo, agentRunRepo, agentSvc, taskRepo, projectRepo, memorySvc, hub, bus, templateSvc, agentSvc.TeamArtifactService())
		agentSvc.SetTeamService(teamSvc)
		agentSvc.SetMemoryService(memorySvc)
	}
	// Attach team + workflow cascade hooks to the lifecycle service now that
	// both dependencies exist. Invitation revoke is left unwired pending the
	// Wave 2 invitation service.
	if teamSvc != nil {
		projectLifecycleSvc.WithTeamCanceller(teamSvc)
	}
	projectLifecycleSvc.WithWorkflowCanceller(dagWorkflowSvc)
	// Seed system templates on startup so CreateFromStrategy can resolve them.
	go func() {
		_ = templateSvc.SeedSystemTemplates(context.Background())
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

	// Employee runtime: persistent agent entity with role binding, skill overrides,
	// memory namespace, and lifecycle state.
	employeeRepo := repository.NewEmployeeRepository(taskRepo.DB())
	employeeRoleReg := employeeRoleRegistryAdapter{store: workflowRoleStore}
	var agentSpawnerForEmployee employee.AgentSpawner
	if agentSvc != nil {
		agentSpawnerForEmployee = agentSvc
	}
	employeeSvc := employee.NewService(employeeRepo, employeeRoleReg, agentSpawnerForEmployee)
	effectApplier.EmployeeSpawner = employee.ApplierAdapter{Svc: employeeSvc}

	// Seed YAML-defined Employees into every active project.
	go func() {
		seedCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		projects, err := projectRepo.ListActive(seedCtx)
		if err != nil {
			log.WithError(err).Warn("employee seed: list active projects")
			return
		}
		if len(projects) == 0 {
			return
		}
		projectIDs := make([]uuid.UUID, 0, len(projects))
		for _, p := range projects {
			projectIDs = append(projectIDs, p.ID)
		}
		empRegistry := employee.NewRegistry(employeeSvc)
		report, err := empRegistry.SeedFromDir(seedCtx, "employees", projectIDs)
		if err != nil {
			log.WithError(err).Warn("employee seed: seed from dir")
			return
		}
		log.WithFields(log.Fields{
			"upserted": report.Upserted,
			"skipped":  report.Skipped,
			"errors":   len(report.Errors),
		}).Info("employee seed complete")
	}()

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
	automationEngine.SetProjectStatusLookup(projectRepo)
	taskH = taskH.WithAutomation(automationEngine)
	customFieldH = customFieldH.WithAutomation(automationEngine)
	reviewSvc.SetAutomationEvaluator(automationEngine)
	if agentSvc != nil {
		agentSvc.SetAutomationEvaluator(automationEngine)
	}
	reviewSvc.WithExecutionPlanner(service.NewReviewExecutionPlanner(pluginSvc))
	// Wire the optional workflow-backed review path. When USE_WORKFLOW_BACKED_REVIEW
	// is unset or false (the default) the adapter is present but the flag gate
	// keeps launchWorkflowBackedReview a no-op, so the legacy flow is unchanged.
	if templateSvc != nil && dagWorkflowSvc != nil {
		reviewSvc.WithWorkflowLauncher(
			reviewWorkflowLauncherAdapter{
				templates: templateSvc,
				dag:       dagWorkflowSvc,
				defRepo:   dagDefRepo,
			},
			func() bool { return cfg.UseWorkflowBackedReview },
		)
	}
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
		dispatchSvc = service.NewTaskDispatchService(taskRepo, memberRepo, agentSvc, hub, bus, notificationSvc, taskProgressSvc)
		dispatchSvc = dispatchSvc.WithQueueWriter(agentSvc)
		dispatchSvc = dispatchSvc.WithBudgetChecker(budgetSvc)
		dispatchSvc = dispatchSvc.WithAttemptRecorder(dispatchAttemptRepo)
		dispatchSvc = dispatchSvc.WithRoleStore(workflowRoleStore)
		dispatchSvc = dispatchSvc.WithProjectStatusLookup(projectRepo)
		dispatchPreflightH = handler.NewDispatchPreflightHandler(taskRepo, memberRepo, budgetSvc, agentSvc).WithRunReader(agentSvc).WithRoleStore(workflowRoleStore)
		dispatchStatsH = handler.NewDispatchStatsHandler(dispatchAttemptRepo, agentPoolQueueRepo)
		dispatchHistoryH = handler.NewDispatchHistoryHandler(dispatchAttemptRepo)
		queueManagementH = handler.NewQueueManagementHandler(agentSvc)
		budgetQueryH = handler.NewBudgetQueryHandler(budgetSvc)
		taskH = taskH.WithDispatcher(dispatchSvc)
		agentH = agentH.WithDispatcher(dispatchSvc)
		agentH = agentH.WithAudit(auditSvc)
	}
	documentH := handler.NewDocumentHandler(documentSvc)

	// Knowledge asset handler (unified wiki + ingested documents).
	knowledgeAssetRepo := knowledge.NewPgKnowledgeAssetRepository(taskRepo.DB())
	knowledgeVersionRepo := knowledge.NewPgAssetVersionRepository(taskRepo.DB())
	knowledgeCommentRepo := knowledge.NewPgAssetCommentRepository(taskRepo.DB())
	knowledgeChunkRepo := knowledge.NewPgAssetIngestChunkRepository(taskRepo.DB())
	knowledgeBlobStorage := knowledge.NewLocalBlobStorage("./data/uploads")
	knowledgeSearchProvider := knowledge.NewPgFTSProvider(taskRepo.DB())
	knowledgeAssetSvc := knowledge.NewKnowledgeAssetService(
		knowledgeAssetRepo,
		knowledgeVersionRepo,
		knowledgeCommentRepo,
		knowledgeChunkRepo,
		knowledgeSearchProvider,
		knowledge.NoopIndexPipeline{},
		knowledgeEventBusAdapter{bus: bus},
	)
	knowledgeH := handler.NewKnowledgeAssetHandler(knowledgeAssetSvc)
	auditH := handler.NewAuditHandler(auditSvc)
	_ = knowledgeBlobStorage // available for upload service wiring when implemented

	// Live-artifact projector registry. The projection endpoint and WS
	// subscription router look projectors up by LiveArtifactKind.
	liveArtifactRegistry := liveartifact.NewRegistry()
	liveArtifactRegistry.Register(liveartifact.NewAgentRunProjector(agentRunRepo))
	liveArtifactRegistry.Register(liveartifact.NewCostSummaryProjector(agentRunRepo))
	liveArtifactRegistry.Register(liveartifact.NewReviewProjector(reviewRepo, taskRepo))
	liveArtifactRegistry.Register(liveartifact.NewTaskGroupProjector(taskRepo, savedViewRepo))
	knowledgeH = knowledgeH.WithLiveArtifactRegistry(liveArtifactRegistry)

	// Wire the WS subscription router so live-artifact block refs get
	// per-asset fan-out on entity events. The router consumes events from
	// Hub.BroadcastEvent; legacy FanoutBytes callers do not trigger it.
	liveArtifactRouter := ws.NewLiveArtifactRouter(hub, liveArtifactRegistry)
	hub.SetSubscriptionRouter(liveArtifactRouter)

	logSvc := service.NewLogService(logRepo, hub, bus)
	logH := handler.NewLogHandler(logSvc)

	costQuerySvc := service.NewCostQueryService(taskRepo, sprintRepo, agentRunRepo, budgetSvc)
	costH := handler.NewCostHandler(costQuerySvc)
	workflowRunRepo := repository.NewWorkflowPluginRunRepository()
	stepRouter := service.NewWorkflowStepRouterExecutor(agentSvc, reviewSvc, dispatchSvc).
		WithDAGChildStarter(service.NewWorkflowStepDAGChildAdapter(dagWorkflowSvc), parentLinkRepo).
		WithCrossEngineRecursionGuard(effectApplier.SubWorkflowGuard)
	workflowExec := service.NewWorkflowExecutionService(
		pluginSvc,
		workflowRunRepo,
		workflowRoleStore,
		stepRouter,
	)
	// Wire the plugin-runtime resumer so the DAG service's terminal-state hook
	// can resume a parked plugin parent when a DAG child with parent_kind
	// ='plugin_run' terminates (bridge-legacy-to-dag-invocation).
	dagWorkflowSvc.SetPluginRunResumer(workflowExec)
	// Unified workflow.run.* WS fan-out. Both engines publish lifecycle
	// transitions through this emitter in addition to their existing
	// engine-native channels (bridge-unified-run-view).
	workflowRunEmitter := service.NewWorkflowRunEventEmitter(bus)
	workflowExec.SetRunEmitter(workflowRunEmitter)
	dagWorkflowSvc.SetRunEmitter(workflowRunEmitter)
	// Cross-engine workflow run view — composes DAG executions and plugin
	// runs into a single project-scoped read surface (bridge-unified-run-view).
	workflowRunViewSvc := service.NewWorkflowRunViewService(dagExecRepo, workflowRunRepo, dagDefRepo).
		WithNodeRepo(dagNodeExecRepo).
		WithParentLinkRepo(parentLinkRepo)
	workflowRunViewH := handler.NewWorkflowRunViewHandler(workflowRunViewSvc)
	automationEngine.SetWorkflowStarter(workflowExec)
	// Register the plugin engine adapter now that the workflow plugin runtime
	// is wired. Trigger rows whose target_kind is "plugin" will dispatch here.
	triggerRouter.RegisterEngine(trigger.NewPluginEngineAdapter(workflowExec))
	triggerRegistrar.WithPluginResolver(pluginSvc)
	// Register the plugin sub-workflow engine so DAG sub_workflow nodes
	// targeting legacy workflow plugins dispatch through the same start seam
	// the trigger router uses. Also wire the terminal-state bridge so a plugin
	// run that started as a sub-workflow child resumes the parent DAG node
	// when it reaches completed/failed/cancelled.
	effectApplier.SubWorkflowEngines.Register(service.NewPluginSubWorkflowEngine(workflowExec, pluginSvc))
	workflowExec.RegisterTerminalObserver(&service.PluginSubWorkflowTerminalBridge{DAG: dagWorkflowSvc})
	taskWorkflowSvc := service.NewTaskWorkflowService(workflowRepo, hub, bus)
	taskWorkflowSvc.SetTaskRepository(taskRepo)
	taskWorkflowSvc.SetNotifier(notifRepo)
	taskWorkflowSvc.SetProgressRecorder(taskProgressSvc)
	taskWorkflowSvc.SetWorkflowRuntime(workflowExec)
	if dispatchSvc != nil {
		taskWorkflowSvc.SetDispatcher(dispatchSvc)
	}
	taskH = taskH.WithWorkflowService(taskWorkflowSvc)
	pluginH := handler.NewPluginHandler(pluginSvc).
		WithWorkflowExecution(workflowExec).
		WithParentLinkReader(parentLinkRepo)
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

	// Project templates (user-library scope — not project-scoped).
	// Authorization is caller == owner enforced at service layer.
	protected.GET("/project-templates", projectTemplateH.List)
	protected.GET("/project-templates/:id", projectTemplateH.Get)
	protected.PUT("/project-templates/:id", projectTemplateH.Update)
	protected.DELETE("/project-templates/:id", projectTemplateH.Delete)

	// Project-scoped routes
	projectMw := appMiddleware.ProjectMiddleware(projectRepo)
	projectGroup := protected.Group("/projects/:pid", projectMw)
	// Apply a group-level archived-project write guard. This sits AFTER the
	// project middleware (project in context) and BEFORE per-route RBAC.
	// The guard blocks any write (non-GET/HEAD/OPTIONS) on an archived
	// project unless the path matches a whitelisted lifecycle suffix
	// (/unarchive). Reads always pass so the archived project remains
	// inspectable (audit, settings, knowledge assets). Archive itself is
	// idempotent — the handler returns 409 AlreadyArchived if it runs on
	// an already-archived project — so it does NOT need to be whitelisted
	// here.
	projectGroup.Use(appMiddleware.ArchivedProjectWriteGuard(appMiddleware.ArchivedProjectWriteGuardConfig{
		WhitelistedSuffixes: []string{"/unarchive"},
	}))
	// Each route is tagged with its canonical ActionID via appMiddleware.Require.
	// The matrix in middleware/rbac.go is the single source of "who can do what".
	projectGroup.POST("/archive", projectH.Archive, appMiddleware.Require(appMiddleware.ActionProjectArchive))
	projectGroup.POST("/unarchive", projectH.Unarchive, appMiddleware.Require(appMiddleware.ActionProjectUnarchive))
	projectGroup.POST("/save-as-template", projectTemplateH.SaveAsTemplate, appMiddleware.Require(appMiddleware.ActionProjectSaveAsTemplate))
	projectGroup.POST("/members", memberH.Create, appMiddleware.Require(appMiddleware.ActionMemberCreate))
	projectGroup.GET("/members", memberH.List, appMiddleware.Require(appMiddleware.ActionMemberRead))
	projectGroup.POST("/members/bulk-update", memberH.BulkUpdate, appMiddleware.Require(appMiddleware.ActionMemberBulkUpdate))
	// Invitation endpoints — admin+ for every mutation; list gated by view.
	projectGroup.POST("/invitations", invitationH.Create, appMiddleware.Require(appMiddleware.ActionInvitationCreate))
	projectGroup.GET("/invitations", invitationH.List, appMiddleware.Require(appMiddleware.ActionInvitationView))
	projectGroup.POST("/invitations/:id/revoke", invitationH.Revoke, appMiddleware.Require(appMiddleware.ActionInvitationRevoke))
	projectGroup.POST("/invitations/:id/resend", invitationH.Resend, appMiddleware.Require(appMiddleware.ActionInvitationResend))
	projectGroup.POST("/tasks", taskH.Create, appMiddleware.Require(appMiddleware.ActionTaskCreate))
	projectGroup.GET("/tasks", taskH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.POST("/links", entityLinkH.Create, appMiddleware.Require(appMiddleware.ActionTaskUpdate))
	projectGroup.GET("/links", entityLinkH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.DELETE("/links/:linkId", entityLinkH.Delete, appMiddleware.Require(appMiddleware.ActionTaskUpdate))
	projectGroup.GET("/tasks/:tid/comments", taskCommentH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.POST("/tasks/:tid/comments", taskCommentH.Create, appMiddleware.Require(appMiddleware.ActionTaskComment))
	projectGroup.PATCH("/tasks/:tid/comments/:cid", taskCommentH.Update, appMiddleware.Require(appMiddleware.ActionTaskComment))
	projectGroup.DELETE("/tasks/:tid/comments/:cid", taskCommentH.Delete, appMiddleware.Require(appMiddleware.ActionTaskComment))
	projectGroup.GET("/fields", customFieldH.ListDefinitions, appMiddleware.Require(appMiddleware.ActionCustomFieldRead))
	projectGroup.POST("/fields", customFieldH.CreateDefinition, appMiddleware.Require(appMiddleware.ActionCustomFieldWrite))
	projectGroup.PUT("/fields/reorder", customFieldH.ReorderDefinitions, appMiddleware.Require(appMiddleware.ActionCustomFieldWrite))
	projectGroup.PUT("/fields/:fid", customFieldH.UpdateDefinition, appMiddleware.Require(appMiddleware.ActionCustomFieldWrite))
	projectGroup.DELETE("/fields/:fid", customFieldH.DeleteDefinition, appMiddleware.Require(appMiddleware.ActionCustomFieldWrite))
	projectGroup.GET("/tasks/:tid/fields", customFieldH.ListTaskValues, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.PUT("/tasks/:tid/fields/:fid", customFieldH.SetTaskValue, appMiddleware.Require(appMiddleware.ActionTaskUpdate))
	projectGroup.DELETE("/tasks/:tid/fields/:fid", customFieldH.ClearTaskValue, appMiddleware.Require(appMiddleware.ActionTaskUpdate))
	projectGroup.GET("/views", savedViewH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.POST("/views", savedViewH.Create, appMiddleware.Require(appMiddleware.ActionSavedViewWrite))
	projectGroup.PUT("/views/:vid", savedViewH.Update, appMiddleware.Require(appMiddleware.ActionSavedViewWrite))
	projectGroup.DELETE("/views/:vid", savedViewH.Delete, appMiddleware.Require(appMiddleware.ActionSavedViewWrite))
	projectGroup.POST("/views/:vid/default", savedViewH.SetDefault, appMiddleware.Require(appMiddleware.ActionSavedViewWrite))
	projectGroup.GET("/forms", formH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.POST("/forms", formH.Create, appMiddleware.Require(appMiddleware.ActionFormWrite))
	projectGroup.PUT("/forms/:formId", formH.Update, appMiddleware.Require(appMiddleware.ActionFormWrite))
	projectGroup.DELETE("/forms/:formId", formH.Delete, appMiddleware.Require(appMiddleware.ActionFormWrite))
	projectGroup.GET("/automations", automationH.ListRules, appMiddleware.Require(appMiddleware.ActionAutomationRead))
	projectGroup.POST("/automations", automationH.CreateRule, appMiddleware.Require(appMiddleware.ActionAutomationWrite))
	projectGroup.PUT("/automations/:rid", automationH.UpdateRule, appMiddleware.Require(appMiddleware.ActionAutomationWrite))
	projectGroup.DELETE("/automations/:rid", automationH.DeleteRule, appMiddleware.Require(appMiddleware.ActionAutomationWrite))
	projectGroup.GET("/automations/logs", automationH.ListLogs, appMiddleware.Require(appMiddleware.ActionAutomationRead))
	projectGroup.GET("/dashboards", dashboardH.List, appMiddleware.Require(appMiddleware.ActionDashboardRead))
	projectGroup.POST("/dashboards", dashboardH.Create, appMiddleware.Require(appMiddleware.ActionDashboardWrite))
	projectGroup.PUT("/dashboards/:did", dashboardH.Update, appMiddleware.Require(appMiddleware.ActionDashboardWrite))
	projectGroup.DELETE("/dashboards/:did", dashboardH.Delete, appMiddleware.Require(appMiddleware.ActionDashboardWrite))
	projectGroup.GET("/dashboards/:did/widgets", dashboardH.ListWidgets, appMiddleware.Require(appMiddleware.ActionDashboardRead))
	projectGroup.POST("/dashboards/:did/widgets", dashboardH.SaveWidget, appMiddleware.Require(appMiddleware.ActionDashboardWrite))
	projectGroup.DELETE("/dashboards/:did/widgets/:wid", dashboardH.DeleteWidget, appMiddleware.Require(appMiddleware.ActionDashboardWrite))
	projectGroup.GET("/dashboard/widgets/:type", dashboardH.WidgetData, appMiddleware.Require(appMiddleware.ActionDashboardRead))
	projectGroup.GET("/milestones", milestoneH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.POST("/milestones", milestoneH.Create, appMiddleware.Require(appMiddleware.ActionMilestoneWrite))
	projectGroup.PUT("/milestones/:mid", milestoneH.Update, appMiddleware.Require(appMiddleware.ActionMilestoneWrite))
	projectGroup.DELETE("/milestones/:mid", milestoneH.Delete, appMiddleware.Require(appMiddleware.ActionMilestoneWrite))
	projectGroup.POST("/sprints", sprintH.Create, appMiddleware.Require(appMiddleware.ActionSprintWrite))
	projectGroup.GET("/sprints", sprintH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.PUT("/sprints/:sid", sprintH.Update, appMiddleware.Require(appMiddleware.ActionSprintWrite))
	projectGroup.GET("/sprints/:sid/metrics", sprintH.Metrics, appMiddleware.Require(appMiddleware.ActionTaskRead))
	projectGroup.GET("/workflow", workflowH.Get, appMiddleware.Require(appMiddleware.ActionWorkflowRead))
	projectGroup.PUT("/workflow", workflowH.Put, appMiddleware.Require(appMiddleware.ActionWorkflowWrite))
	projectGroup.POST("/workflows", workflowH.CreateDefinition, appMiddleware.Require(appMiddleware.ActionWorkflowWrite))
	projectGroup.GET("/workflows", workflowH.ListDefinitions, appMiddleware.Require(appMiddleware.ActionWorkflowRead))
	projectGroup.POST("/memory", memoryH.Store, appMiddleware.Require(appMiddleware.ActionMemoryWrite))
	projectGroup.GET("/memory", memoryH.Search, appMiddleware.Require(appMiddleware.ActionMemoryRead))
	projectGroup.GET("/memory/stats", memoryH.Stats, appMiddleware.Require(appMiddleware.ActionMemoryRead))
	projectGroup.GET("/memory/export", memoryH.Export, appMiddleware.Require(appMiddleware.ActionMemoryRead))
	projectGroup.POST("/memory/bulk-delete", memoryH.BulkDelete, appMiddleware.Require(appMiddleware.ActionMemoryWrite))
	projectGroup.POST("/memory/cleanup", memoryH.Cleanup, appMiddleware.Require(appMiddleware.ActionMemoryWrite))
	projectGroup.GET("/memory/:mid", memoryH.Get, appMiddleware.Require(appMiddleware.ActionMemoryRead))
	projectGroup.PATCH("/memory/:mid", memoryH.Update, appMiddleware.Require(appMiddleware.ActionMemoryWrite))
	projectGroup.DELETE("/memory/:mid", memoryH.Delete, appMiddleware.Require(appMiddleware.ActionMemoryWrite))
	// Old /documents routes removed; ingested files are now served via /knowledge/assets.
	_ = documentH
	projectGroup.GET("/logs", logH.List, appMiddleware.Require(appMiddleware.ActionLogRead))
	projectGroup.POST("/logs", logH.Create, appMiddleware.Require(appMiddleware.ActionLogWrite))
	if dispatchPreflightH != nil {
		projectGroup.GET("/dispatch/preflight", dispatchPreflightH.Get, appMiddleware.Require(appMiddleware.ActionTaskRead))
	}
	if dispatchStatsH != nil {
		projectGroup.GET("/dispatch/stats", dispatchStatsH.Get, appMiddleware.Require(appMiddleware.ActionTaskRead))
	}
	if queueManagementH != nil {
		projectGroup.GET("/queue", queueManagementH.List, appMiddleware.Require(appMiddleware.ActionTaskRead))
		projectGroup.DELETE("/queue/:entryId", queueManagementH.Cancel, appMiddleware.Require(appMiddleware.ActionTaskDispatch))
	}
	if budgetQueryH != nil {
		projectGroup.GET("/budget/summary", budgetQueryH.ProjectSummary, appMiddleware.Require(appMiddleware.ActionDashboardRead))
	}
	// Knowledge asset routes (unified: replaces /wiki and /documents).
	projectGroup.GET("/knowledge/assets", knowledgeH.ListAssets, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.POST("/knowledge/assets", knowledgeH.CreateAsset, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.GET("/knowledge/assets/tree", knowledgeH.GetTree, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.GET("/knowledge/search", knowledgeH.Search, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.GET("/knowledge/assets/:id", knowledgeH.GetAsset, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.PUT("/knowledge/assets/:id", knowledgeH.UpdateAsset, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.DELETE("/knowledge/assets/:id", knowledgeH.DeleteAsset, appMiddleware.Require(appMiddleware.ActionWikiDelete))
	projectGroup.POST("/knowledge/assets/:id/restore", knowledgeH.RestoreAsset, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.PATCH("/knowledge/assets/:id/move", knowledgeH.MoveAsset, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.POST("/knowledge/assets/:id/reupload", knowledgeH.ReuploadAsset, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.POST("/knowledge/assets/:id/materialize-as-wiki", knowledgeH.MaterializeAsWiki, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.GET("/knowledge/assets/:id/versions", knowledgeH.ListVersions, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.POST("/knowledge/assets/:id/versions", knowledgeH.CreateVersion, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.GET("/knowledge/assets/:id/versions/:vid", knowledgeH.GetVersion, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.POST("/knowledge/assets/:id/versions/:vid/restore", knowledgeH.RestoreVersion, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.GET("/knowledge/assets/:id/comments", knowledgeH.ListComments, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.POST("/knowledge/assets/:id/comments", knowledgeH.CreateComment, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.PATCH("/knowledge/assets/:id/comments/:cid", knowledgeH.UpdateComment, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.DELETE("/knowledge/assets/:id/comments/:cid", knowledgeH.DeleteComment, appMiddleware.Require(appMiddleware.ActionWikiWrite))
	projectGroup.POST("/knowledge/assets/:id/decompose-tasks", docDecomposeH.Decompose, appMiddleware.Require(appMiddleware.ActionTaskCreate))
	projectGroup.POST("/knowledge/assets/:id/live-artifacts/project", knowledgeH.ProjectLiveArtifacts, appMiddleware.Require(appMiddleware.ActionWikiRead))
	projectGroup.POST("/knowledge/assets/:id/live-artifacts/:blockId/freeze", knowledgeH.FreezeLiveArtifact, appMiddleware.Require(appMiddleware.ActionWikiWrite))

	// Audit log query API. Both routes are gated by the canonical
	// audit.read ActionID (admin+).
	projectGroup.GET("/audit-events", auditH.List, appMiddleware.Require(appMiddleware.ActionAuditRead))
	projectGroup.GET("/audit-events/:eventId", auditH.Get, appMiddleware.Require(appMiddleware.ActionAuditRead))

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

	// Invitation public surface. by-token is anonymous read, decline is
	// anonymous-allowed, accept requires auth.
	v1.GET("/invitations/by-token/:token", invitationH.GetByToken)
	v1.POST("/invitations/decline", invitationH.Decline)
	protected.POST("/invitations/accept", invitationH.Accept)

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
	projectGroup.GET("/workflow-reviews", workflowH.ListPendingReviews, appMiddleware.Require(appMiddleware.ActionWorkflowRead))

	// Unified workflow-run list + detail (cross-engine: DAG executions and
	// legacy workflow plugin runs through the same project-scoped lens).
	projectGroup.GET("/workflow-runs", workflowRunViewH.List, appMiddleware.Require(appMiddleware.ActionWorkflowRead))
	projectGroup.GET("/workflow-runs/:engine/:id", workflowRunViewH.Detail, appMiddleware.Require(appMiddleware.ActionWorkflowRead))

	// Employee resource (persistent agent entities with role binding and lifecycle state).
	employeeH := handler.NewEmployeeHandler(employeeSvc)
	employeeH.Register(projectGroup)

	// Per-employee unified runs feed (workflow_executions ∪ agent_runs).
	// Route is global (not project-scoped) because the employee id is
	// self-disambiguating and the FE drills down from the employee detail
	// shell, not from a project picker. Project RBAC is enforced by the
	// existing JWT middleware on `protected`.
	employeeRunsRepo := repository.NewEmployeeRunsRepository(taskRepo.DB())
	employeeRunsH := handler.NewEmployeeRunsHandler(employeeRunsRepo)
	protected.GET("/employees/:id/runs", employeeRunsH.List)

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
		WithWorkflowTemplateRepo(dagDefRepo).
		WithProjectTemplateInstaller(projectTemplateSvc)
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
	if imReactionEventRepo != nil {
		imSvc.SetReactionStore(service.NewIMReactionStoreAdapter(imReactionEventRepo))
	}
	imActionExecutor := service.NewBackendIMActionExecutor(
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
	)
	if dagWorkflowSvc != nil {
		imActionExecutor = imActionExecutor.WithReviewWorkflow(dagWorkflowSvc, wfReviewRepo)
	}
	imSvc.SetActionExecutor(imActionExecutor)
	automationEngine.SetIMSender(imSvc)
	wikiSvc.WithIMForwarder(imSvc, cfg.IMNotifyPlatform, cfg.IMNotifyTargetChatID).WithIMChannelResolver(imControlPlane)
	imH := handler.NewIMHandler(imSvc)
	imControlH := handler.NewIMControlHandler(imControlPlane, imSvc)
	v1.POST("/im/message", imH.HandleMessage)
	v1.POST("/im/command", imH.HandleCommand)
	v1.POST("/intent", imH.HandleIntent)
	v1.POST("/im/action", imH.HandleAction)
	v1.POST("/im/reactions", imH.HandleReaction)
	v1.POST("/im/reactions/shortcuts", imH.BindReactionShortcut)
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
		AuditSink:    auditSink,
		Invitation:   invitationSvc,
	}
}
