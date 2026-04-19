package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/agentforge/im-bridge/audit"
	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/commands"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/core/plugin"
	"github.com/agentforge/im-bridge/core/state"
	"github.com/agentforge/im-bridge/notify"
	"github.com/google/uuid"
)

// reactionSinkAdapter bridges notify.ReactionSink to the AgentForge backend
// by forwarding events through AgentForgeClient.PostReaction.
type reactionSinkAdapter struct {
	client *client.AgentForgeClient
}

func (a *reactionSinkAdapter) RecordReaction(ctx context.Context, event notify.ReactionEvent) error {
	return a.client.PostReaction(ctx, client.ReactionEvent{
		Platform:    event.Platform,
		ChatID:      event.ChatID,
		MessageID:   event.MessageID,
		UserID:      event.UserID,
		EmojiCode:   event.EmojiCode,
		RawEmoji:    event.RawEmoji,
		ReactedAt:   event.ReactedAt,
		Removed:     event.Removed,
		ReplyTarget: event.ReplyTarget,
		Metadata:    event.Metadata,
	})
}

type backendActionRelay struct {
	client   *client.AgentForgeClient
	bridgeID string
}

type platformActionHandlerSetter interface {
	SetActionHandler(notify.ActionHandler)
}

func (r *backendActionRelay) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	if r == nil || r.client == nil || req == nil {
		return nil, nil
	}

	scopedClient := r.client
	if source := strings.TrimSpace(req.Platform); source != "" {
		scopedClient = scopedClient.WithSource(source)
	}
	replyTarget := req.ReplyTarget
	bridgeID := strings.TrimSpace(req.BridgeID)
	if bridgeID == "" {
		bridgeID = strings.TrimSpace(r.bridgeID)
	}
	scopedClient = scopedClient.WithBridgeContext(bridgeID, replyTarget)

	resp, err := scopedClient.HandleIMAction(ctx, client.IMActionRequest{
		Platform:    req.Platform,
		Action:      req.Action,
		EntityID:    req.EntityID,
		ChannelID:   req.ChatID,
		UserID:      req.UserID,
		BridgeID:    bridgeID,
		ReplyTarget: replyTarget,
		Metadata:    req.Metadata,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return &notify.ActionResponse{}, nil
	}
	metadata := resp.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	if strings.TrimSpace(resp.Status) != "" {
		metadata["action_status"] = strings.TrimSpace(resp.Status)
	}
	return &notify.ActionResponse{
		Result:      resp.Result,
		ReplyTarget: resp.ReplyTarget,
		Metadata:    metadata,
		Structured:  resp.Structured,
		Native:      resp.Native,
	}, nil
}

type config struct {
	APIBase                 string
	ProjectID               string
	APIKey                  string
	BridgeIDFile            string
	ControlSharedSecret     string
	HeartbeatInterval       time.Duration
	ControlReconnectDelay   time.Duration
	StateDir                string
	DisableDurableState     bool
	SignatureSkew           time.Duration
	AuditDir                string
	DisableAudit            bool
	AuditRotateSizeMB       int
	AuditRetainDays         int
	AuditHashSalt           string
	AuditShipViaControlPlane bool
	Platform                string
	Platforms               []string // derived from IM_PLATFORMS (comma separated); falls back to [Platform]
	TransportMode           string
	TenantsConfigPath       string
	PluginDir               string
	FeishuApp               string
	FeishuSec               string
	FeishuVerificationToken string
	FeishuEventEncryptKey   string
	FeishuCallbackPath      string
	SlackBotToken           string
	SlackAppToken           string
	DingTalkAppKey          string
	DingTalkAppSecret       string
	DingTalkCardTemplateID  string
	WeComCorpID             string
	WeComAgentID            string
	WeComAgentSecret        string
	WeComCallbackToken      string
	WeComCallbackPort       string
	WeComCallbackPath       string
	QQOneBotWSURL           string
	QQAccessToken           string
	WeChatAppID             string
	WeChatAppSecret         string
	WeChatCallbackToken     string
	WeChatCallbackPort      string
	WeChatCallbackPath      string
	QQBotAppID              string
	QQBotAppSecret          string
	QQBotCallbackPort       string
	QQBotCallbackPath       string
	QQBotAPIBase            string
	QQBotTokenBase          string
	TelegramBotToken        string
	TelegramUpdateMode      string
	TelegramWebhookURL      string
	DiscordAppID            string
	DiscordBotToken         string
	DiscordPublicKey        string
	DiscordInteractionsPort string
	DiscordCommandGuildID   string
	EmailSMTPHost           string
	EmailSMTPPort           string
	EmailSMTPUser           string
	EmailSMTPPass           string
	EmailFromAddress        string
	EmailSMTPTLS            string
	EmailIMAPHost           string
	EmailIMAPPort           string
	EmailIMAPUser           string
	EmailIMAPPass           string
	NotifyPort              string
	TestPort                string
}

