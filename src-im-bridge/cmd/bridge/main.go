package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/commands"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
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
	Platform                string
	TransportMode           string
	FeishuApp               string
	FeishuSec               string
	SlackBotToken           string
	SlackAppToken           string
	DingTalkAppKey          string
	DingTalkAppSecret       string
	WeComCorpID             string
	WeComAgentID            string
	WeComAgentSecret        string
	WeComCallbackToken      string
	WeComCallbackPort       string
	WeComCallbackPath       string
	TelegramBotToken        string
	TelegramUpdateMode      string
	TelegramWebhookURL      string
	DiscordAppID            string
	DiscordBotToken         string
	DiscordPublicKey        string
	DiscordInteractionsPort string
	DiscordCommandGuildID   string
	NotifyPort              string
	TestPort                string
}

func loadConfig() *config {
	return &config{
		APIBase:                 envOrDefault("AGENTFORGE_API_BASE", "http://localhost:7777"),
		ProjectID:               envOrDefault("AGENTFORGE_PROJECT_ID", "default-project"),
		APIKey:                  envOrDefault("AGENTFORGE_API_KEY", ""),
		BridgeIDFile:            envOrDefault("IM_BRIDGE_ID_FILE", ".agentforge/im-bridge-id"),
		ControlSharedSecret:     os.Getenv("IM_CONTROL_SHARED_SECRET"),
		HeartbeatInterval:       durationEnvOrDefault("IM_BRIDGE_HEARTBEAT_INTERVAL", 30*time.Second),
		ControlReconnectDelay:   durationEnvOrDefault("IM_BRIDGE_RECONNECT_DELAY", 3*time.Second),
		Platform:                envOrDefault("IM_PLATFORM", "feishu"),
		TransportMode:           envOrDefault("IM_TRANSPORT_MODE", transportModeStub),
		FeishuApp:               os.Getenv("FEISHU_APP_ID"),
		FeishuSec:               os.Getenv("FEISHU_APP_SECRET"),
		SlackBotToken:           os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken:           os.Getenv("SLACK_APP_TOKEN"),
		DingTalkAppKey:          os.Getenv("DINGTALK_APP_KEY"),
		DingTalkAppSecret:       os.Getenv("DINGTALK_APP_SECRET"),
		WeComCorpID:             os.Getenv("WECOM_CORP_ID"),
		WeComAgentID:            os.Getenv("WECOM_AGENT_ID"),
		WeComAgentSecret:        os.Getenv("WECOM_AGENT_SECRET"),
		WeComCallbackToken:      os.Getenv("WECOM_CALLBACK_TOKEN"),
		WeComCallbackPort:       os.Getenv("WECOM_CALLBACK_PORT"),
		WeComCallbackPath:       envOrDefault("WECOM_CALLBACK_PATH", "/wecom/callback"),
		TelegramBotToken:        os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramUpdateMode:      envOrDefault("TELEGRAM_UPDATE_MODE", "longpoll"),
		TelegramWebhookURL:      os.Getenv("TELEGRAM_WEBHOOK_URL"),
		DiscordAppID:            os.Getenv("DISCORD_APP_ID"),
		DiscordBotToken:         os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordPublicKey:        os.Getenv("DISCORD_PUBLIC_KEY"),
		DiscordInteractionsPort: os.Getenv("DISCORD_INTERACTIONS_PORT"),
		DiscordCommandGuildID:   os.Getenv("DISCORD_COMMAND_GUILD_ID"),
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

func durationEnvOrDefault(key string, fallback time.Duration) time.Duration {
	if raw := os.Getenv(key); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			return parsed
		}
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

	// Create API client.
	apiClient := client.NewAgentForgeClient(cfg.APIBase, cfg.ProjectID, cfg.APIKey).WithSource(provider.Source()).WithBridgeContext(bridgeID, nil)

	// Create engine and register commands.
	engine := core.NewEngine(platform)

	// Configure rate limiter: 20 commands per minute per user (configurable via env).
	rateLimitRate := 20
	if v := os.Getenv("RATE_LIMIT_RATE"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &rateLimitRate); n == 0 || err != nil {
			rateLimitRate = 20
		}
	}
	engine.SetRateLimiter(core.NewRateLimiter(rateLimitRate, time.Minute))

	commands.RegisterTaskCommands(engine, apiClient)
	commands.RegisterAgentCommands(engine, apiClient)
	commands.RegisterCostCommands(engine, apiClient)
	commands.RegisterReviewCommands(engine, apiClient)
	commands.RegisterSprintCommands(engine, apiClient)
	commands.RegisterHelpCommand(engine)

	// Natural language fallback: call NLU intent classification via Go backend.
	engine.SetFallback(func(p core.Platform, msg *core.Message) {
		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext(bridgeID, msg.ReplyTarget)
		reply, err := scopedClient.SendNLU(ctx, msg.Content, msg.UserID)
		if err != nil || reply == "" {
			reply = "理解失败，请使用 /help 查看可用命令。"
		}
		_ = p.Reply(ctx, msg.ReplyCtx, reply)
	})

	runtimeControl := newBridgeRuntimeControl(cfg, bridgeID, provider, apiClient)
	if err := runtimeControl.Start(context.Background()); err != nil {
		log.WithField("component", "main").WithError(err).Fatal("Failed to start runtime control plane")
	}

	// Start notification receiver in background.
	notifyServer := notify.NewReceiverWithMetadata(platform, provider.Metadata(), cfg.NotifyPort)
	notifyServer.SetSharedSecret(cfg.ControlSharedSecret)
	relay := &backendActionRelay{
		client:   apiClient,
		bridgeID: bridgeID,
	}
	notifyServer.SetActionHandler(relay)
	configurePlatformActionCallbacks(platform, relay)
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

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.WithField("component", "main").Info("Shutting down...")
	_ = engine.Stop()
	_ = runtimeControl.Stop(context.Background())
	_ = notifyServer.Stop()
	log.WithField("component", "main").Info("Goodbye")
}

func configurePlatformActionCallbacks(platform core.Platform, handler notify.ActionHandler) {
	if platform == nil || handler == nil {
		return
	}
	if setter, ok := platform.(platformActionHandlerSetter); ok {
		setter.SetActionHandler(handler)
	}
}
