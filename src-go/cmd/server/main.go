package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/server"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/version"
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
	agentRunRepo := repository.NewAgentRunRepository(db)
	notifRepo := repository.NewNotificationRepository(db)

	// Create Echo instance and register routes
	e := server.New(cfg, cacheRepo)
	server.RegisterRoutes(e, cfg, authSvc, cacheRepo,
		projectRepo, memberRepo, sprintRepo, taskRepo, agentRunRepo, notifRepo,
	)

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("shutting down server...")
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