func loadConfig() *config {
	return &config{
		APIBase:                 envOrDefault("AGENTFORGE_API_BASE", "http://localhost:7777"),
		ProjectID:               normalizedProjectScope(os.Getenv("AGENTFORGE_PROJECT_ID")),
		APIKey:                  envOrDefault("AGENTFORGE_API_KEY", ""),
		BridgeIDFile:            envOrDefault("IM_BRIDGE_ID_FILE", ".agentforge/im-bridge-id"),
		ControlSharedSecret:     os.Getenv("IM_CONTROL_SHARED_SECRET"),
		HeartbeatInterval:       durationEnvOrDefault("IM_BRIDGE_HEARTBEAT_INTERVAL", 30*time.Second),
		ControlReconnectDelay:   durationEnvOrDefault("IM_BRIDGE_RECONNECT_DELAY", 3*time.Second),
		StateDir:                envOrDefault("IM_BRIDGE_STATE_DIR", ".agentforge"),
		DisableDurableState:     boolEnvOrDefault("IM_DISABLE_DURABLE_STATE", false),
		SignatureSkew:           skewEnvOrDefault("IM_SIGNATURE_SKEW_SECONDS", 5*time.Minute),
		AuditDir:                envOrDefault("IM_BRIDGE_AUDIT_DIR", ".agentforge/audit"),
		DisableAudit:            boolEnvOrDefault("IM_DISABLE_AUDIT", false),
		AuditRotateSizeMB:       intEnvOrDefault("IM_AUDIT_ROTATE_SIZE_MB", 128),
		AuditRetainDays:         intEnvOrDefault("IM_AUDIT_RETAIN_DAYS", 14),
		AuditHashSalt:           os.Getenv("IM_AUDIT_HASH_SALT"),
		AuditShipViaControlPlane: boolEnvOrDefault("IM_AUDIT_SHIP_VIA_CONTROL_PLANE", false),
		Platform:                envOrDefault("IM_PLATFORM", "feishu"),
		Platforms:               parsePlatformList(os.Getenv("IM_PLATFORMS"), envOrDefault("IM_PLATFORM", "feishu")),
		TransportMode:           envOrDefault("IM_TRANSPORT_MODE", transportModeStub),
		TenantsConfigPath:       strings.TrimSpace(os.Getenv("IM_TENANTS_CONFIG")),
		PluginDir:               strings.TrimSpace(os.Getenv("IM_BRIDGE_PLUGIN_DIR")),
		FeishuApp:               os.Getenv("FEISHU_APP_ID"),
		FeishuSec:               os.Getenv("FEISHU_APP_SECRET"),
		FeishuVerificationToken: os.Getenv("FEISHU_VERIFICATION_TOKEN"),
		FeishuEventEncryptKey:   os.Getenv("FEISHU_EVENT_ENCRYPT_KEY"),
		FeishuCallbackPath:      envOrDefault("FEISHU_CALLBACK_PATH", "/feishu/callback"),
		SlackBotToken:           os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken:           os.Getenv("SLACK_APP_TOKEN"),
		DingTalkAppKey:          os.Getenv("DINGTALK_APP_KEY"),
		DingTalkAppSecret:       os.Getenv("DINGTALK_APP_SECRET"),
		DingTalkCardTemplateID:  os.Getenv("DINGTALK_CARD_TEMPLATE_ID"),
		WeComCorpID:             os.Getenv("WECOM_CORP_ID"),
		WeComAgentID:            os.Getenv("WECOM_AGENT_ID"),
		WeComAgentSecret:        os.Getenv("WECOM_AGENT_SECRET"),
		WeComCallbackToken:      os.Getenv("WECOM_CALLBACK_TOKEN"),
		WeComCallbackPort:       os.Getenv("WECOM_CALLBACK_PORT"),
		WeComCallbackPath:       envOrDefault("WECOM_CALLBACK_PATH", "/wecom/callback"),
		WeChatAppID:             os.Getenv("WECHAT_APP_ID"),
		WeChatAppSecret:         os.Getenv("WECHAT_APP_SECRET"),
		WeChatCallbackToken:     os.Getenv("WECHAT_CALLBACK_TOKEN"),
		WeChatCallbackPort:      os.Getenv("WECHAT_CALLBACK_PORT"),
		WeChatCallbackPath:      envOrDefault("WECHAT_CALLBACK_PATH", "/wechat/callback"),
		QQOneBotWSURL:           os.Getenv("QQ_ONEBOT_WS_URL"),
		QQAccessToken:           os.Getenv("QQ_ACCESS_TOKEN"),
		QQBotAppID:              os.Getenv("QQBOT_APP_ID"),
		QQBotAppSecret:          os.Getenv("QQBOT_APP_SECRET"),
		QQBotCallbackPort:       os.Getenv("QQBOT_CALLBACK_PORT"),
		QQBotCallbackPath:       envOrDefault("QQBOT_CALLBACK_PATH", "/qqbot/callback"),
		QQBotAPIBase:            envOrDefault("QQBOT_API_BASE", "https://api.sgroup.qq.com"),
		QQBotTokenBase:          envOrDefault("QQBOT_TOKEN_BASE", "https://bots.qq.com"),
		TelegramBotToken:        os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramUpdateMode:      envOrDefault("TELEGRAM_UPDATE_MODE", "longpoll"),
		TelegramWebhookURL:      os.Getenv("TELEGRAM_WEBHOOK_URL"),
		DiscordAppID:            os.Getenv("DISCORD_APP_ID"),
		DiscordBotToken:         os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordPublicKey:        os.Getenv("DISCORD_PUBLIC_KEY"),
		DiscordInteractionsPort: os.Getenv("DISCORD_INTERACTIONS_PORT"),
		DiscordCommandGuildID:   os.Getenv("DISCORD_COMMAND_GUILD_ID"),
		EmailSMTPHost:           os.Getenv("EMAIL_SMTP_HOST"),
		EmailSMTPPort:           envOrDefault("EMAIL_SMTP_PORT", "587"),
		EmailSMTPUser:           os.Getenv("EMAIL_SMTP_USER"),
		EmailSMTPPass:           os.Getenv("EMAIL_SMTP_PASS"),
		EmailFromAddress:        os.Getenv("EMAIL_FROM_ADDRESS"),
		EmailSMTPTLS:            envOrDefault("EMAIL_SMTP_TLS", "true"),
		EmailIMAPHost:           os.Getenv("EMAIL_IMAP_HOST"),
		EmailIMAPPort:           envOrDefault("EMAIL_IMAP_PORT", "993"),
		EmailIMAPUser:           os.Getenv("EMAIL_IMAP_USER"),
		EmailIMAPPass:           os.Getenv("EMAIL_IMAP_PASS"),
		NotifyPort:              envOrDefault("NOTIFY_PORT", "7779"),
		TestPort:                envOrDefault("TEST_PORT", "7780"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parsePlatformList turns the comma-separated IM_PLATFORMS value into a
// deduplicated slice. An empty value falls back to [legacy], preserving the
// single-provider semantics of IM_PLATFORM. Whitespace around entries is
// trimmed. Empty entries are silently dropped; duplicate detection is done
// downstream in selectProviders where we can report the duplicate name.
func parsePlatformList(raw, legacy string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if legacy = strings.TrimSpace(legacy); legacy != "" {
			return []string{legacy}
		}
		return nil
	}
	pieces := strings.Split(raw, ",")
	out := make([]string, 0, len(pieces))
	for _, p := range pieces {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// providerSpecificSecret returns the HMAC shared secret for a given
// provider id. It prefers IM_SECRET_<NORMALIZED> and falls back to the
// process-wide IM_CONTROL_SHARED_SECRET.
func providerSpecificSecret(providerID, fallback string) string {
	if override := strings.TrimSpace(os.Getenv("IM_SECRET_" + strings.ToUpper(core.NormalizePlatformName(providerID)))); override != "" {
		return override
	}
	return fallback
}

// providerSpecificNotifyPort returns the per-provider notify port, if
// configured. Operators may set IM_NOTIFY_PORT_<PROVIDER> to give a
// provider its own HTTP server. When no override exists, callers supply a
// fallback port (typically derived by offset from the base NOTIFY_PORT).
func providerSpecificNotifyPort(providerID, fallback string) string {
	if override := strings.TrimSpace(os.Getenv("IM_NOTIFY_PORT_" + strings.ToUpper(core.NormalizePlatformName(providerID)))); override != "" {
		return override
	}
	return fallback
}

func normalizedProjectScope(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if _, err := uuid.Parse(trimmed); err != nil {
		return ""
	}
	return trimmed
}

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	if raw := os.Getenv(key); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolEnvOrDefault(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// skewEnvOrDefault parses IM_SIGNATURE_SKEW_SECONDS. Operators specify raw
// seconds (e.g. "300") rather than Go duration syntax; we accept both.
func skewEnvOrDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return time.Duration(n) * time.Second
	}
	if parsed, err := time.ParseDuration(raw); err == nil {
		return parsed
	}
	return fallback
}

// collectProviderCredentials builds the ReconcileConfig.Credentials map
// from the current environment. The map covers all known provider secrets
// and optional tunables so providers that implement HotReloader can pick
// the fields they care about and ignore the rest.
func collectProviderCredentials(cfg *config) map[string]string {
	out := map[string]string{
		"FEISHU_APP_ID":                  cfg.FeishuApp,
		"FEISHU_APP_SECRET":              cfg.FeishuSec,
		"FEISHU_VERIFICATION_TOKEN":      cfg.FeishuVerificationToken,
		"FEISHU_EVENT_ENCRYPT_KEY":       cfg.FeishuEventEncryptKey,
		"SLACK_BOT_TOKEN":                cfg.SlackBotToken,
		"SLACK_APP_TOKEN":                cfg.SlackAppToken,
		"DINGTALK_APP_KEY":               cfg.DingTalkAppKey,
		"DINGTALK_APP_SECRET":            cfg.DingTalkAppSecret,
		"WECOM_CORP_ID":                  cfg.WeComCorpID,
		"WECOM_AGENT_ID":                 cfg.WeComAgentID,
		"WECOM_AGENT_SECRET":             cfg.WeComAgentSecret,
		"WECOM_CALLBACK_TOKEN":           cfg.WeComCallbackToken,
		"QQ_ONEBOT_WS_URL":               cfg.QQOneBotWSURL,
		"QQ_ACCESS_TOKEN":                cfg.QQAccessToken,
		"QQBOT_APP_ID":                   cfg.QQBotAppID,
		"QQBOT_APP_SECRET":               cfg.QQBotAppSecret,
		"TELEGRAM_BOT_TOKEN":             cfg.TelegramBotToken,
		"DISCORD_APP_ID":                 cfg.DiscordAppID,
		"DISCORD_BOT_TOKEN":              cfg.DiscordBotToken,
		"DISCORD_PUBLIC_KEY":             cfg.DiscordPublicKey,
		"IM_CONTROL_SHARED_SECRET":       cfg.ControlSharedSecret,
	}
	return out
}

func intEnvOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	return fallback
}

func selectPlatform(cfg *config) (core.Platform, error) {
	provider, err := selectProvider(cfg)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{"component": "main", "platform": provider.Source(), "transport": provider.TransportMode}).Info("Selected platform")
	return provider.Platform, nil
}

// providerBinding is the per-provider runtime wiring produced by setup.
// Every active provider has its own engine, notify receiver, and runtime
// control-plane connection. State store / audit writer / client factory
// are shared across all providers in the bridge process.
type providerBinding struct {
	Provider       *activeProvider
	Engine         *core.Engine
	NotifyServer   *notify.Receiver
	RuntimeControl *bridgeRuntimeControl
	NotifyPort     string
}

func main() {
	cfg := loadConfig()

	platforms := cfg.Platforms
	if len(platforms) == 0 {
		platforms = []string{cfg.Platform}
	}

	providers, err := selectProviders(cfg, platforms)
	if err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Invalid IM bridge configuration")
	}
	if len(providers) == 0 {
		log.WithField("component", "main").Fatal("No IM providers selected")
	}
	primary := providers[0]
	platform := primary.Platform
	log.WithFields(log.Fields{
		"component":       "main",
		"platform_count":  len(providers),
		"primary":         core.NormalizePlatformName(platform.Name()),
	}).Info("IM platforms selected")

	bridgeID, err := loadOrCreateBridgeID(cfg.BridgeIDFile)
	if err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Failed to initialize bridge id")
	}

	// Open durable state store (SQLite). Authoritative for dedupe/rate/nonce.
	// When IM_DISABLE_DURABLE_STATE is set the bridge falls back to the
	// historical in-memory behaviour with a clear warning in the log.
	var stateStore *state.Store
	if cfg.DisableDurableState {
		log.WithField("component", "main").Warn("durable_state=disabled fallback=memory: dedupe/rate state lost on restart")
	} else {
		statePath := filepath.Join(cfg.StateDir, "state.db")
		stateStore, err = state.Open(state.Config{Path: statePath})
		if err != nil {
			log.WithField("component", "main").WithError(err).Fatal("Failed to open durable state store")
		}
		log.WithFields(log.Fields{"component": "main", "state_path": statePath}).Info("Durable state store ready")
	}

	// Resolve audit hash salt (env > state.db-persisted > generate+persist).
	auditSalt := strings.TrimSpace(cfg.AuditHashSalt)
	auditSaltSource := "env"
	if auditSalt == "" && stateStore != nil {
		if v, ok, _ := stateStore.SettingsGet("audit_salt"); ok && v != "" {
			auditSalt = v
			auditSaltSource = "state"
		} else {
			generated, err := audit.GenerateSalt()
			if err != nil {
				log.WithField("component", "main").WithError(err).Fatal("Failed to generate audit salt")
			}
			if err := stateStore.SettingsPut("audit_salt", generated); err != nil {
				log.WithField("component", "main").WithError(err).Fatal("Failed to persist audit salt")
			}
			auditSalt = generated
			auditSaltSource = "generated"
		}
	}

	// Open structured audit writer.
	auditWriter, err := audit.New(audit.RotatingConfig{
		Dir:            cfg.AuditDir,
		MaxSizeBytes:   int64(cfg.AuditRotateSizeMB) * 1024 * 1024,
		RetainDuration: time.Duration(cfg.AuditRetainDays) * 24 * time.Hour,
		DisableWriter:  cfg.DisableAudit,
	})
	if err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Failed to open audit writer")
	}
	log.WithFields(log.Fields{
		"component":         "main",
		"audit_dir":         cfg.AuditDir,
		"audit_disabled":    cfg.DisableAudit,
		"audit_salt_source": auditSaltSource,
	}).Info("Audit writer ready")

	// Load tenant registry + resolver. Empty file path disables tenant
	// routing (legacy single-tenant mode). Any parse error fails fast.
	tenantResult, err := core.LoadTenantsConfig(cfg.TenantsConfigPath)
	if err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Failed to load tenants config")
	}
	if tenantResult.Registry != nil {
		log.WithFields(log.Fields{
			"component":     "main",
			"tenant_count":  tenantResult.Registry.Len(),
			"default_tenant": func() string {
				if tenantResult.Default != nil {
					return tenantResult.Default.ID
				}
				return ""
			}(),
		}).Info("Tenants configuration loaded")
	}

	// Pair every provider with the list of tenants that serve it. Today we
	// don't yet have per-(provider,tenant) scoping in tenants.yaml, so we
	// attach every tenant id to every provider. The registration payload
	// can later narrow this using provider-specific resolvers.
	tenantIDs := []string{}
	if tenantResult.Registry != nil {
		tenantIDs = tenantResult.Registry.IDs()
	}
	for _, p := range providers {
		p.Tenants = append([]string(nil), tenantIDs...)
	}

	// Shared client factory. Produces per-tenant AgentForgeClient.
	clientFactory := client.NewClientFactory(client.FactoryOptions{
		BaseURL:        cfg.APIBase,
		DefaultProject: cfg.ProjectID,
		DefaultAPIKey:  cfg.APIKey,
		DefaultSource:  primary.Source(),
		Registry:       tenantResult.Registry,
	})
	// Legacy apiClient retained for the capability probe and action relay —
	// both of which operate on bridge-level state rather than per-tenant.
	apiClient := clientFactory.Default().WithBridgeContext(bridgeID, nil)

	// Create an engine per provider so each platform is serviced in its
	// own context. Commands are registered against each engine in a shared
	// loop so the command surface is identical across providers.
	bindings := make([]*providerBinding, 0, len(providers))
	for i, p := range providers {
		engine := core.NewEngine(p.Platform)
		engine.SetBridgeCapabilityProbe(core.BridgeCapabilityProbeFunc(func(ctx context.Context, capability core.BridgeCapability) error {
			switch capability {
			case core.BridgeCapabilityDecompose,
				core.BridgeCapabilityGenerate,
				core.BridgeCapabilityClassifyIntent,
				core.BridgeCapabilityPool,
				core.BridgeCapabilityHealth,
				core.BridgeCapabilityRuntimes,
				core.BridgeCapabilityTools:
				_, err := apiClient.GetBridgeHealth(ctx)
				return err
			default:
				return nil
			}
		}))
		bindings = append(bindings, &providerBinding{
			Provider:   p,
			Engine:     engine,
			NotifyPort: providerSpecificNotifyPort(p.Descriptor.ID, offsetPort(cfg.NotifyPort, i)),
		})
	}
	// Default engine/platform variables kept for legacy reload paths.
	_ = bindings[0].Engine

	// Configure rate limiter. Operators override the default policy set
	// via IM_RATE_POLICY (JSON array); otherwise DefaultPolicies() applies.
	var policies []core.RateLimitPolicy
	if raw := os.Getenv("IM_RATE_POLICY"); strings.TrimSpace(raw) != "" {
		parsed, err := core.ParsePolicies(raw)
		if err != nil {
			log.WithField("component", "main").WithError(err).Fatal("Invalid IM_RATE_POLICY")
		}
		policies = parsed
	}
	// Back-compat: if the legacy RATE_LIMIT_RATE is set, treat it as an
	// override for the session-default policy.
	if v := os.Getenv("RATE_LIMIT_RATE"); v != "" && len(policies) == 0 {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && n > 0 {
			policies = []core.RateLimitPolicy{{
				ID:         "session-default",
				Dimensions: []core.RateDimension{core.DimChat, core.DimUser},
				Rate:       n,
				Window:     time.Minute,
			}}
		}
	}
	// Rate limiter is shared across providers — policies apply per tenant /
	// chat / user but the counter storage is global to the bridge process.
	rateLimiter := core.NewRateLimiter(policies)
	if stateStore != nil {
		rateLimiter.SetStore(stateStore)
	}

	// Configure egress sanitization mode (process-wide).
	core.SetDefaultSanitizeMode(core.ParseSanitizeMode(envOrDefault("IM_SANITIZE_EGRESS", "strict")))

	// Attachment staging directory for inbound/outbound file payloads. When
	// IM_BRIDGE_STATE_DIR is unset the staging store is nil and the receiver
	// rejects attachment payloads.
	stagingDir := strings.TrimSpace(os.Getenv("IM_BRIDGE_ATTACHMENT_DIR"))
	if stagingDir == "" {
		stagingDir = filepath.Join(cfg.StateDir, "attachments")
	}
	var staging *notify.StagingStore
	if s, err := notify.NewStagingStore(stagingDir); err != nil {
		log.WithField("component", "main").WithError(err).Warn("attachment staging disabled")
	} else if s != nil {
		s.StartWorker(time.Minute)
		staging = s
	}

	// Command plugin registry: when IM_BRIDGE_PLUGIN_DIR is set, load
	// plugin.yaml manifests from that dir and install the registry's
	// dispatch as a fallback for unknown commands. Missing dir disables
	// the feature — built-in commands still work as before.
	var pluginRegistry *plugin.Registry
	if cfg.PluginDir != "" {
		pluginRegistry = plugin.NewRegistry(cfg.PluginDir)
		if err := pluginRegistry.ReloadAll(); err != nil {
			log.WithField("component", "main").WithError(err).Warn("plugin reload failed; continuing without plugins")
		}
		pluginCtx, pluginCancel := context.WithCancel(context.Background())
		defer pluginCancel()
		pluginRegistry.StartWatcher(pluginCtx, 30*time.Second)
		log.WithFields(log.Fields{
			"component":    "main",
			"plugin_count": len(pluginRegistry.Plugins()),
			"plugin_dir":   cfg.PluginDir,
		}).Info("Plugin registry loaded")
	}

	// Wire each provider binding. Each gets its own engine / notify receiver /
	// runtime control-plane connection but shares state/audit/factory/rate.
	for _, b := range bindings {
		provider := b.Provider
		b.Engine.SetRateLimiter(rateLimiter)
		b.Engine.SetBridgeID(bridgeID)
		b.Engine.SetCommandAllowlist(core.NewCommandAllowlist(os.Getenv("IM_COMMAND_ALLOWLIST")))
		b.Engine.SetTenantResolver(tenantResult.Resolver, tenantResult.Default)
		registerCommandHandlers(b.Engine, clientFactory, bridgeID, stateStore)
		if pluginRegistry != nil {
			attachPluginRegistry(b.Engine, pluginRegistry)
		}

		b.RuntimeControl = newBridgeRuntimeControl(cfg, bridgeID, provider, apiClient)
		// Registration payload: tenants the provider serves + tenant list.
		b.RuntimeControl.SetTenants(provider.Tenants, buildTenantManifest(tenantResult))
		if err := b.RuntimeControl.Start(context.Background()); err != nil {
			log.WithField("component", "main").WithError(err).Fatal("Failed to start runtime control plane")
		}

		notifyServer := notify.NewReceiverWithMetadata(provider.Platform, provider.Metadata(), b.NotifyPort)
		notifyServer.SetSharedSecret(providerSpecificSecret(provider.Descriptor.ID, cfg.ControlSharedSecret))
		notifyServer.SetSignatureSkew(cfg.SignatureSkew)
		notifyServer.SetBridgeID(bridgeID)
		notifyServer.SetAuditWriter(auditWriter, auditSalt)
		if stateStore != nil {
			notifyServer.SetDedupeStore(stateStore)
		}
		relay := &backendActionRelay{client: apiClient, bridgeID: bridgeID}
		cmdHandler := &commands.CommandActionHandler{
			Engine:      b.Engine,
			Inner:       relay,
			GetPlatform: func() core.Platform { return provider.Platform },
		}
		notifyServer.SetActionHandler(cmdHandler)
		configurePlatformActionCallbacks(provider.Platform, cmdHandler)
		configurePlatformLifecycle(provider.Platform)
		if staging != nil {
			notifyServer.SetStagingStore(staging)
			notifyServer.SetReactionSink(&reactionSinkAdapter{client: apiClient})
		}
		b.NotifyServer = notifyServer

		go func(nsvr *notify.Receiver, port string) {
			if err := nsvr.Start(); err != nil {
				log.WithField("component", "main").WithField("port", port).WithError(err).Error("Notification receiver error")
			}
		}(notifyServer, b.NotifyPort)
	}

	// Start every engine (starts each platform transport).
	for _, b := range bindings {
		if err := b.Engine.Start(); err != nil {
			log.WithField("component", "main").WithField("provider", b.Provider.Descriptor.ID).WithError(err).Fatal("Failed to start engine")
		}
	}

	portList := make([]string, 0, len(bindings))
	platformList := make([]string, 0, len(bindings))
	for _, b := range bindings {
		portList = append(portList, b.NotifyPort)
		platformList = append(platformList, core.NormalizePlatformName(b.Provider.Platform.Name()))
	}
	log.WithFields(log.Fields{
		"component":   "main",
		"platforms":   strings.Join(platformList, ","),
		"transport":   normalizeTransportMode(cfg.TransportMode),
		"notify_ports": strings.Join(portList, ","),
		"test_port":   cfg.TestPort,
	}).Info("IM Bridge started successfully")

	// Hot-reload handler: SIGHUP reloads env-driven values and asks every
	// active provider to reconcile in-place. Providers that cannot honor
	// hot reload log `manual_restart_required`.
	hup := make(chan os.Signal, 1)
	installHotReloadSignal(hup)
	go func() {
		for range hup {
			newCfg := loadConfig()
			core.SetDefaultSanitizeMode(core.ParseSanitizeMode(envOrDefault("IM_SANITIZE_EGRESS", "strict")))
			allowlist := core.NewCommandAllowlist(os.Getenv("IM_COMMAND_ALLOWLIST"))
			for _, b := range bindings {
				b.Engine.SetCommandAllowlist(allowlist)
				if b.NotifyServer != nil {
					b.NotifyServer.SetSignatureSkew(newCfg.SignatureSkew)
				}
			}

			// Reload tenants if path is set.
			if newCfg.TenantsConfigPath != "" {
				if tResult, terr := core.LoadTenantsConfig(newCfg.TenantsConfigPath); terr == nil && tResult.Registry != nil {
					clientFactory.SetTenantRegistry(tResult.Registry)
					for _, b := range bindings {
						b.Engine.SetTenantResolver(tResult.Resolver, tResult.Default)
					}
					log.WithField("component", "main").Info("Tenant registry reloaded on SIGHUP")
				} else if terr != nil {
					log.WithField("component", "main").WithError(terr).Warn("Tenant reload failed; keeping previous registry")
				}
			}

			creds := collectProviderCredentials(newCfg)
			for _, b := range bindings {
				platform := b.Provider.Platform
				if reloader, ok := platform.(core.HotReloader); ok {
					result := reloader.Reconcile(context.Background(), core.ReconcileConfig{Credentials: creds})
					fields := log.Fields{
						"component": "main",
						"provider":  b.Provider.Descriptor.ID,
						"applied":   result.Applied,
						"deferred":  result.Deferred,
					}
					if len(result.Errors) > 0 {
						errStrings := make([]string, 0, len(result.Errors))
						for _, e := range result.Errors {
							errStrings = append(errStrings, e.Error())
						}
						fields["errors"] = errStrings
						log.WithFields(fields).Warn("SIGHUP reconcile reported errors")
					} else {
						log.WithFields(fields).Info("SIGHUP reconcile complete")
					}
				} else {
					log.WithFields(log.Fields{
						"component": "main",
						"provider":  b.Provider.Descriptor.ID,
					}).Warn("SIGHUP ignored: manual_restart_required")
				}
			}
		}
	}()

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.WithField("component", "main").Info("Shutting down...")
	signal.Stop(hup)
	close(hup)

	// Stop every provider in parallel with a shared deadline so one hung
	// provider does not starve the others.
	var shutdownWG sync.WaitGroup
	for _, b := range bindings {
		shutdownWG.Add(1)
		go func(b *providerBinding) {
			defer shutdownWG.Done()
			_ = b.Engine.Stop()
			if b.RuntimeControl != nil {
				_ = b.RuntimeControl.Stop(context.Background())
			}
			if b.NotifyServer != nil {
				_ = b.NotifyServer.Stop()
			}
		}(b)
	}
	done := make(chan struct{})
	go func() { shutdownWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		log.WithField("component", "main").Warn("Shutdown deadline exceeded; some providers still draining")
	}

	if stateStore != nil {
		_ = stateStore.Close()
	}
	if auditWriter != nil {
		_ = auditWriter.Close()
	}
	log.WithField("component", "main").Info("Goodbye")
	_ = platform // retained for legacy referrers; see bindings[0].Provider.Platform
}

