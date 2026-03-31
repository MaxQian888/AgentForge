package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/marketplace/internal/config"
	"github.com/agentforge/marketplace/internal/handler"
	appI18n "github.com/agentforge/marketplace/internal/i18n"
	"github.com/agentforge/marketplace/internal/repository"
	"github.com/agentforge/marketplace/internal/server"
	"github.com/agentforge/marketplace/internal/service"
	"github.com/agentforge/marketplace/migrations"
	"github.com/agentforge/marketplace/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Set up structured logging.
	log.SetOutput(os.Stdout)
	if cfg.Env == "production" {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetLevel(log.WarnLevel)
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
		log.SetLevel(log.DebugLevel)
	}

	if cfg.JWTSecret == "" {
		if cfg.Env == "production" {
			log.Fatal("JWT_SECRET must be set in production")
		}
		cfg.JWTSecret = "dev-secret-change-me-in-production-32ch"
		log.Warn("JWT_SECRET not set — using insecure dev default")
	}

	if cfg.PostgresURL == "" {
		log.Fatal("POSTGRES_URL must be set")
	}

	db, err := database.NewPostgres(cfg.PostgresURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Info("PostgreSQL connected")

	if err := database.RunMigrations(cfg.PostgresURL, migrations.FS); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	if err := os.MkdirAll(cfg.ArtifactsDir, 0o755); err != nil {
		log.Fatalf("failed to create artifacts dir: %v", err)
	}

	// Wire dependencies.
	itemRepo := repository.NewMarketplaceItemRepository(db)
	reviewRepo := repository.NewMarketplaceReviewRepository(db)
	svc := service.NewMarketplaceService(itemRepo, reviewRepo, cfg.ArtifactsDir)

	itemH := handler.NewItemHandler(svc)
	versionH := handler.NewVersionHandler(svc)
	reviewH := handler.NewReviewHandler(svc)
	adminH := handler.NewAdminHandler(svc, cfg.AdminUserIDs)

	// Initialize i18n.
	appI18n.Init()

	e := server.New(cfg)
	server.RegisterRoutes(e, cfg, itemH, versionH, reviewH, adminH)

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%s", cfg.Port)
		log.WithField("addr", addr).Info("marketplace server listening")
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("server start failed")
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.WithError(err).Error("server shutdown error")
	}

	if err := database.ClosePostgres(db); err != nil {
		log.WithError(err).Warn("postgres close error")
	}

	log.Info("server stopped")
}
