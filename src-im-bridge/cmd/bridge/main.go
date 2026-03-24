package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/commands"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

type config struct {
	APIBase                 string
	ProjectID               string
	APIKey                  string
	Platform                string
	TransportMode           string
	FeishuApp               string
	FeishuSec               string
	SlackBotToken           string
	SlackAppToken           string
	DingTalkAppKey          string
	DingTalkAppSecret       string
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
		Platform:                envOrDefault("IM_PLATFORM", "feishu"),
		TransportMode:           envOrDefault("IM_TRANSPORT_MODE", transportModeStub),
		FeishuApp:               os.Getenv("FEISHU_APP_ID"),
		FeishuSec:               os.Getenv("FEISHU_APP_SECRET"),
		SlackBotToken:           os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken:           os.Getenv("SLACK_APP_TOKEN"),
		DingTalkAppKey:          os.Getenv("DINGTALK_APP_KEY"),
		DingTalkAppSecret:       os.Getenv("DINGTALK_APP_SECRET"),
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

func selectPlatform(cfg *config) (core.Platform, error) {
	descriptor, err := lookupPlatformDescriptor(cfg.Platform)
	if err != nil {
		return nil, err
	}

	mode := normalizeTransportMode(cfg.TransportMode)
	if mode != transportModeStub && mode != transportModeLive {
		return nil, fmt.Errorf("unsupported IM_TRANSPORT_MODE %q", cfg.TransportMode)
	}
	if err := descriptor.ValidateConfig(cfg, mode); err != nil {
		return nil, err
	}

	var factory platformFactory
	switch mode {
	case transportModeStub:
		factory = descriptor.NewStub
	case transportModeLive:
		factory = descriptor.NewLive
	}
	if factory == nil {
		return nil, fmt.Errorf("selected platform %s does not support %s transport", descriptor.Metadata.Source, mode)
	}

	platform, err := factory(cfg)
	if err != nil {
		return nil, err
	}
	log.Printf("[main] Selected platform %s using %s transport", descriptor.Metadata.Source, mode)
	return platform, nil
}

func main() {
	cfg := loadConfig()

	platform, err := selectPlatform(cfg)
	if err != nil {
		log.Fatalf("[main] Invalid IM bridge configuration: %v", err)
	}
	log.Printf("[main] IM platform selected: %s", core.NormalizePlatformName(platform.Name()))

	// Create API client.
	apiClient := client.NewAgentForgeClient(cfg.APIBase, cfg.ProjectID, cfg.APIKey).WithPlatform(platform)

	// Create engine and register commands.
	engine := core.NewEngine(platform)

	commands.RegisterTaskCommands(engine, apiClient)
	commands.RegisterAgentCommands(engine, apiClient)
	commands.RegisterCostCommands(engine, apiClient)
	commands.RegisterHelpCommand(engine)

	// Natural language fallback.
	engine.SetFallback(func(p core.Platform, msg *core.Message) {
		_ = p.Reply(context.Background(), msg.ReplyCtx,
			"自然语言理解功能即将推出。目前请使用 /task create <标题> 创建任务。发送 /help 查看所有命令。")
	})

	// Start notification receiver in background.
	notifyServer := notify.NewReceiver(platform, cfg.NotifyPort)
	go func() {
		if err := notifyServer.Start(); err != nil {
			log.Printf("[main] Notification receiver error: %v", err)
		}
	}()

	// Start engine (starts platform).
	if err := engine.Start(); err != nil {
		log.Fatalf("[main] Failed to start engine: %v", err)
	}
	log.Printf("[main] IM Bridge started successfully (platform=%s transport=%s notify_port=%s test_port=%s)", core.NormalizePlatformName(platform.Name()), normalizeTransportMode(cfg.TransportMode), cfg.NotifyPort, cfg.TestPort)

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("[main] Shutting down...")
	_ = engine.Stop()
	_ = notifyServer.Stop()
	log.Println("[main] Goodbye")
}