// offsetPort adds `i` to the numeric form of `base`. Non-numeric base
// values are returned unchanged (ignoring offset) so operators can override
// individual ports via IM_NOTIFY_PORT_<PROVIDER> when they need named ports.
func offsetPort(base string, i int) string {
	n, err := strconv.Atoi(strings.TrimSpace(base))
	if err != nil {
		return base
	}
	return strconv.Itoa(n + i)
}

// attachPluginRegistry registers every plugin-declared slash command on
// the engine so operator-installed plugins participate in the same
// dispatch path as builtin commands. For now, plugin commands register
// only when a same-named command is not already registered; the builtin
// handlers win conflicts so marketplace plugins cannot shadow core flows.
func attachPluginRegistry(engine *core.Engine, registry *plugin.Registry) {
	if engine == nil || registry == nil {
		return
	}
	for _, p := range registry.Plugins() {
		for _, cmd := range p.Manifest.Commands {
			slash := strings.TrimSpace(cmd.Slash)
			if slash == "" {
				continue
			}
			cmdID := p.Manifest.ID
			slashName := slash
			engine.RegisterCommand(slashName, func(pl core.Platform, msg *core.Message, args string) {
				ctx := context.Background()
				// Parse the first token as a subcommand for manifests that
				// declare subcommand groups.
				subcommand := ""
				if parts := strings.Fields(strings.TrimSpace(args)); len(parts) > 0 {
					subcommand = parts[0]
				}
				icx := plugin.InvokeContext{
					Command:    slashName,
					Subcommand: subcommand,
					Args:       strings.TrimSpace(args),
					TenantID:   msg.TenantID,
					Platform:   msg.Platform,
					UserID:     msg.UserID,
					ChatID:     msg.ChatID,
					Metadata:   msg.Metadata,
				}
				res, err := registry.Dispatch(ctx, icx)
				if err != nil {
					if err == plugin.ErrNotFound {
						_ = pl.Reply(ctx, msg.ReplyCtx, "该命令未在插件中注册。")
						return
					}
					_ = pl.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("插件 %s 执行失败: %v", cmdID, err))
					return
				}
				if res == nil {
					return
				}
				if res.Err != "" {
					_ = pl.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("插件 %s: %s", cmdID, res.Err))
					return
				}
				if res.Text != "" {
					_ = pl.Reply(ctx, msg.ReplyCtx, res.Text)
				}
			})
		}
	}
}

