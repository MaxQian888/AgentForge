package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/role"
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
	var logHandler slog.Handler
	if cfg.Env == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(logHandler))

	slog.Info("starting server",
		"version", version.Version,
		"commit", version.Commit,
		"buildDate", version.BuildDate,
		"env", cfg.Env,
	)

	// Dev fallback: auto-generate a secret so the server starts without config.
	// NEVER use this in production.
	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			slog.Error("JWT_SECRET environment variable is required in production")
			os.Exit(1)
		}
		cfg.JWTSecret = "dev-secret-change-me-in-production-32ch"
		slog.Warn("JWT_SECRET not set — using insecure dev default")
	}

	// Connect to PostgreSQL (optional — server starts in degraded mode if unavailable)
	db, err := database.NewPostgres(cfg.PostgresURL)
	if err != nil {
		slog.Warn("PostgreSQL unavailable, auth endpoints will not work", "error", err)
		db = nil
	}

	// Connect to Redis (optional)
	rdb, err := database.NewRedis(cfg.RedisURL)
	if err != nil {
		slog.Warn("Redis unavailable, token cache disabled", "error", err)
		rdb = nil
	}

	// Run database migrations if DB is available
	if db != nil {
		if err := database.RunMigrations(cfg.PostgresURL, migrations.FS); err != nil {
			slog.Warn("migration error", "error", err)
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
	taskProgressRepo := repository.NewTaskProgressRepository(db)
	agentRunRepo := repository.NewAgentRunRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	reviewRepo := repository.NewReviewRepository(db)
	hub := ws.NewHub()
	go hub.Run()
	bridgeClient := bridge.NewClient(cfg.BridgeURL)
	worktreeMgr := worktree.NewManager(cfg.WorktreeBasePath, cfg.RepoBasePath, cfg.MaxActiveAgents)
	runStartupWorktreeSweep(cfg, worktreeMgr)
	roleStore := role.NewFileStore(cfg.RolesDir)
	agentSvc := service.NewAgentService(agentRunRepo, taskRepo, projectRepo, hub, bridgeClient, worktreeMgr, roleStore)

	// Create Echo instance and register routes
	e := server.New(cfg, cacheRepo)
	taskProgressSvc := server.RegisterRoutes(e, cfg, authSvc, cacheRepo,
		projectRepo, memberRepo, sprintRepo, taskRepo, taskProgressRepo, agentRunRepo, notifRepo, reviewRepo, hub, bridgeClient, agentSvc,
	)
	detectorCtx, detectorCancel := context.WithCancel(context.Background())
	defer detectorCancel()
	if taskProgressSvc != nil {
		go runTaskProgressDetector(detectorCtx, cfg.TaskProgressDetectorInterval, taskProgressSvc)
	}

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("shutting down server...")
		detectorCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := e.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
		if db != nil {
			db.Close()
		}
		if rdb != nil {
			_ = rdb.Close()
		}
	}()

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server listening", "addr", addr)

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		slog.Error("server start failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
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
		slog.Warn("worktree startup sweep skipped", "error", err)
		return
	}

	for _, projectSlug := range projects {
		report, err := summarizeStartupWorktreeProject(ctx, manager, projectSlug)
		if err != nil {
			slog.Warn("worktree startup sweep failed", "project", projectSlug, "error", err)
			continue
		}
		if report.TotalBefore == 0 && report.TotalAfter == 0 && report.Cleaned == 0 {
			continue
		}
		slog.Info(
			"worktree startup sweep inventory",
			"project", report.ProjectSlug,
			"total_before", report.TotalBefore,
			"managed_before", report.ManagedBefore,
			"stale_before", report.StaleBefore,
			"cleaned", report.Cleaned,
			"total_after", report.TotalAfter,
			"managed_after", report.ManagedAfter,
			"stale_after", report.StaleAfter,
		)
		if report.StaleAfter > 0 {
			slog.Warn("worktree startup sweep left stale state behind", "project", report.ProjectSlug, "stale_after", report.StaleAfter)
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

func runTaskProgressDetector(ctx context.Context, interval time.Duration, progressSvc *service.TaskProgressService) {
	if progressSvc == nil || interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := progressSvc.EvaluateOpenTasks(ctx); err != nil {
				slog.Warn("task progress detector tick failed", "error", err)
			}
		}
	}
}
