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
	"github.com/agentforge/im-bridge/platform/dingtalk"
	"github.com/agentforge/im-bridge/platform/feishu"
	"github.com/agentforge/im-bridge/platform/slack"
)

type config struct {
	APIBase           string
	ProjectID         string
	APIKey            string
	Platform          string
	FeishuApp         string
	FeishuSec         string
	SlackBotToken     string
	SlackAppToken     string
	DingTalkAppKey    string
	DingTalkAppSecret string
	NotifyPort        string
	TestPort          string
}

func loadConfig() *config {
	return &config{
		APIBase:           envOrDefault("AGENTFORGE_API_BASE", "http://localhost:7777"),
		ProjectID:         envOrDefault("AGENTFORGE_PROJECT_ID", "default-project"),
		APIKey:            envOrDefault("AGENTFORGE_API_KEY", ""),
		Platform:          envOrDefault("IM_PLATFORM", "feishu"),
		FeishuApp:         os.Getenv("FEISHU_APP_ID"),
		FeishuSec:         os.Getenv("FEISHU_APP_SECRET"),
		SlackBotToken:     os.Getenv("SLACK_BOT_TOKEN"),
		SlackAppToken:     os.Getenv("SLACK_APP_TOKEN"),
		DingTalkAppKey:    os.Getenv("DINGTALK_APP_KEY"),
		DingTalkAppSecret: os.Getenv("DINGTALK_APP_SECRET"),
		NotifyPort:        envOrDefault("NOTIFY_PORT", "7779"),
		TestPort:          envOrDefault("TEST_PORT", "7780"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func selectPlatform(cfg *config) (core.Platform, error) {
	switch core.NormalizePlatformName(cfg.Platform) {
	case "feishu":
		if cfg.FeishuApp != "" && cfg.FeishuSec != "" {
			log.Printf("[main] Selected platform feishu with configured credentials; using local stub adapter on :%s until live transport is integrated", cfg.TestPort)
		} else {
			log.Printf("[main] Selected platform feishu without credentials; using local stub adapter on :%s", cfg.TestPort)
		}
		return feishu.NewStub(cfg.TestPort), nil
	case "slack":
		if cfg.SlackBotToken == "" || cfg.SlackAppToken == "" {
			return nil, fmt.Errorf("selected platform slack requires SLACK_BOT_TOKEN and SLACK_APP_TOKEN")
		}
		log.Printf("[main] Selected platform slack; using local stub adapter on :%s", cfg.TestPort)
		return slack.NewStub(cfg.TestPort), nil
	case "dingtalk":
		if cfg.DingTalkAppKey == "" || cfg.DingTalkAppSecret == "" {
			return nil, fmt.Errorf("selected platform dingtalk requires DINGTALK_APP_KEY and DINGTALK_APP_SECRET")
		}
		log.Printf("[main] Selected platform dingtalk; using local stub adapter on :%s", cfg.TestPort)
		return dingtalk.NewStub(cfg.TestPort), nil
	default:
		return nil, fmt.Errorf("unsupported IM_PLATFORM %q", cfg.Platform)
	}
}

func main() {
	cfg := loadConfig()

	platform, err := selectPlatform(cfg)
	if err != nil {
		log.Fatalf("[main] Invalid IM bridge configuration: %v", err)
	}
	log.Printf("[main] IM platform selected: %s", core.NormalizePlatformName(platform.Name()))

	// Create API client.
	apiClient := client.NewAgentForgeClient(cfg.APIBase, cfg.ProjectID, cfg.APIKey).WithSource(platform.Name())

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
	log.Printf("[main] IM Bridge started successfully (platform=%s notify_port=%s test_port=%s)", core.NormalizePlatformName(platform.Name()), cfg.NotifyPort, cfg.TestPort)

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("[main] Shutting down...")
	_ = engine.Stop()
	_ = notifyServer.Stop()
	log.Println("[main] Goodbye")
}
