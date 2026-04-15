package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/config"
	appI18n "github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	pluginruntime "github.com/react-go-quick-starter/server/internal/plugin"
	"github.com/react-go-quick-starter/server/internal/pool"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/scheduler"
	"github.com/react-go-quick-starter/server/internal/server"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/version"
	"github.com/react-go-quick-starter/server/internal/worktree"
	"github.com/react-go-quick-starter/server/internal/ws"
	"github.com/react-go-quick-starter/server/migrations"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func main() {
	// CLI flags — override env vars when passed (e.g. by Tauri sidecar)
	portFlag := flag.String("port", "", "HTTP port to listen on (overrides PORT env var)")
	flag.Parse()

	// Apply --port flag before loading config
	if *portFlag != "" {
		_ = os.Setenv("PORT", *portFlag)
	}

	cfg := config.Load()

	// Set up structured logging
	log.SetOutput(os.Stdout)
	if cfg.Env == "production" {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetLevel(log.WarnLevel)
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
		log.SetLevel(log.DebugLevel)
	}

	log.WithFields(log.Fields{
		"version":   version.Version,
		"commit":    version.Commit,
		"buildDate": version.BuildDate,
		"env":       cfg.Env,
	}).Info("starting server")

	// Dev fallback: auto-generate a secret so the server starts without config.
	// NEVER use this in production.
	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			log.Error("JWT_SECRET environment variable is required in production")
			os.Exit(1)
		}
		cfg.JWTSecret = "dev-secret-change-me-in-production-32ch"
		log.Warn("JWT_SECRET not set — using insecure dev default")
	}

	// Connect to PostgreSQL (optional — server starts in degraded mode if unavailable)
	db, err := database.NewPostgres(cfg.PostgresURL)
	if err != nil {
		log.WithError(err).Warn("PostgreSQL unavailable, auth endpoints will not work")
		db = nil
	} else {
		log.Info("PostgreSQL connected")
	}

	// Connect to Redis (optional)
	rdb, err := database.NewRedis(cfg.RedisURL)
	if err != nil {
		log.WithError(err).Warn("Redis unavailable, token cache disabled")
		rdb = nil
	} else {
		log.Info("Redis connected")
	}

	// Run database migrations if DB is available
	if db != nil {
		if err := database.RunMigrations(cfg.PostgresURL, migrations.FS); err != nil {
			log.WithError(err).Warn("migration error")
		}
	}

	// Wire up dependencies
	userRepo := repository.NewUserRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)
	authSvc := service.NewAuthService(userRepo, cacheRepo, cfg)

	projectRepo := repository.NewProjectRepository(db)
	memberRepo := repository.NewMemberRepository(db)
	sprintRepo := repository.NewSprintRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	entityLinkRepo := repository.NewEntityLinkRepository(db)
	taskCommentRepo := repository.NewTaskCommentRepository(db)
	imReactionEventRepo := repository.NewIMReactionEventRepository(db)
	customFieldRepo := repository.NewCustomFieldRepository(db)
	savedViewRepo := repository.NewSavedViewRepository(db)
	formRepo := repository.NewFormRepository(db)
	automationRuleRepo := repository.NewAutomationRuleRepository(db)
	automationLogRepo := repository.NewAutomationLogRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)
	milestoneRepo := repository.NewMilestoneRepository(db)
	taskProgressRepo := repository.NewTaskProgressRepository(db)
	agentRunRepo := repository.NewAgentRunRepository(db)
	agentPoolQueueRepo := repository.NewAgentPoolQueueRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	reviewRepo := repository.NewReviewRepository(db)
	reviewAggRepo := repository.NewReviewAggregationRepository(db)
	falsePosRepo := repository.NewFalsePositiveRepository(db)
	workflowRepo := repository.NewWorkflowRepository(db)
	teamRepo := repository.NewAgentTeamRepository(db)
	memoryRepo := repository.NewAgentMemoryRepository(db)
	wikiSpaceRepo := repository.NewWikiSpaceRepository(db)
	wikiPageRepo := repository.NewWikiPageRepository(db)
	pageVersionRepo := repository.NewPageVersionRepository(db)
	pageCommentRepo := repository.NewPageCommentRepository(db)
	pageFavoriteRepo := repository.NewPageFavoriteRepository(db)
	pageRecentAccessRepo := repository.NewPageRecentAccessRepository(db)
	scheduledJobRepo := repository.NewScheduledJobRepository(db)
	scheduledJobRunRepo := repository.NewScheduledJobRunRepository(db)
	hub := ws.NewHub()
	go hub.Run()
	bridgeClient := bridge.NewClient(cfg.BridgeURL)
	bridgeHealthCtx, bridgeHealthCancel := context.WithCancel(context.Background())
	bridgeHealthSvc := service.NewBridgeHealthService(bridgeClient)
	bridgeHealthSvc.Start(bridgeHealthCtx)
	worktreeMgr := worktree.NewManager(cfg.WorktreeBasePath, cfg.RepoBasePath, cfg.MaxActiveAgents)
	runStartupWorktreeSweep(cfg, worktreeMgr)
	roleStore := role.NewFileStore(cfg.RolesDir)
	agentSvc := service.NewAgentService(agentRunRepo, taskRepo, projectRepo, hub, bridgeClient, worktreeMgr, roleStore)
	agentSvc.SetBridgeHealth(bridgeHealthSvc)
	agentSvc.SetPool(pool.NewPool(cfg.MaxActiveAgents))
	agentSvc.SetQueueStore(agentPoolQueueRepo)
	teamArtifactRepo := repository.NewTeamArtifactRepository(db)
	teamArtifactSvc := service.NewTeamArtifactService(teamArtifactRepo)
	agentSvc.SetTeamArtifactService(teamArtifactSvc)
	pluginSvc := service.NewPluginService(
		repository.NewPluginRegistryRepository(db),
		bridgeClient,
		pluginruntime.NewWASMRuntimeManager(),
		cfg.PluginsDir,
	).
		WithInstanceStore(repository.NewPluginInstanceRepository(db)).
		WithEventStore(repository.NewPluginEventRepository(db)).
		WithBroadcaster(ws.NewPluginEventBroadcaster(hub))
	if cfg.PluginRegistryURL != "" {
		pluginSvc.SetRemoteRegistry(service.NewHTTPRemoteRegistryClient(http.DefaultClient), cfg.PluginRegistryURL)
	}
	schedulerRegistry := scheduler.NewRegistry(scheduledJobRepo, scheduler.BuiltInCatalog(scheduler.CatalogConfig{
		TaskProgressDetectorInterval: cfg.TaskProgressDetectorInterval,
		ExecutionMode:                schedulerExecutionMode(cfg.SchedulerExecutionMode),
	}))
	schedulerSvc := scheduler.NewService(scheduledJobRepo, scheduledJobRunRepo)
	schedulerSvc.SetBroadcaster(ws.NewSchedulerEventBroadcaster(hub))
	if _, err := schedulerRegistry.Reconcile(context.Background()); err != nil {
		log.WithError(err).Warn("scheduler registry reconcile failed")
	} else {
		log.WithField("executionMode", cfg.SchedulerExecutionMode).Info("scheduler registry reconciled")
	}

	// Initialize i18n
	appI18n.Init()

	// Create Echo instance and register routes
	e := server.New(cfg, cacheRepo)
	routeServices := server.RegisterRoutes(
		e,
		cfg,
		authSvc,
		cacheRepo,
		projectRepo,
		memberRepo,
		sprintRepo,
		taskRepo,
		entityLinkRepo,
		taskCommentRepo,
		imReactionEventRepo,
		customFieldRepo,
		savedViewRepo,
		formRepo,
		automationRuleRepo,
		automationLogRepo,
		dashboardRepo,
		milestoneRepo,
		taskProgressRepo,
		agentRunRepo,
		agentPoolQueueRepo,
		repository.NewDispatchAttemptRepository(db),
		notifRepo,
		reviewRepo,
		reviewAggRepo,
		falsePosRepo,
		workflowRepo,
		teamRepo,
		memoryRepo,
		wikiSpaceRepo,
		wikiPageRepo,
		pageVersionRepo,
		pageCommentRepo,
		pageFavoriteRepo,
		pageRecentAccessRepo,
		repository.NewDocumentRepo(db),
		repository.NewLogRepository(db),
		hub,
		bridgeClient,
		bridgeHealthSvc,
		pluginSvc,
		agentSvc,
		schedulerSvc,
	)
	taskProgressSvc := routeServices.TaskProgress
	automationSchedulerEngine := routeServices.Automation
	if taskProgressSvc != nil {
		schedulerSvc.RegisterHandler("task-progress-detector", scheduler.NewTaskProgressDetectorHandler(taskProgressSvc))
	}
	schedulerSvc.RegisterHandler("automation-due-date-detector", scheduler.NewAutomationDueDateDetectorHandler(automationSchedulerEngine, 24*time.Hour))
	schedulerSvc.RegisterHandler(
		"worktree-garbage-collector",
		scheduler.NewWorktreeGarbageCollectorHandler(
			scheduler.FileProjectSource{RepoBasePath: cfg.RepoBasePath},
			worktreeMgr,
		),
	)
	schedulerSvc.RegisterHandler("bridge-health-reconcile", scheduler.NewBridgeHealthReconcileHandler(bridgeClient))
	schedulerSvc.RegisterHandler(
		"cost-reconcile",
		scheduler.NewCostReconcileHandler(projectRepo, taskRepo, teamRepo, agentRunRepo),
	)
	schedulerSvc.RegisterHandler("scheduler-history-retention", scheduler.NewHistoryRetentionHandler(schedulerSvc))
	log.WithFields(log.Fields{
		"bridgeUrl":         cfg.BridgeURL,
		"rolesDir":          cfg.RolesDir,
		"pluginsDir":        cfg.PluginsDir,
		"pluginRegistryUrl": cfg.PluginRegistryURL,
		"schedulerInterval": "15s",
	}).Info("backend dependencies wired")
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	defer schedulerCancel()
	go scheduler.RunLoop(schedulerCtx, 15*time.Second, schedulerSvc)

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info("shutting down server...")
		bridgeHealthCancel()
		schedulerCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			log.WithError(err).Error("server shutdown error")
		}
		if db != nil {
			if err := database.ClosePostgres(db); err != nil {
				log.WithError(err).Warn("postgres close error")
			}
		}
		if rdb != nil {
			_ = rdb.Close()
		}
	}()

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.WithField("addr", addr).Info("server listening")

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Error("server start failed")
		os.Exit(1)
	}

	log.Info("server stopped")
}

