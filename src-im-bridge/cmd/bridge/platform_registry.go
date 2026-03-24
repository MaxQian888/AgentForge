package main

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/platform/dingtalk"
	"github.com/agentforge/im-bridge/platform/discord"
	"github.com/agentforge/im-bridge/platform/feishu"
	"github.com/agentforge/im-bridge/platform/slack"
	"github.com/agentforge/im-bridge/platform/telegram"
)

const (
	transportModeStub = "stub"
	transportModeLive = "live"
)

type platformFactory func(cfg *config) (core.Platform, error)

type platformDescriptor struct {
	Metadata       core.PlatformMetadata
	ValidateConfig func(cfg *config, mode string) error
	NewStub        platformFactory
	NewLive        platformFactory
}

func normalizeTransportMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return transportModeStub
	}
	return normalized
}

func platformDescriptors() map[string]platformDescriptor {
	return map[string]platformDescriptor{
		"feishu": {
			Metadata: feishu.NewStub("0").Metadata(),
			ValidateConfig: func(cfg *config, mode string) error {
				if mode == transportModeLive && (cfg.FeishuApp == "" || cfg.FeishuSec == "") {
					return fmt.Errorf("selected platform feishu requires FEISHU_APP_ID and FEISHU_APP_SECRET for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return feishu.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return feishu.NewLive(cfg.FeishuApp, cfg.FeishuSec)
			},
		},
		"slack": {
			Metadata: slack.NewStub("0").Metadata(),
			ValidateConfig: func(cfg *config, mode string) error {
				if mode == transportModeLive && (cfg.SlackBotToken == "" || cfg.SlackAppToken == "") {
					return fmt.Errorf("selected platform slack requires SLACK_BOT_TOKEN and SLACK_APP_TOKEN for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return slack.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return slack.NewLive(cfg.SlackBotToken, cfg.SlackAppToken)
			},
		},
		"dingtalk": {
			Metadata: dingtalk.NewStub("0").Metadata(),
			ValidateConfig: func(cfg *config, mode string) error {
				if mode == transportModeLive && (cfg.DingTalkAppKey == "" || cfg.DingTalkAppSecret == "") {
					return fmt.Errorf("selected platform dingtalk requires DINGTALK_APP_KEY and DINGTALK_APP_SECRET for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return dingtalk.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return dingtalk.NewLive(cfg.DingTalkAppKey, cfg.DingTalkAppSecret)
			},
		},
		"telegram": {
			Metadata: telegram.NewStub("0").Metadata(),
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				if strings.TrimSpace(cfg.TelegramBotToken) == "" {
					return fmt.Errorf("selected platform telegram requires TELEGRAM_BOT_TOKEN for live transport")
				}
				return telegramValidateConfig(cfg.TelegramUpdateMode, cfg.TelegramWebhookURL)
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return telegram.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return telegram.NewLive(cfg.TelegramBotToken)
			},
		},
		"discord": {
			Metadata: discord.NewStub("0").Metadata(),
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				if strings.TrimSpace(cfg.DiscordAppID) == "" || strings.TrimSpace(cfg.DiscordBotToken) == "" || strings.TrimSpace(cfg.DiscordPublicKey) == "" {
					return fmt.Errorf("selected platform discord requires DISCORD_APP_ID, DISCORD_BOT_TOKEN, and DISCORD_PUBLIC_KEY for live transport")
				}
				if strings.TrimSpace(cfg.DiscordInteractionsPort) == "" {
					return fmt.Errorf("selected platform discord requires DISCORD_INTERACTIONS_PORT for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return discord.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				opts := make([]discord.LiveOption, 0, 1)
				if strings.TrimSpace(cfg.DiscordCommandGuildID) != "" {
					opts = append(opts, discord.WithCommandGuildID(cfg.DiscordCommandGuildID))
				}
				return discord.NewLive(
					cfg.DiscordAppID,
					cfg.DiscordBotToken,
					cfg.DiscordPublicKey,
					cfg.DiscordInteractionsPort,
					opts...,
				)
			},
		},
	}
}

func telegramValidateConfig(updateMode, webhookURL string) error {
	normalized := strings.ToLower(strings.TrimSpace(updateMode))
	if normalized == "" {
		normalized = "longpoll"
	}
	if normalized != "longpoll" {
		return fmt.Errorf("telegram live transport currently supports only longpoll update mode")
	}
	if strings.TrimSpace(webhookURL) != "" {
		return fmt.Errorf("telegram long polling cannot be combined with webhook configuration")
	}
	return nil
}

func lookupPlatformDescriptor(name string) (platformDescriptor, error) {
	normalized := core.NormalizePlatformName(name)
	descriptor, ok := platformDescriptors()[normalized]
	if !ok {
		return platformDescriptor{}, fmt.Errorf("unsupported IM_PLATFORM %q", name)
	}
	return descriptor, nil
}
