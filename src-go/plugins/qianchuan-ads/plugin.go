package qianchuanads

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	qianchuanprov "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/adsplatform"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/eventbus"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/binding"
	qchandler "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/handler"
	qianchuanoauth "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/oauth"
	qcrepo "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/repo"
	qcservice "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/service"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/strategy"
	qcworkflow "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/workflow"
	"github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
	"github.com/react-go-quick-starter/server/internal/secrets"
	"github.com/react-go-quick-starter/server/internal/service"
)

// qianchuanPluginID mirrors plugins/integrations/qianchuan-ads/manifest.yaml.
// The Qianchuan (live-commerce ads) feature is a first-party integration
// plugin; its implementation currently lives in-proc in the Go binary, but
// it is isolated behind this file so core routing has no hard dependency on
// it. Set AGENTFORGE_PLUGIN_QIANCHUAN=disabled to skip registration.
const qianchuanPluginID = "qianchuan-ads"

func qianchuanPluginEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENTFORGE_PLUGIN_QIANCHUAN"))) {
	case "", "1", "true", "yes", "on", "enabled":
		return true
	}
	return false
}

type qianchuanBindingsAuditEmitter struct{ svc *service.AuditService }

func (e qianchuanBindingsAuditEmitter) Emit(
	ctx context.Context,
	projectID, actorUserID, bindingID uuid.UUID,
	action, payload string,
) {
	if e.svc == nil {
		return
	}
	var actor *uuid.UUID
	if actorUserID != uuid.Nil {
		a := actorUserID
		actor = &a
	}
	if payload == "" {
		payload = "{}"
	}
	event := &model.AuditEvent{
		ID:                  uuid.New(),
		ProjectID:           projectID,
		OccurredAt:          time.Now().UTC(),
		ActorUserID:         actor,
		ActionID:            action,
		ResourceType:        model.AuditResourceTypeQianchuanBinding,
		ResourceID:          bindingID.String(),
		PayloadSnapshotJSON: payload,
	}
	_ = e.svc.RecordEvent(ctx, event)
}

type InstallDeps struct {
	DB           *gorm.DB
	Cfg          *config.Config
	Secrets      *secrets.Service
	Audit        *service.AuditService
	Bus          eventbus.Publisher
	Echo         *echo.Echo
	Protected    *echo.Group
	ProjectGroup *echo.Group
	// NodeRegistry is the workflow node-type registry the plugin's
	// handlers (qianchuan_metrics_fetcher, _strategy_runner,
	// _action_executor) register into. Must be provided before
	// LockGlobal() is called on the same registry.
	NodeRegistry *nodetypes.NodeTypeRegistry
	// PluginSvc lets the plugin self-register in the control-plane
	// registry so GET /api/v1/plugins lists it with lifecycle_state=active
	// when enabled. Optional — when nil, routes are still wired but the
	// plugin won't appear in the control-plane inventory.
	PluginSvc *service.PluginService
}

// qianchuanManifestPath locates the first-party manifest relative to the
// backend working directory. Callers who run the binary from a different
// cwd can override it with AGENTFORGE_QIANCHUAN_MANIFEST.
func qianchuanManifestPath() string {
	if override := strings.TrimSpace(os.Getenv("AGENTFORGE_QIANCHUAN_MANIFEST")); override != "" {
		return override
	}
	return "plugins/integrations/qianchuan-ads/manifest.yaml"
}