type startupSweepManager interface {
	Inventory(ctx context.Context, projectSlug string) (*worktree.Inventory, error)
	GarbageCollectAll(ctx context.Context, projectSlug string) ([]worktree.Inspection, error)
}

type startupWorktreeSweepReport struct {
	ProjectSlug   string
	TotalBefore   int
	ManagedBefore int
	StaleBefore   int
	Cleaned       int
	TotalAfter    int
	ManagedAfter  int
	StaleAfter    int
}

func runStartupWorktreeSweep(cfg *config.Config, manager *worktree.Manager) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projects, err := collectStartupWorktreeProjects(cfg.RepoBasePath)
	if err != nil {
		log.WithError(err).Warn("worktree startup sweep skipped")
		return
	}

	for _, projectSlug := range projects {
		report, err := summarizeStartupWorktreeProject(ctx, manager, projectSlug)
		if err != nil {
			log.WithError(err).WithField("project", projectSlug).Warn("worktree startup sweep failed")
			continue
		}
		if report.TotalBefore == 0 && report.TotalAfter == 0 && report.Cleaned == 0 {
			continue
		}
		log.WithFields(log.Fields{
			"project":        report.ProjectSlug,
			"total_before":   report.TotalBefore,
			"managed_before": report.ManagedBefore,
			"stale_before":   report.StaleBefore,
			"cleaned":        report.Cleaned,
			"total_after":    report.TotalAfter,
			"managed_after":  report.ManagedAfter,
			"stale_after":    report.StaleAfter,
		}).Info("worktree startup sweep inventory")
		if report.StaleAfter > 0 {
			log.WithFields(log.Fields{"project": report.ProjectSlug, "stale_after": report.StaleAfter}).Warn("worktree startup sweep left stale state behind")
		}
	}
}

