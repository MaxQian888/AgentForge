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
	"github.com/agentforge/im-bridge/core/state"
	"github.com/agentforge/im-bridge/notify"
	"github.com/google/uuid"
)

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
	TransportMode           string
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
		TransportMode:           envOrDefault("IM_TRANSPORT_MODE", transportModeStub),
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

func main() {
	cfg := loadConfig()

	provider, err := selectProvider(cfg)
	if err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Invalid IM bridge configuration")
	}
	platform := provider.Platform
	log.WithFields(log.Fields{"component": "main", "platform": core.NormalizePlatformName(platform.Name())}).Info("IM platform selected")

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

	// Create API client.
	apiClient := client.NewAgentForgeClient(cfg.APIBase, cfg.ProjectID, cfg.APIKey).WithSource(provider.Source()).WithBridgeContext(bridgeID, nil)

	// Create engine and register commands.
	engine := core.NewEngine(platform)
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
	rateLimiter := core.NewRateLimiter(policies)
	if stateStore != nil {
		rateLimiter.SetStore(stateStore)
	}
	engine.SetRateLimiter(rateLimiter)
	engine.SetBridgeID(bridgeID)

	// Configure egress sanitization mode.
	core.SetDefaultSanitizeMode(core.ParseSanitizeMode(envOrDefault("IM_SANITIZE_EGRESS", "strict")))

	// Command allowlist gate (coarse-grained operator kill-switch).
	engine.SetCommandAllowlist(core.NewCommandAllowlist(os.Getenv("IM_COMMAND_ALLOWLIST")))

	registerCommandHandlers(engine, apiClient, bridgeID)

	runtimeControl := newBridgeRuntimeControl(cfg, bridgeID, provider, apiClient)
	if err := runtimeControl.Start(context.Background()); err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Failed to start runtime control plane")
	}

	// Start notification receiver in background.
	notifyServer := notify.NewReceiverWithMetadata(platform, provider.Metadata(), cfg.NotifyPort)
	notifyServer.SetSharedSecret(cfg.ControlSharedSecret)
	notifyServer.SetSignatureSkew(cfg.SignatureSkew)
	notifyServer.SetBridgeID(bridgeID)
	notifyServer.SetAuditWriter(auditWriter, auditSalt)
	if stateStore != nil {
		notifyServer.SetDedupeStore(stateStore)
	}
	relay := &backendActionRelay{
		client:   apiClient,
		bridgeID: bridgeID,
	}
	cmdHandler := &commands.CommandActionHandler{
		Engine:      engine,
		Inner:       relay,
		GetPlatform: func() core.Platform { return platform },
	}
	notifyServer.SetActionHandler(cmdHandler)
	configurePlatformActionCallbacks(platform, cmdHandler)
	configurePlatformLifecycle(platform)
	go func() {
		if err := notifyServer.Start(); err != nil {
			log.WithField("component", "main").WithError(err).Error("Notification receiver error")
		}
	}()

	// Start engine (starts platform).
	if err := engine.Start(); err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Failed to start engine")
	}
	log.WithFields(log.Fields{
		"component":   "main",
		"platform":    core.NormalizePlatformName(platform.Name()),
		"transport":   normalizeTransportMode(cfg.TransportMode),
		"notify_port": cfg.NotifyPort,
		"test_port":   cfg.TestPort,
	}).Info("IM Bridge started successfully")

	// Hot-reload handler: SIGHUP reloads env-driven values and asks the
	// active provider to reconcile in-place. Providers that cannot honor
	// hot reload log `manual_restart_required`.
	hup := make(chan os.Signal, 1)
	installHotReloadSignal(hup)
	go func() {
		for range hup {
			newCfg := loadConfig()
			core.SetDefaultSanitizeMode(core.ParseSanitizeMode(envOrDefault("IM_SANITIZE_EGRESS", "strict")))
			engine.SetCommandAllowlist(core.NewCommandAllowlist(os.Getenv("IM_COMMAND_ALLOWLIST")))
			notifyServer.SetSignatureSkew(newCfg.SignatureSkew)

			creds := collectProviderCredentials(newCfg)
			if reloader, ok := platform.(core.HotReloader); ok {
				result := reloader.Reconcile(context.Background(), core.ReconcileConfig{Credentials: creds})
				fields := log.Fields{
					"component": "main",
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
					"platform":  core.NormalizePlatformName(platform.Name()),
				}).Warn("SIGHUP ignored: manual_restart_required")
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
	_ = engine.Stop()
	_ = runtimeControl.Stop(context.Background())
	_ = notifyServer.Stop()
	if stateStore != nil {
		_ = stateStore.Close()
	}
	if auditWriter != nil {
		_ = auditWriter.Close()
	}
	log.WithField("component", "main").Info("Goodbye")
}

func registerCommandHandlers(engine *core.Engine, apiClient *client.AgentForgeClient, bridgeID string) {
	var historyMu sync.Mutex
	historyBySession := make(map[string][]string)

	commands.RegisterTaskCommands(engine, apiClient)
	commands.RegisterAgentCommands(engine, apiClient)
	commands.RegisterCostCommands(engine, apiClient)
	commands.RegisterReviewCommands(engine, apiClient)
	commands.RegisterSprintCommands(engine, apiClient)
	commands.RegisterQueueCommands(engine, apiClient)
	commands.RegisterTeamCommands(engine, apiClient)
	commands.RegisterMemoryCommands(engine, apiClient)
	commands.RegisterLoginCommands(engine, apiClient)
	commands.RegisterProjectCommands(engine, apiClient)
	commands.RegisterToolsCommands(engine, apiClient)
	commands.RegisterDocumentCommands(engine, apiClient)
	commands.RegisterHelpCommand(engine)

	engine.SetFallback(func(p core.Platform, msg *core.Message) {
		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext(bridgeID, msg.ReplyTarget)
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
		historyMu.Lock()
		history := append([]string(nil), historyBySession[historyKey]...)
		history = append(history, msg.Content)
		if len(history) > 5 {
			history = append([]string(nil), history[len(history)-5:]...)
		}
		historyBySession[historyKey] = append([]string(nil), history...)
		historyMu.Unlock()

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