// buildTenantManifest produces the {id: projectId} list shipped in the
// registration payload so the backend can index bridges by tenant.
func buildTenantManifest(res *core.TenantLoadResult) []client.TenantBinding {
	if res == nil || res.Registry == nil {
		return nil
	}
	all := res.Registry.All()
	out := make([]client.TenantBinding, 0, len(all))
	for _, t := range all {
		out = append(out, client.TenantBinding{ID: t.ID, ProjectID: t.ProjectID})
	}
	return out
}

func registerCommandHandlers(engine *core.Engine, factory client.ClientProvider, bridgeID string, stateStore *state.Store) {
	commands.RegisterTaskCommands(engine, factory)
	commands.RegisterAgentCommands(engine, factory)
	commands.RegisterCostCommands(engine, factory)
	commands.RegisterReviewCommands(engine, factory)
	commands.RegisterSprintCommands(engine, factory)
	commands.RegisterQueueCommands(engine, factory)
	commands.RegisterTeamCommands(engine, factory)
	commands.RegisterMemoryCommands(engine, factory)
	commands.RegisterLoginCommands(engine, factory)
	commands.RegisterProjectCommands(engine, factory)
	commands.RegisterToolsCommands(engine, factory)
	commands.RegisterDocumentCommands(engine, factory)
	commands.RegisterWorkflowCommands(engine, factory)
	commands.RegisterHelpCommand(engine)

	// Session store: persisted when stateStore is available, in-memory
	// otherwise. The backing store is keyed by (tenantID, sessionKey) so
	// intent/NLU context survives restarts without bleeding across tenants.
	sessionStore := state.NewSessionStore(stateStore)

	engine.SetFallback(func(p core.Platform, msg *core.Message) {
		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext(bridgeID, msg.ReplyTarget)
		if resolved := commands.ResolveDirectRuntimeMention(msg.Content); resolved != "" {
			log.WithFields(log.Fields{
				"component": "main",
				"platform":  msg.Platform,
				"userId":    msg.UserID,
				"command":   resolved,
			}).Info("Direct runtime mention routed to agent run command")
			cloned := *msg
			cloned.Content = resolved
			engine.HandleMessage(p, &cloned)
			return
		}
		historyKey := strings.TrimSpace(msg.SessionKey)
		if historyKey == "" {
			historyKey = fmt.Sprintf("%s:%s:%s", msg.Platform, msg.ChatID, msg.UserID)
		}
		// Append the current message and fetch the most-recent window used
		// by the intent classifier. The underlying store is persistent when
		// durable state is enabled so restart does not reset history.
		_ = sessionStore.Append(msg.TenantID, historyKey, msg.Content)
		history := sessionStore.Recent(msg.TenantID, historyKey, 5)

		classified, classifyErr := scopedClient.ClassifyMentionIntent(ctx, client.MentionIntentRequest{
			Text:       msg.Content,
			UserID:     msg.UserID,
			Candidates: commands.IntentCandidates(),
			Context: map[string]any{
				"platform":   msg.Platform,
				"sessionKey": msg.SessionKey,
				"threadId":   msg.ThreadID,
				"chatId":     msg.ChatID,
				"history":    history,
			},
		})
		if classifyErr == nil && classified != nil {
			if classified.Confidence >= 0.7 {
				if resolved := commands.ResolveIntentCommand(classified.Intent, classified.Command, classified.Args); resolved != "" {
					log.WithFields(log.Fields{
						"component":  "main",
						"intent":     classified.Intent,
						"command":    resolved,
						"confidence": classified.Confidence,
						"platform":   msg.Platform,
						"userId":     msg.UserID,
					}).Info("Bridge mention intent routed to command")
					cloned := *msg
					cloned.Content = resolved
					engine.HandleMessage(p, &cloned)
					return
				}
				if strings.TrimSpace(classified.Reply) != "" {
					log.WithFields(log.Fields{
						"component":  "main",
						"intent":     classified.Intent,
						"confidence": classified.Confidence,
						"platform":   msg.Platform,
						"userId":     msg.UserID,
					}).Info("Bridge mention intent replied directly")
					_ = p.Reply(ctx, msg.ReplyCtx, classified.Reply)
					return
				}
			}
			log.WithFields(log.Fields{
				"component":  "main",
				"intent":     classified.Intent,
				"command":    classified.Command,
				"confidence": classified.Confidence,
				"platform":   msg.Platform,
				"userId":     msg.UserID,
			}).Info("Bridge mention intent returned low-confidence disambiguation")
			reply := commands.FormatIntentDisambiguation(msg.Content, commands.ResolveIntentCommand(classified.Intent, classified.Command, classified.Args))
			if strings.TrimSpace(classified.Reply) != "" {
				reply = strings.TrimSpace(classified.Reply) + "\n" + reply
			}
			_ = p.Reply(ctx, msg.ReplyCtx, reply)
			return
		}
		if classifyErr != nil {
			log.WithFields(log.Fields{
				"component": "main",
				"platform":  msg.Platform,
				"userId":    msg.UserID,
			}).WithError(classifyErr).Warn("Bridge mention classification failed; falling back to legacy intent endpoint")
		}

		reply, err := scopedClient.SendNLU(ctx, msg.Content, msg.UserID)
		if err != nil || strings.TrimSpace(reply) == "" {
			suggestion := commands.SuggestCommandFromCatalog(msg.Content)
			if suggestion == "/help" {
				reply = commands.DefaultCommandGuidance()
			} else {
				reply = fmt.Sprintf("我建议先使用 %s", suggestion)
			}
			log.WithFields(log.Fields{
				"component": "main",
				"platform":  msg.Platform,
				"userId":    msg.UserID,
				"suggested": suggestion,
			}).WithError(err).Warn("Legacy mention intent fallback returned no reply; using local command suggestion")
		}
		_ = p.Reply(ctx, msg.ReplyCtx, reply)
	})
}

type platformLifecycleHandlerSetter interface {
	SetLifecycleHandler(core.LifecycleHandler)
}

func configurePlatformLifecycle(platform core.Platform) {
	if platform == nil {
		return
	}
	if setter, ok := platform.(platformLifecycleHandlerSetter); ok {
		setter.SetLifecycleHandler(&commands.BotLifecycleHandler{})
	}
}

func configurePlatformActionCallbacks(platform core.Platform, handler notify.ActionHandler) {
	if platform == nil || handler == nil {
		return
	}
	if setter, ok := platform.(platformActionHandlerSetter); ok {
		setter.SetActionHandler(handler)
	}
}