func collectStartupWorktreeProjects(repoBasePath string) ([]string, error) {
	repoEntries, err := os.ReadDir(repoBasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	projects := make([]string, 0, len(repoEntries))
	for _, entry := range repoEntries {
		if !entry.IsDir() {
			continue
		}
		projects = append(projects, entry.Name())
	}
	sort.Strings(projects)
	return projects, nil
}

func summarizeStartupWorktreeProject(ctx context.Context, manager startupSweepManager, projectSlug string) (*startupWorktreeSweepReport, error) {
	before, err := manager.Inventory(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	cleaned, err := manager.GarbageCollectAll(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	after, err := manager.Inventory(ctx, projectSlug)
	if err != nil {
		return nil, err
	}

	return &startupWorktreeSweepReport{
		ProjectSlug:   projectSlug,
		TotalBefore:   before.Total,
		ManagedBefore: before.Managed,
		StaleBefore:   before.Stale,
		Cleaned:       len(cleaned),
		TotalAfter:    after.Total,
		ManagedAfter:  after.Managed,
		StaleAfter:    after.Stale,
	}, nil
}

func schedulerExecutionMode(value string) model.ScheduledJobExecutionMode {
	switch value {
	case string(model.ScheduledJobExecutionModeOSRegistered):
		return model.ScheduledJobExecutionModeOSRegistered
	default:
		return model.ScheduledJobExecutionModeInProcess
	}
}
