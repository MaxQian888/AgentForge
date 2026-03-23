package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/commands"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	"github.com/agentforge/im-bridge/platform/feishu"
)

type config struct {
	APIBase    string
	ProjectID  string
	APIKey     string
	FeishuApp  string
	FeishuSec  string
	NotifyPort string
	TestPort   string
}

func loadConfig() *config {
	return &config{
		APIBase:    envOrDefault("AGENTFORGE_API_BASE", "http://localhost:7777"),
		ProjectID:  envOrDefault("AGENTFORGE_PROJECT_ID", "default-project"),
		APIKey:     envOrDefault("AGENTFORGE_API_KEY", ""),
		FeishuApp:  os.Getenv("FEISHU_APP_ID"),
		FeishuSec:  os.Getenv("FEISHU_APP_SECRET"),
		NotifyPort: envOrDefault("NOTIFY_PORT", "7779"),
		TestPort:   envOrDefault("TEST_PORT", "7780"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := loadConfig()

	// Create API client.
	apiClient := client.NewAgentForgeClient(cfg.APIBase, cfg.ProjectID, cfg.APIKey)

	// Create platform: real Feishu or stub for testing.
	var platform core.Platform
	if cfg.FeishuApp != "" {
		log.Println("[main] Feishu credentials detected, but full SDK not yet integrated. Using stub.")
		platform = feishu.NewStub(cfg.TestPort)
	} else {
		log.Println("[main] No Feishu credentials, starting in stub/test mode")
		platform = feishu.NewStub(cfg.TestPort)
	}

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
	log.Println("[main] IM Bridge started successfully")

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("[main] Shutting down...")
	_ = engine.Stop()
	_ = notifyServer.Stop()
	log.Println("[main] Goodbye")
}