// installQianchuanPlugin wires all Qianchuan-specific HTTP routes, services,
// and background loops. Returns the OAuth token refresher so main.go can
// supervise its lifecycle, or nil if the plugin is disabled.
func Install(deps InstallDeps) *qianchuanoauth.Refresher {
	if !qianchuanPluginEnabled() {
		log.WithField("plugin", qianchuanPluginID).Info("plugin disabled via AGENTFORGE_PLUGIN_QIANCHUAN — skipping registration")
		return nil
	}
	log.WithField("plugin", qianchuanPluginID).Info("enabling first-party integration plugin")

	emitter := qianchuanBindingsAuditEmitter{svc: deps.Audit}

	qcBindingRepo := qianchuanbinding.NewGormRepo(deps.DB)
	qcRegistry := adsplatform.NewRegistry()
	qianchuanprov.Register(qcRegistry)
	qcProvider, qcProviderErr := qcRegistry.Resolve("qianchuan")
	if qcProviderErr != nil {
		log.WithError(qcProviderErr).Fatal("qianchuan provider not registered")
	}
	qcBindingSvc := qianchuanbinding.NewService(qcBindingRepo, deps.Secrets, qcProvider)
	qcBindingH := qchandler.NewQianchuanBindingsHandler(qcBindingSvc, emitter)
	qcBindingH.Register(deps.ProjectGroup)
	qcBindingH.RegisterFlat(deps.Echo)

	qcOAuthStateRepo := qianchuanoauth.NewOAuthStateRepo(deps.DB)
	qcPublicBase := deps.Cfg.PublicBaseURL
	if qcPublicBase == "" {
		qcPublicBase = "http://localhost:7777"
		log.Warn("AGENTFORGE_PUBLIC_BASE_URL not set — qianchuan OAuth callbacks won't work over the public internet")
	}
	qcFEBase := os.Getenv("AGENTFORGE_FE_PUBLIC_BASE_URL")
	if qcFEBase == "" {
		qcFEBase = "http://localhost:3000"
	}
	qcOAuthH := &qchandler.QianchuanOAuthHandler{
		States:       qcOAuthStateRepo,
		Registry:     qcRegistry,
		Secrets:      deps.Secrets,
		Bindings:     qcBindingSvc,
		Audit:        emitter,
		PublicBase:   qcPublicBase,
		FEPublicBase: qcFEBase,
	}
	deps.ProjectGroup.POST("/qianchuan/oauth/bind/initiate", qcOAuthH.Initiate, appMiddleware.Require(appMiddleware.ActionQianchuanBindWrite))
	deps.Echo.GET("/api/v1/qianchuan/oauth/callback", qcOAuthH.Callback) // state token is the trust anchor

	qianchuanRepo := qcrepo.NewQianchuanStrategyRepository(deps.DB)
	qianchuanSvc := qcservice.NewQianchuanStrategyService(qianchuanRepo)
	qianchuanH := qchandler.NewQianchuanStrategiesHandler(qianchuanSvc)
	qchandler.RegisterQianchuanStrategyRoutes(deps.Protected, qianchuanH)
	if err := strategy.SeedSystemStrategies(context.Background(), qianchuanRepo); err != nil {
		log.WithError(err).Warn("seed qianchuan system strategies")
	}

	refresher := &qianchuanoauth.Refresher{
		Bindings:    qcBindingRepo,
		Secrets:     deps.Secrets,
		Registry:    qcRegistry,
		Bus:         deps.Bus,
		Audit:       emitter,
		TickEvery:   60 * time.Second,
		EarlyWindow: 10 * time.Minute,
	}

	// Register the workflow node-type handlers the plugin contributes
	// (qianchuan_metrics_fetcher, _strategy_runner, _action_executor).
	// This runs BEFORE nodeRegistry.LockGlobal() because the plugin's
	// handlers live alongside core builtins in the global scope.
	if deps.NodeRegistry != nil {
		if err := qcworkflow.RegisterAll(deps.NodeRegistry); err != nil {
			log.WithError(err).Warn("qianchuan plugin: workflow handler registration failed")
		}
	}

	// Announce ourselves to the plugin control plane so the feature shows
	// up in GET /api/v1/plugins as an active first-party integration.
	// Failure here is non-fatal: the feature still works, it just won't
	// appear in the plugin inventory UI until the next boot retry.
	if deps.PluginSvc != nil {
		if _, err := deps.PluginSvc.RegisterFirstPartyInProc(context.Background(), qianchuanManifestPath()); err != nil {
			log.WithError(err).Warn("qianchuan plugin: self-registration into control plane failed")
		}
	}

	return refresher
}
